# MQTT Bridge for Wallbox

This open-source project connects your Wallbox fully locally to Home Assistant, providing you with unparalleled speed and reliability.

Note: This fork is maintained at [Leventionz/wallbox-mqtt-bridge](https://github.com/Leventionz/wallbox-mqtt-bridge) and includes the telemetry-backed fixes required for firmware 6.7.x and newer.

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

- Firmware 6.7.x stops populating the legacy `m2w` Redis hashes for per-phase power, current, and temperatures.  
- This fork now taps into the Wallbox telemetry Redis channel (`/wbx/telemetry/events`) and maps those live sensor readings back into the existing Home Assistant entities.  
- No Home Assistant reconfiguration is required: the standard `charging_power*`, `charging_current*`, `temp_l*`, `status`, `control_pilot`, `state_machine`, `charging_enable`, and `cable_connected` entities now automatically emit the telemetry values (while older firmware still uses the legacy data paths).
- `sensor.wallbox_s2_open` is derived from the telemetry control-pilot status (S2 is considered closed only while telemetry reports `Charging`). Legacy `state` hash values are used only as a fallback.
- `sensor.wallbox_m2w_status` mirrors the telemetry-backed state machine to still get meaningful values even with `debug_sensors=true`.
- Power Boost sensors now use telemetry when available: on single-phase installations the L1 sensors publish the telemetry proposal current/power while the unused phases report `0`. Legacy `m2w` readings are used automatically on older firmware or multi-phase setups.
- If you update your Wallbox beyond 6.7.x, simply redeploy using the installer command above to keep the telemetry fixes in place.

## Acknowledgments

The credits go out to jagheterfredrik (https://github.com/jagheterfredrik/wallbox-mqtt-bridge), who made the original MQTT Bridge for the Wallbox and [@jethrovo] for his updated version supporting version v6.6.x.
A big shoutout to [@tronikos](https://github.com/tronikos) for their valuable contributions. This project wouldn't be the same without the collaborative spirit of the open-source community.
