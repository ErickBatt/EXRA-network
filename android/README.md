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

## Toolchain (post-2026-04-17)

- Kotlin `2.2.10` + Compose plugin `org.jetbrains.kotlin.plugin.compose:2.2.10`
- AGP `9.1.1`, Gradle wrapper `9.3.1`
- Compose BOM `2024.12.01`, coroutines `1.9.0`
- Substrate SDK: `store.silencio:peaqsdk:1.0.15` — пакеты `dev.sublab.*` (не Nova `io.novasama.*`).
  SDK требует Kotlin ≥ 2.3 metadata-reader, поэтому downgrade на Kotlin 1.x невозможен.
- `bcprov-jdk15on` глобально исключён (дубли с `bcprov-jdk15to18:1.78.1`).

Подробности — в корневом `AGENTS.md`, секция 14.
