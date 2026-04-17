# Release Checklist

## Backend

- [ ] DB migrations applied (`001` -> `003`).
- [ ] Secrets configured (`PROXY_SECRET`, `NODE_SECRET`).
- [ ] API health endpoint green.
- [ ] Smoke script passes.

## Dashboard

- [ ] `npm install` and `npm run build` pass.
- [ ] API base URL points to target backend.
- [ ] Buyer profile/sessions/topup pages return live data.

## Android

- [ ] APK builds successfully.
- [ ] Foreground service starts and reconnects.
- [ ] WS register/ping/pong/traffic events visible in backend logs.

## Payout/Economy

- [ ] Node earnings accumulate on traffic.
- [ ] Payout has no minimum threshold gate.
- [ ] Payout precheck returns gas/rent/net breakdown.
- [ ] Insufficient gas/rent path returns clear user-facing error.
- [ ] Admin approve flow updates status.
