# exra Roadmap

This roadmap is organized by delivery priority.

## Recent Change Notes (2026-04)

- TON migration executed in backend/runtime docs and code paths (replacing Solana-first wording).
- Canonical economics source fixed in `docs/PROTOCOL_ECONOMY_SPEC.md` (hard cap, halving, finalized policy).
- Marketplace flow implemented: buyer offer -> oracle assign -> locked-price settlement.
- Oracle queue operability added: retries, DLQ fields, monitor endpoint, manual retry endpoint.
- Instant swap protections added via swap circuit breaker (volatility guard).

## P0 - Correctness and Safety [DONE]

- [x] Make session finalization and buyer charging atomic.
- [x] Prevent double charging for the same session.
- [x] Guarantee non-negative buyer balance at write-time.
- [x] Unify billing behavior between `/proxy` and `/api/session/{id}/end`.
- [x] Add comprehensive regression checks (Unit + Integration tests).

## P1 - Reliability and Operability [DONE]

- [x] Add graceful shutdown and server timeouts.
- [x] Improve proxy relay robustness (bidirectional close and error handling).
- [x] Add structured logging for sessions and billing events.
- [x] Add robust Health Check and Prometheus Metrics.
- [ ] Replace runtime schema bootstrap with explicit migrations.

## P2 - Product and Scale [IN PROGRESS]

- [x] Add rate limits and abuse protections.
- [x] Add passive-income Proof of Presence reward engine (PoP 3-stream split).
- [x] Prepare node mesh for Proxy + Edge AI dual workload (Compute Marketplace).
- [x] Android Foreground Service baseline.
- [ ] Add node selection strategy improvements (quality/load aware).
- [ ] Introduce admin reporting endpoints for usage and revenue.
- [ ] Expand API validation and error contract consistency.

## P3 - Tokenomics and Onchain Economy [IN PROGRESS]

- [x] Finalize deterministic off-chain earnings ledger with policy snapshots.
- [ ] Add node quality/fraud gates (tiering + ASN + velocity checks).
- [ ] Integrate TON contract adaptation for `mint_reward` and `burn_fee`.
- [x] Add reconciliation and retry workflow for backend accounting vs onchain queue states.
- [x] Implement buyer payment loop (`USDT`/`EXRA`) with burn/swap policy guards.

## Definition of Done (Current Phase)

- Billing for a session is applied at most once.
- Endpoints keep session and balance state consistent under retries.
- Core usage flow has reproducible checks and documented runbook.
- Payout follows True Crypto Spirit rule: no minimum threshold, only network fee feasibility.
