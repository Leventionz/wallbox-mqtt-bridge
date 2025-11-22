# MQTT Bridge for Wallbox

This open-source project connects your Wallbox fully locally to Home Assistant, providing you with unparalleled speed and reliability.

> adds full telemetry support for firmware 6.7.x+ (control pilot, state machine, session energy, Power Boost, etc.) and keeps older firmware working automatically via legacy fallbacks.

## Features

- **Instant Sensor Data:** The Wallbox's internal state is polled every second and any updates are immediately pushed to the external MQTT broker.

- **Instant Control:** Quickly lock/unlock, pause/resume or change the max charging current, without involving the manufacturer's servers.

- **Always available:** As long as your local network is up and your Wallbox has power, you're in control! No need to rely on a third party to communicate with the device you own.

- **Home Assistant MQTT Auto Discovery:** Enjoy a hassle-free setup with Home Assistant MQTT Auto Discovery support. The integration effortlessly integrates with your existing Home Assistant environment.

<br/>
<p align="center">
   <img src="https://github.com/jagheterfredrik/wallbox-mqtt-bridge/assets/9987465/06488a5d-e6fe-4491-b11d-e7176792a7f5" height="507" />
</p>

## Getting Started

1. [Root your Wallbox](https://github.com/jagheterfredrik/wallbox-pwn)
2. Setup an MQTT Broker, if you don't already have one. Here's an example [installing it as a Home Assistant add-on](https://www.youtube.com/watch?v=dqTn-Gk4Qeo)
3. `ssh` to your Wallbox and run

```sh
curl -sSfL https://github.com/Leventionz/wallbox-mqtt-bridge/releases/download/bridgechannels-2025.11.21/install.sh > install.sh && bash install.sh
```

Note: To upgrade to new version, simply run the command from step 3 again.

> Tip: set `BRIDGE_VERSION=<tag>` in front of the command if you want to pin a different release (e.g. testing a prerelease build).

## EVCC quickstart

- The installer now asks whether you want an EVCC helper file.
- Answer `y` and it will auto-detect your Wallbox serial (or prompt for it) and drop `~/mqtt-bridge/evcc-wallbox.yaml` containing the proper `meters`, `chargers`, and `loadpoints` sections.
- Copy that snippet into your EVCC config and adjust MQTT broker credentials on the EVCC side—topics already match the bridge’s Home Assistant entities.

## Firmware 6.7.x support

| Area | Behaviour on 6.7.x | Notes / fallback |
| --- | --- | --- |
| **Control pilot** | Telemetry control-pilot codes (161, 162, 177, 178, 193, 194, 195) drive `sensor.wallbox_control_pilot` **and** `binary_sensor.wallbox_cable_connected`. A companion `sensor.wallbox_control_pilot_state` converts those codes back to the familiar SAE/IEC letters (A/B/C). | Falls back to `state.ctrlPilot` on older firmware. |
| **State machine / status** | Telemetry `SENSOR_STATE_MACHINE` feeds `sensor.wallbox_state_machine`, `sensor.wallbox_status`, and the debug `sensor.wallbox_m2w_status`. Every code in the official Wallbox enum (Waiting, Scheduled, Paused, Charging, Locked, Updating, etc.) is mapped to a friendly string. | Falls back to the legacy `m2w/state` hashes and existing override tables automatically. |
| **OCPP visibility** | The bridge exposes `sensor.wallbox_ocpp_status` (codes 1–9 mapped to Available/Preparing/Charging/Suspended etc.), `binary_sensor.wallbox_ocpp_mismatch`, and `sensor.wallbox_ocpp_last_restart`. | `ocpp_status` lives with the other debug telemetry sensors (set `debug_sensors = true` to surface it). The mismatch + last restart sensors stay online even without the debug flag so you can monitor the self-heal. |
| **Session energy** | `sensor.wallbox_added_energy` now tracks a telemetry baseline and reports **session** Wh (Internal Meter Energy – baseline) while `sensor.wallbox_cumulative_added_energy` remains the total. | Baseline resets whenever telemetry reports a non-charging state; older firmware continues to use `scheduleEnergy`. |
| **S2 relay** | `sensor.wallbox_s2_open` is derived from control-pilot telemetry (S2 is “closed” only while telemetry reports a charging state). | Falls back to `state.S2open` where telemetry is unavailable. |
| **Charging enable** | `sensor.wallbox_charging_enable` mirrors the telemetry `SENSOR_CHARGING_ENABLE` flag so toggles are instantaneous. | Falls back to `wallbox_config.charging_enable` on older firmware. |
| **Power Boost** | When telemetry reports a PowerBoost session, the L1 sensors publish the telemetry proposal current/power; unused phases report `0`. If legacy `m2w` data exists (older firmware / multi-phase setups) it’s used automatically. | Assumes single-phase hardware unless telemetry supplies per-phase values. |
| **Other telemetry** | `charging_power*`, `charging_current*`, `temp_l*`, `status`, `control_pilot`, `state_machine`, `charging_enable`, `cable_connected`, and all debug telemetry entities emit live telemetry values out of the box. | Legacy data paths remain in place for <6.7.x devices. |

> If you update your Wallbox beyond 6.7.x, simply redeploy using the installer command above to keep the telemetry fixes in place. The bridge auto-detects telemetry and switches to legacy data when telemetry is missing.

## Release highlights (bridgechannels-2025.11.21)

- Control-pilot driven entities strictly follow SAE J1772 state mapping so Home Assistant shows “cable disconnected” whenever the pilot remains at 12 V.
- Telemetry debug sensors (`control_pilot_high_voltage`, duty cycle, etc.) no longer report zeroes on 6.7.x.
- The installer can generate an EVCC-ready YAML snippet, so you can copy/paste the MQTT topics straight into EVCC without hand-editing.
- New always-on OCPP sensors (`sensor.ocpp_status`, `binary_sensor.ocpp_mismatch`, `sensor.ocpp_last_restart`) plus an optional self-heal that restarts `wallboxsmachine.service` and `ocppwallbox.service` whenever the backend thinks the car is unplugged but the control pilot is still connected. Config saved in bridge.ini.
- `sensor.wallbox_ocpp_status` now listens to `journalctl -u ocppwallbox.service` so it mirrors the live `StatusNotification` events rather than the occasionally-stale telemetry hash.
- Added `sensor.wallbox_control_pilot_state` (letter notation) and new debug sensors for bridge firmware (`sensor.wallbox_bridge_version`) and charger firmware (`sensor.wallbox_firmware_version`) to simplify support reports.

## OCPP self-healing & sensors

The installer (or `./bridge --config`) can auto-populate these settings:

```ini
[settings]
auto_restart_ocpp = true
ocpp_mismatch_seconds = 60            # how long the mismatch must persist
ocpp_restart_cooldown_seconds = 600   # wait time between restarts
```

## Acknowledgments

The credits go out to jagheterfredrik (https://github.com/jagheterfredrik/wallbox-mqtt-bridge), who made the original MQTT Bridge for the Wallbox and jethrovo for his updated version supporting version v6.6.x.
A big shoutout to [@tronikos](https://github.com/tronikos) for their valuable contributions. This project wouldn't be the same without the collaborative spirit of the open-source community.
