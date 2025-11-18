# MQTT Bridge for Wallbox

This open-source project connects your Wallbox fully locally to Home Assistant, providing you with unparalleled speed and reliability.

> **Maintained fork**: [Leventionz/wallbox-mqtt-bridge](https://github.com/Leventionz/wallbox-mqtt-bridge) adds full telemetry support for firmware 6.7.x+ (control pilot, state machine, session energy, Power Boost, etc.) and keeps older firmware working automatically via legacy fallbacks.

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
curl -sSfL https://github.com/Leventionz/wallbox-mqtt-bridge/releases/download/bridgechannels-2025.18.11/install.sh > install.sh && bash install.sh
```

Note: To upgrade to new version, simply run the command from step 3 again.

## Firmware 6.7.x support

| Area | Behaviour on 6.7.x | Notes / fallback |
| --- | --- | --- |
| **Control pilot** | Telemetry control-pilot codes (161, 162, 177, 178, 193, 194, 195) drive `sensor.wallbox_control_pilot` and `binary_sensor.wallbox_cable_connected`. The raw code is shown as “Ready 1”, “Connected 2”, “Charging 1/2” just like SAE J1772. | Falls back to `state.ctrlPilot` on older firmware. |
| **State machine / status** | Telemetry `SENSOR_STATE_MACHINE` feeds `sensor.wallbox_state_machine`, `sensor.wallbox_status`, and the debug `sensor.wallbox_m2w_status`. Every code in the official Wallbox enum (Waiting, Scheduled, Paused, Charging, Locked, Updating, etc.) is mapped to a friendly string. | Falls back to the legacy `m2w/state` hashes and existing override tables automatically. |
| **Session energy** | `sensor.wallbox_added_energy` now tracks a telemetry baseline and reports **session** Wh (Internal Meter Energy – baseline) while `sensor.wallbox_cumulative_added_energy` remains the total. | Baseline resets whenever telemetry reports a non-charging state; older firmware continues to use `scheduleEnergy`. |
| **S2 relay** | `sensor.wallbox_s2_open` is derived from control-pilot telemetry (S2 is “closed” only while telemetry reports a charging state). | Falls back to `state.S2open` where telemetry is unavailable. |
| **Charging enable** | `sensor.wallbox_charging_enable` mirrors the telemetry `SENSOR_CHARGING_ENABLE` flag so toggles are instantaneous. | Falls back to `wallbox_config.charging_enable` on older firmware. |
| **Power Boost** | When telemetry reports a PowerBoost session, the L1 sensors publish the telemetry proposal current/power; unused phases report `0`. If legacy `m2w` data exists (older firmware / multi-phase setups) it’s used automatically. | Assumes single-phase hardware unless telemetry supplies per-phase values. |
| **Other telemetry** | `charging_power*`, `charging_current*`, `temp_l*`, `status`, `control_pilot`, `state_machine`, `charging_enable`, `cable_connected`, and all debug telemetry entities emit live telemetry values out of the box. | Legacy data paths remain in place for <6.7.x devices. |

> If you update your Wallbox beyond 6.7.x, simply redeploy using the installer command above to keep the telemetry fixes in place. The bridge auto-detects telemetry and switches to legacy data when telemetry is missing.

## Acknowledgments

The credits go out to jagheterfredrik (https://github.com/jagheterfredrik/wallbox-mqtt-bridge), who made the original MQTT Bridge for the Wallbox and [@jethrovo] for his updated version supporting version v6.6.x.
A big shoutout to [@tronikos](https://github.com/tronikos) for their valuable contributions. This project wouldn't be the same without the collaborative spirit of the open-source community.
