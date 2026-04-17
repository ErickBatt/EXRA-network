# Android Node MVP

This folder contains a minimal Android node client scaffold for exra.

## MVP behavior

- Foreground service keeps websocket session alive.
- Connects to backend `/ws`.
- Sends `register`, responds to `ping` with `pong`, emits periodic `traffic`.
- Reconnects with backoff.
- Stores `device_id` in shared preferences.

## Next steps

- Add full Gradle project files.
- Add UI polish and runtime permission handling.
- Add battery/network resilience policies.
