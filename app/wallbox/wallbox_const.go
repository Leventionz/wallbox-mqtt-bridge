package wallbox

var wallboxStatusCodes = []string{
	"Ready",
	"Charging",
	"Connected waiting car",
	"Connected waiting schedule",
	"Paused",
	"Schedule end",
	"Locked",
	"Error",
	"Connected waiting current assignation",
	"Unconfigured power sharing",
	"Queue by power boost",
	"Discharging",
	"Connected waiting admin auth for mid",
	"Connected mid safety margin exceeded",
	"OCPP unavailable",
	"OCPP charge finishing",
	"OCPP reserved",
	"Updating",
	"Queue by eco smart",
}

var stateOverrides = map[int]int{
	0xA1: 0,
	0xA2: 9,
	0xA3: 14,
	0xA4: 15,
	0xA6: 17,
	0xB1: 3,
	0xB2: 4,
	0xB3: 3,
	0xB4: 2,
	0xB5: 2,
	0xB6: 4,
	0xB7: 8,
	0xB8: 8,
	0xB9: 10,
	0xBA: 10,
	0xBB: 12,
	0xBC: 13,
	0xBD: 18,
	0xC1: 1,
	0xC2: 1,
	0xC3: 11,
	0xC4: 11,
	0xD1: 6,
	0xD2: 6,
}

var stateMachineStates = map[int]string{
	0xE:  "Error",
	0xF:  "Unviable",
	0xA1: "Ready",
	0xA2: "PS Unconfig",
	0xA3: "Unavailable",
	0xA4: "Finish",
	0xA5: "Reserved",
	0xA6: "Updating",
	0xB1: "Connected 1", // Make new session?
	0xB2: "Connected 2",
	0xB3: "Connected 3", // Waiting schedule ?
	0xB4: "Connected 4",
	0xB5: "Connected 5", // Connected waiting car ?
	0xB6: "Connected 6", // Paused
	0xB7: "Waiting 1",
	0xB8: "Waiting 2",
	0xB9: "Waiting 3",
	0xBA: "Waiting 4",
	0xBB: "Mid 1",
	0xBC: "Mid 2",
	0xBD: "Waiting eco power",
	0xC1: "Charging 1",
	0xC2: "Charging 2",
	0xC3: "Discharging 1",
	0xC4: "Discharging 2",
	0xD1: "Lock",
	0xD2: "Wait Unlock",
}

var controlPilotStates = map[int]string{
	0xE:  "Error",
	0xF:  "Failure",
	0xA1: "Ready 1", // S1 at 12V, car not connected
	0xA2: "Ready 2",
	0xB1: "Connected 1", // S1 at 9V, car connected not allowed charge
	0xB2: "Connected 2", // S1 at Oscillator, car connected allowed charge
	0xC1: "Charging 1",
	0xC2: "Charging 2", // S2 closed
}

var telemetryControlPilotStates = map[int]string{
	161: "Ready 1",
	162: "Ready 2",
	177: "Connected 1",
	178: "Connected 2",
	193: "Charging 1",
	194: "Charging 2",
	195: "Charging 2",
}

var telemetryControlPilotConnected = map[int]bool{
	161: false,
	162: false,
	177: true,
	178: true,
	193: true,
	194: true,
	195: true,
}

var telemetryStatusDescriptions = map[int]string{
	0:   "Disconnected",
	14:  "Error",
	15:  "Error",
	161: "Ready",
	162: "Ready",
	163: "Disconnected",
	164: "Waiting",
	165: "Locked",
	166: "Updating",
	177: "Scheduled",
	178: "Paused",
	179: "Scheduled",
	180: "Waiting",
	181: "Waiting",
	182: "Paused",
	183: "Waiting",
	184: "Waiting",
	185: "Waiting",
	186: "Waiting",
	187: "Waiting",
	188: "Waiting",
	189: "Waiting",
	193: "Charging",
	194: "Charging",
	195: "Charging",
	196: "Discharging",
	209: "Locked",
	210: "Locked",
}

func describeTelemetryStatus(code int) string {
	if desc, ok := telemetryStatusDescriptions[code]; ok {
		return desc
	}

	return "Unknown"
}

func isChargingTelemetryStatus(code int) bool {
	return code == 193 || code == 194 || code == 195
}

func isTelemetryCableConnected(code int) bool {
	if connected, ok := telemetryControlPilotConnected[code]; ok {
		return connected
	}

	desc := describeTelemetryStatus(code)
	return desc != "Disconnected" && desc != "Ready" && desc != "Unknown"
}

var telemetryChargingStates = map[int]bool{
	193: true,
	194: true,
	195: true,
}

func isTelemetryCharging(code int) bool {
	return telemetryChargingStates[code]
}

var ocppStatusDescriptions = map[int]string{
	1: "Available",
	2: "Preparing",
	3: "Charging",
	4: "SuspendedEVSE",
	5: "SuspendedEV",
	6: "Finishing",
	7: "Reserved",
	8: "Unavailable",
	9: "Faulted",
}

func describeOCPPStatus(code int) string {
	if desc, ok := ocppStatusDescriptions[code]; ok {
		return desc
	}
	return "Unknown"
}

var ocppStatusUnplugged = map[int]bool{
	1: true, // Available – backend thinks connector is free
	4: true, // SuspendedEVSE – EVSE paused session
	5: true, // SuspendedEV – vehicle paused session
	8: true, // Unavailable
	9: true, // Faulted
}

func ocppStatusIndicatesDisconnect(code int) bool {
	return ocppStatusUnplugged[code]
}
