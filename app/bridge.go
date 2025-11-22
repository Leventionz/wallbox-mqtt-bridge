package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"wallbox-mqtt-bridge/app/wallbox"
)

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	panic("Connection to MQTT lost")
}

func RunBridge(configPath string) {
	c := LoadConfig(configPath)
	if c.Settings.OCPPMismatchSeconds == 0 {
		c.Settings.OCPPMismatchSeconds = 60
	}
	if c.Settings.OCPPRestartCooldown == 0 {
		c.Settings.OCPPRestartCooldown = 600
	}

	w := wallbox.New()
	w.RefreshData()
	stopJournal := startOCPPJournalWatcher(w)
	defer stopJournal()

	serialNumber := w.SerialNumber()
	firmwareVersion := w.FirmwareVersion()
	entityConfig := getEntities(w)
	if c.Settings.DebugSensors {
		for k, v := range getDebugEntities(w) {
			entityConfig[k] = v
		}
		for k, v := range getTelemetryEventEntities(w) {
			entityConfig[k] = v
		}
	}

	if c.Settings.PowerBoostEnabled {
		for k, v := range getPowerBoostEntities(w, c) {
			entityConfig[k] = v
		}
	}

	ocppMismatchState := "0"
	ocppLastRestart := "never"
	var mismatchStart time.Time
	var lastRestart time.Time

	entityConfig["ocpp_mismatch"] = Entity{
		Component: "binary_sensor",
		Getter:    func() string { return ocppMismatchState },
		Config: map[string]string{
			"name":            "OCPP mismatch",
			"payload_on":      "1",
			"payload_off":     "0",
			"device_class":    "problem",
			"entity_category": "diagnostic",
		},
	}

	entityConfig["ocpp_last_restart"] = Entity{
		Component: "sensor",
		Getter:    func() string { return ocppLastRestart },
		Config: map[string]string{
			"name":            "OCPP last restart",
			"entity_category": "diagnostic",
		},
	}

	topicPrefix := "wallbox_" + serialNumber
	availabilityTopic := topicPrefix + "/availability"

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", c.MQTT.Host, c.MQTT.Port))
	opts.SetUsername(c.MQTT.Username)
	opts.SetPassword(c.MQTT.Password)
	opts.SetWill(availabilityTopic, "offline", 1, true)
	opts.OnConnectionLost = connectLostHandler

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	for key, val := range entityConfig {
		component := val.Component
		uid := serialNumber + "_" + key
		config := map[string]interface{}{
			"~":                  topicPrefix + "/" + key,
			"availability_topic": availabilityTopic,
			"state_topic":        "~/state",
			"unique_id":          uid,
			"device": map[string]string{
				"identifiers": serialNumber,
				"name":        c.Settings.DeviceName,
				"sw_version":  fmt.Sprintf("%s (FW %s)", bridgeVersion(), firmwareVersion),
			},
		}
		if val.Setter != nil {
			config["command_topic"] = "~/set"
		}
		for k, v := range val.Config {
			config[k] = v
		}
		jsonPayload, _ := json.Marshal(config)
		token := client.Publish("homeassistant/"+component+"/"+uid+"/config", 1, true, jsonPayload)
		token.Wait()
	}

	token := client.Publish(availabilityTopic, 1, true, "online")
	token.Wait()

	messageHandler := func(client mqtt.Client, msg mqtt.Message) {
		field := strings.Split(msg.Topic(), "/")[1]
		payload := string(msg.Payload())
		setter := entityConfig[field].Setter
		fmt.Println("Setting", field, payload)
		setter(payload)
	}

	topic := topicPrefix + "/+/set"
	client.Subscribe(topic, 1, messageHandler)

	ticker := time.NewTicker(time.Duration(c.Settings.PollingIntervalSeconds) * time.Second)
	defer ticker.Stop()

	published := make(map[string]interface{})

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	// for debugging purposes, only run for the first 2 minutes
	// w.StartTimeConstrainedRedisSubscriptions(2 * time.Minute)
	w.StartRedisSubscriptions()

	for {
		select {
		case <-ticker.C:
			w.RefreshData()
			now := time.Now()

			connected := w.HasTelemetry && w.CableConnected() == 1
			ocppCode := w.OCPPStatusCode()
			ocppIndicatesDisconnect := w.OCPPIndicatesDisconnect()

			chargingPilot := w.IsChargingPilot()

			if connected && ocppIndicatesDisconnect && !chargingPilot {
				if mismatchStart.IsZero() {
					mismatchStart = now
					log.Printf("OCPP mismatch detected: pilot=%d (%s), OCPP=%d (%s)", w.ControlPilotCode(), w.ControlPilotStatus(), ocppCode, w.OCPPStatusDescription())
				}
				ocppMismatchState = "1"
			} else {
				if ocppMismatchState != "0" {
					log.Println("OCPP mismatch cleared")
				}
				ocppMismatchState = "0"
				mismatchStart = time.Time{}
			}

			if c.Settings.AutoRestartOCPP && ocppMismatchState == "1" && !mismatchStart.IsZero() {
				threshold := time.Duration(c.Settings.OCPPMismatchSeconds) * time.Second
				cooldown := time.Duration(c.Settings.OCPPRestartCooldown) * time.Second

				if now.Sub(mismatchStart) >= threshold && (lastRestart.IsZero() || now.Sub(lastRestart) >= cooldown) {
					log.Printf("Restarting wallboxsmachine + ocppwallbox after %s mismatch (OCPP %d: %s)", now.Sub(mismatchStart).Round(time.Second), ocppCode, w.OCPPStatusDescription())
					if err := restartCriticalServices(); err != nil {
						log.Printf("Failed to restart charging stack: %v", err)
						continue
					}
					lastRestart = now
					mismatchStart = now
					ocppLastRestart = now.Format(time.RFC3339)
				}
			}

			for key, val := range entityConfig {
				payload := val.Getter()
				bytePayload := []byte(fmt.Sprint(payload))
				if published[key] != payload {
					if val.RateLimit != nil && !val.RateLimit.Allow(strToFloat(payload)) {
						continue
					}
					fmt.Println("Publishing: ", key, payload)
					token := client.Publish(topicPrefix+"/"+key+"/state", 1, true, bytePayload)
					token.Wait()
					published[key] = payload
				}
			}
		case <-interrupt:
			fmt.Println("Interrupted. Exiting...")
			token := client.Publish(availabilityTopic, 1, true, "offline")
			token.Wait()
			client.Disconnect(250)
			return
		}
	}

	w.StopRedisSubscriptions()
}

func restartCriticalServices() error {
	services := []string{
		"wallboxsmachine.service",
		"ocppwallbox.service",
	}

	for _, svc := range services {
		cmd := exec.Command("systemctl", "restart", svc)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("restart %s: %w", svc, err)
		}
	}

	return nil
}

func bridgeVersion() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if ok {
		return buildInfo.Main.Version
	}
	return "dev"
}
func startOCPPJournalWatcher(w *wallbox.Wallbox) func() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		log.Println("Starting OCPP journal watcher...")
		if err := watchOCPPJournal(ctx, w); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("OCPP journal watcher exited: %v", err)
		} else {
			log.Println("OCPP journal watcher stopped")
		}
	}()
	return cancel
}

var statusRegex = regexp.MustCompile(`status"\s*:\s*"([^"]+)"`)

func watchOCPPJournal(ctx context.Context, w *wallbox.Wallbox) error {
	cmd, scanner, err := startJournalCommand(ctx, "json")
	if err != nil {
		log.Printf("Falling back to journalctl cat mode: %v", err)
		cmd, scanner, err = startJournalCommand(ctx, "cat")
		if err != nil {
			return err
		}
		log.Println("OCPP journal watcher running in cat mode (regex parser)")
	} else {
		log.Println("OCPP journal watcher running in json mode")
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			cmd.Process.Kill()
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		status := extractStatusFromJournal(line)
		if status == "" {
			continue
		}

		if code, ok := wallbox.LookupOCPPStatusCode(status); ok {
			log.Printf("OCPP journal: status=%s code=%d", status, code)
			w.SetOCPPStatusOverride(code)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return cmd.Wait()
}

func extractStatusFromJournal(line []byte) string {
	message := parseJournalMessage(line)
	if message == "" {
		return ""
	}

	if !strings.Contains(message, "StatusNotification") {
		return ""
	}

	matches := statusRegex.FindStringSubmatch(message)
	if len(matches) < 2 {
		return ""
	}

	return matches[1]
}

func parseJournalMessage(line []byte) string {
	var entry struct {
		Message string `json:"MESSAGE"`
	}

	if err := json.Unmarshal(line, &entry); err == nil && entry.Message != "" {
		return entry.Message
	}

	// fallback to cat mode line
	return string(line)
}

func startJournalCommand(ctx context.Context, mode string) (*exec.Cmd, *bufio.Scanner, error) {
	args := []string{"-u", "ocppwallbox.service", "-f"}
	if mode == "json" {
		args = append(args, "-o", "json")
	} else {
		args = append(args, "-o", "cat")
	}

	cmd := exec.CommandContext(ctx, "journalctl", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	return cmd, bufio.NewScanner(stdout), nil
}
