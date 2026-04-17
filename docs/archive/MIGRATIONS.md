# Migrations

exra stores SQL migrations in `server/migrations`.

## Files

- `001_init.sql` - baseline schema for nodes, buyers, sessions, usage logs.
- `002_node_compat.sql` - compatibility updates for device-centric ws flow and billing constraints.
- `003_payouts.sql` - earnings and payout request tables.
- `004_tokenomics_t1_t2_t3_t4.sql` - tokenomics columns, reward audit trail, oracle queue, burn events.
- `005_true_crypto_spirit.sql` - payout fee breakdown columns for no-minimum withdrawal model.

## Apply manually (PostgreSQL)

Example:

1. Connect to your database with `psql`.
2. Run:
   - `\i server/migrations/001_init.sql`
   - `\i server/migrations/002_node_compat.sql`
   - `\i server/migrations/003_payouts.sql`
   - `\i server/migrations/004_tokenomics_t1_t2_t3_t4.sql`
   - `\i server/migrations/005_true_crypto_spirit.sql`

## Runtime compatibility

The backend still keeps runtime compatibility checks in `server/db/db.go` for existing environments.
For production, prefer applying migrations before app startup.
