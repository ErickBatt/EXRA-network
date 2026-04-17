# TMA & Marketplace Hardening Plan

Started: 2026-04-17. Owner: hardening pass on Telegram Mini App + Marketplace
flows ahead of production.

This document is the source of truth for the multi-step hardening pass. It is
updated as work progresses so anyone (or a future Claude session) can pick up
where we left off without re-reading the whole conversation.

---

## 0. Context recap

EXRA is a DePIN proxy/compute marketplace. Three surfaces are in scope:

| Surface       | Stack                                  | Purpose                                  |
|---------------|----------------------------------------|------------------------------------------|
| TMA           | Next.js page + Telegram WebApp SDK     | Node owners view earnings, link devices, withdraw |
| Marketplace   | Next.js dashboard for buyers           | Buyers browse nodes, create offers, top up, run sessions |
| Server (Go)   | `gorilla/mux` HTTP + WS hub + Postgres | Auth, matchmaking, proxy data plane, payouts ledger |

On-chain interactions live in `server/peaq/peaq_client.go` and Substrate
pallet `peaq/pallets/exra/src/lib.rs` (4 extrinsics: `add_oracle`,
`batch_mint`, `stake_for_peak`, `update_reputations`).

---

## 1. Verified findings (after reading the code)

The first pass analysis listed P0s that turned out to already be done. After
reading the actual source these are the **real** P0/P1 items:

### Closed in commit `78600ca`

| # | Issue | Why it mattered | Fix |
|---|-------|-----------------|-----|
| S1 | `POST /claim/{did}` had no auth | Any caller could create a payout request for *any* DID and direct funds to their own `recipient_wallet`. Velocity lock did not help — the first call always succeeded. | Wrapped in `DIDAuth` middleware (signed node identity + 5-min replay window) and added a path-vs-header DID match check inside the handler. |
| S2 | `POST /api/tma/link-device` accepted attacker-controlled `telegram_id` in the body | Spam/abuse vector: any caller could spawn approval prompts at any node, impersonating any Telegram user, even setting fake `tg_user`/`tg_first_name`. | Now requires a signed Telegram `init_data`. Telegram identity (id, username, first name) is derived server-side from the signed `user` field. Frontend updated to match. |
| S3 | `verifyTelegramInitData` had no `auth_date` freshness check, used non–constant-time hash compare, and returned `(nil, nil)` on failure | Replay risk: leaked initData (debug log, screenshot of URL, etc.) was usable forever. The `(nil, nil)` shape made it easy for callers to forget the auth check. | Added 24h freshness window, `hmac.Equal`, typed errors, and a `telegramUserFromInitData` helper used by both auth handlers. |

Tests: 11/11 green in `server/handlers/`. Each fix has at least one negative
test asserting the request is rejected.

### Things that turned out to **not** be P0 (initial analysis was wrong)

- `runtimeHub` was already initialised — `handlers.SetHub(wsHub)` at
  `server/main.go:45`. Nil-pointer paranoia was unfounded.
- `TmaAuth` was already returning 401 on invalid initData. The earlier
  `(nil, nil)` shape was odd, but not wrong by itself.
- The WebSocket data plane is fully implemented in `proxy.go` with
  `TunnelManager.RequestTunnel/AwaitTunnel/RegisterTunnel`, hijack-based
  bidirectional `io.Copy`, failover, and proper session finalisation.

### Open work

| # | Issue | Severity | Plan |
|---|-------|----------|------|
| P1 | Payout lifecycle never reaches a terminal state | High | See §2 below — needs a design call before code |
| P1 | Marketplace buyer API key in `localStorage` | Medium-high | XSS risk; move to httpOnly cookie via Next.js route |
| P1 | Marketplace `Create Offer` form has no client-side validation | Medium | `parseFloat(...) || 0` accepts NaN; add positive-number guards; backend already rejects but UX is bad |
| P2 | `LiveMap` component is imported but missing | Low | Stub it out or delete the import |
| P2 | No tests for matcher/offer/pool/buyer handlers | Medium | Mirror `tma_test.go` patterns once flow is stable |
| P2 | Marketplace not responsive < 768px | Low | Sidebar collapses below the fold |

---

## 2. Payout lifecycle — design call needed before coding

This was originally written down as "wire pending payouts to a Peaq on-chain
worker". After reading the pallet that interpretation looks **wrong**, so I am
pausing for a design decision rather than building the wrong thing.

### What the code actually does today

```
TMA / external caller
        │
        ▼
POST /api/tma/withdraw  (or  POST /claim/{did} — now DIDAuth-protected)
        │
        ▼
models.ClaimPayout
  ├── velocity check (1 active lock per DID)
  ├── tier + tax calculation
  ├── balance check + FOR UPDATE
  ├── INSERT payout_requests   status='pending'
  ├── INSERT node_earnings (-amount)   ← off-chain ledger debit
  └── INSERT did_payout_velocity (timelock until eligible_at)
        │
        ▼
admin POST /api/payout/{id}/approve   ← only flips status to 'approved'
        │
        ▼
                ❓ nothing
```

`UpdatePayoutStatus` is called only with `'approved'` or `'rejected'`. No code
ever sets `'paid'`, `'completed'`, or attaches an on-chain tx hash. The
operator-facing balance is debited at request time, but the user never sees
funds arrive — there is no fulfilment step.

### What the Peaq pallet actually offers

```rust
// peaq/pallets/exra/src/lib.rs
call_index(0)  add_oracle             // governance
call_index(1)  batch_mint             // oracle mints EXRA to node accounts
call_index(2)  stake_for_peak         // node burns 100 EXRA → Peak tier
call_index(3)  update_reputations     // oracle pushes RS scores
```

There is **no** `transfer_payout`, `claim`, `withdraw`, or anything similar.
EXRA is minted directly to each node's `AccountId` via `batch_mint`, so the
on-chain payout actually happens *inside the daily oracle batch* — the node
operator already owns their EXRA on Peaq L1.

### Possible interpretations of `recipient_wallet`

| Interpretation | Implication | Status |
|----------------|-------------|--------|
| (a) External Peaq AccountId — operator wants to consolidate to a different on-chain wallet | Need either a `balances.transfer` wrapper in `peaq_client.go` or a new pallet extrinsic. Operator would normally do this themselves on-chain — server intervention is unusual. | unlikely |
| (b) Off-ramp address (CEX deposit, fiat partner, USDC on another chain) | Fulfilled manually by ops or a separate off-ramp service. Server's job is just ledger + intent + idempotency. Needs a `payout/{id}/mark-paid` admin endpoint with `tx_hash` / `provider_ref`. | most likely |
| (c) The operator's own AccountId — `recipient_wallet` is redundant | Then `payout_requests` is purely an audit/velocity ledger and should not debit `node_earnings` again (since `batch_mint` already credited the chain). | unlikely — would be a double-debit bug |

### My recommendation (before writing any code)

Treat the flow as **(b) — off-ramp intent ledger**. Concretely:

1. Add status values to the contract: `pending → approved → paid` (or
   `rejected`). `paid` is terminal and carries `tx_hash` + `paid_at`.
2. Add admin endpoint `POST /api/admin/payout/{id}/mark-paid` taking
   `{tx_hash, provider, note}`. Wraps `UpdatePayoutStatus` + a new column
   write. AdminFinance role only.
3. TMA shows `payout.status` and `tx_hash` to the user so they can verify.
4. (Later) If we *do* want server-side on-chain fulfilment, add a
   `balances.transfer` wrapper in `peaq_client.go` and a worker that picks up
   `approved` rows, submits the extrinsic, stores the hash, marks `paid`.

This avoids building a worker against a pallet method that doesn't exist
yet, while still closing the user-visible "I clicked withdraw and nothing
happened" gap.

**Decision needed from a human**: confirm interpretation (b) is correct, or
point me at the missing pallet/spec.

---

## 3. Step-by-step plan (post commit `78600ca`)

Order is chosen so each step is independently shippable and revertible.

### Step 4 — payout lifecycle (this is the next unit of work)

**Why:** today `pending` is a black hole. Even if interpretation (b) is wrong,
the missing terminal state is a real defect and the work below is reusable.

**Files I'll touch:**
- `server/migrations/` — new migration adding `tx_hash`, `provider`, `paid_at`,
  `payout_note` columns to `payout_requests`
- `server/models/payout.go` — `MarkPayoutPaid(id, txHash, provider, note)` and
  reject `mark-paid` on rows that aren't `approved`
- `server/handlers/admin.go` — new `AdminMarkPayoutPaid` handler
- `server/main.go` — register `POST /api/admin/payout/{id}/mark-paid` under
  `adminFinance` middleware
- `server/handlers/payout.go` — surface the new fields in `ListPayouts`
- `dashboard/app/admin/...` — add a simple "Mark paid" action with a tx-hash
  input (only if admin UI exists; else skip until UI work)
- `dashboard/app/tma/TMAApp.tsx` — show `tx_hash` if present in the withdraw
  history
- New test file `server/handlers/payout_admin_test.go` — covers happy path
  and rejecting `mark-paid` on a non-approved row

**Why not a migration-free implementation:** ledgers should be auditable;
adding columns is the cheapest way to keep tx hash and timestamp queryable.

**Open question I'll resolve before coding:** does the admin console already
exist? If yes, wire the action in; if no, expose only the API and surface
the hash in the TMA + Marketplace.

### Step 5 — marketplace API key storage

**Why:** `localStorage.getItem('exra_buyer_api_key')` is XSS-stealable. Any
malicious script in the dashboard or any sub-domain CDN can lift it and the
buyer's billing balance is exposed. Bearer tokens in the dashboard
specifically should not sit in JS-readable storage.

**Plan:**
1. Add Next.js route `app/api/buyer/key/route.ts` that:
   - GET: reads from an httpOnly, `SameSite=Strict`, `Secure` cookie
   - POST: writes the cookie, gated by Supabase session
   - DELETE: clears it on logout
2. Replace `fetchJson('/api/buyer/me', localApiKey)` with a server-proxied
   variant: dashboard calls `/next-buyer/me`, the route reads the cookie and
   forwards with `X-API-Key`. Mirrors the existing `next-tma` pattern.
3. Migrate existing localStorage on first load (read once, POST to set
   cookie, then `localStorage.removeItem`).

**Files I'll touch:**
- `dashboard/app/api/buyer/key/route.ts` (new)
- `dashboard/app/next-buyer/[...path]/route.ts` (new — mirror of next-tma)
- `dashboard/app/marketplace/page.tsx` — drop direct API key reads
- `dashboard/lib/api.ts` — make `fetchJson` work without the apiKey arg for
  buyer-scoped calls

**Why not just sessionStorage:** still JS-readable. httpOnly is the only
storage XSS cannot reach.

### Step 6 — marketplace offer form validation

**Why:** `parseFloat(e.target.value) || 0` lets users post `target_gb=0`
or `max_price_per_gb=0` to the backend. The backend has `validateFloat` for
some cases but the offer endpoint doesn't enforce it cleanly, and the UX is
broken even when the backend rejects.

**Plan:**
- Add inline error state for each field
- Disable `Create offer` button while invalid
- Trim country to ISO-2 (already uppercased)
- Backend: ensure `CreateOffer` runs `validateFloat` on both numeric fields

**Files I'll touch:**
- `dashboard/app/marketplace/page.tsx` (form section around line 410)
- `server/handlers/offer.go` — confirm or add `validateFloat` on inputs
- New `server/handlers/offer_test.go` — at least one negative test

### Step 7 — handler test coverage

Once the above land, add tests for the highest-blast-radius handlers that
currently have none:
- `matcher.go` — `BidScore` purity test, `CreateOfferAndMatch` happy path
- `offer.go` — input validation
- `pool.go` — join/leave + treasury fee math
- `buyer.go` — register/sync edge cases

These are pure additions; no code change required.

### Step 8 (deferred) — `LiveMap` component

`dashboard/components/LiveMap.tsx` is imported by `marketplace/page.tsx:10`
but doesn't exist. Either build a real one (canvas/SVG with node geo dots
streamed from `/ws/map` which exists at `server/main.go:88`) or stub it.

### Step 9 (deferred) — responsive marketplace

Sidebar is `position: fixed; width: 260px`. Below 768px the layout breaks.
Add a media query that collapses the sidebar to a top hamburger.

### Step 10 (deferred) — error boundaries + loading states

Both TMA and Marketplace will white-screen on a thrown render error. Wrap
both top-level pages in a React Error Boundary with a friendly fallback.

---

## 4. Out of scope for this pass

- Tokenomics rebalancing (separate doc: `docs/PROTOCOL_ECONOMY_SPEC.md`)
- Anything touching `peaq/pallets/` Rust code (separate change ticket)
- Mobile Android client (separate doc)
- Adding new pallet extrinsics (would block on a pallet upgrade vote)

---

## 5. How to verify each step

| Step | Verification |
|------|--------------|
| Auth fixes (done) | `go test ./handlers/ -count=1` — 11/11 |
| Payout lifecycle | New unit test + manual: create payout, approve, mark-paid, observe TMA shows `tx_hash` |
| API key storage | Open devtools → Application → Local Storage shows no `exra_buyer_api_key`. Cookie tab shows httpOnly entry. Marketplace still loads buyer profile. |
| Offer validation | Submit negative target_gb → button disabled with inline error. Submit valid → request goes through. |
| Handler tests | `go test ./... -count=1` — full green |

---

## 6. Update log

- **2026-04-17 — commit `78600ca`** — auth holes S1/S2/S3 closed, plan doc
  written, payout-lifecycle design call surfaced for human decision.
