package models

import (
	"errors"
	"exra/db"
	"time"
)

type NodeEarningsSummary struct {
	DeviceID   string  `json:"device_id"`
	TotalBytes int64   `json:"total_bytes"`
	TotalUSD   float64 `json:"total_usd"`
}

type PayoutFeeBreakdown struct {
	GasFeeChain     float64 `json:"gas_fee_chain"`
	StorageFeeChain float64 `json:"storage_fee_chain"`
	TotalFeeChain   float64 `json:"total_fee_chain"`
	WalletReady     bool    `json:"wallet_ready"`
}

type PayoutPrecheck struct {
	DeviceID           string             `json:"device_id"`
	RecipientWallet    string             `json:"recipient_wallet"`
	RequestedAmountUSD float64            `json:"requested_amount_usd"`
	BalanceUSD         float64            `json:"balance_usd"`
	Fees               PayoutFeeBreakdown `json:"fees"`
	TotalFeeUSD        float64            `json:"total_fee_usd"`
	NetAmountUSD       float64            `json:"net_amount_usd"`
	CanPayout          bool               `json:"can_payout"`
	Alert              string             `json:"alert,omitempty"`
}

type PayoutRequest struct {
	ID                string    `json:"id"`
	DeviceID          string    `json:"device_id"`
	RecipientWallet   string    `json:"recipient_wallet"`
	AmountUSD         float64   `json:"amount_usd"`
	GasFeeChain       float64   `json:"gas_fee_chain"`
	StorageFeeChain   float64   `json:"storage_fee_chain"`
	TotalFeeChain     float64   `json:"total_fee_chain"`
	NetAmountUSD      float64   `json:"net_amount_usd"`
	WalletInitialized bool      `json:"wallet_initialized"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type OracleMintItem struct {
	ID            int64   `json:"id"`
	RewardEventID int64   `json:"reward_event_id"`
	DeviceID      string  `json:"device_id"`
	AmountExra    float64 `json:"amount_exra"`
	Status        string  `json:"status"`
}

type OracleQueueItem struct {
	ID          int64      `json:"id"`
	DeviceID    string     `json:"device_id"`
	AmountExra  float64    `json:"amount_exra"`
	Status      string     `json:"status"`
	TxSignature string     `json:"tx_signature"`
	ErrorText   string     `json:"error_text"`
	RetryCount  int        `json:"retry_count"`
	NextRetryAt *time.Time `json:"next_retry_at,omitempty"`
	DLQReason   string     `json:"dlq_reason"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
type TokenomicsStats struct {
	TotalExraPendingMint float64 `json:"total_exra_pending_mint"`
	TotalExraMinted      float64 `json:"total_exra_minted"`
	TotalExraBurned      float64 `json:"total_exra_burned"`
	TreasuryUSDCBalance  float64 `json:"treasury_usdc_balance"`
}

type SwapQuote struct {
	DeviceID      string  `json:"device_id"`
	ExraAmount    float64 `json:"exra_amount"`
	UsdcReceived  float64 `json:"usdc_received"`
	SpreadUSD     float64 `json:"spread_usd"`
	TreasuryFloor bool    `json:"treasury_floor_reached"`
}

// GetAvgDailyMintRate returns the average EXRA minted per day over the last 7 days.
// Returns 0 if there is not enough data (< 2 days of history).
func GetAvgDailyMintRate() float64 {
	var rate float64
	err := db.DB.QueryRow(`
		SELECT COALESCE(SUM(amount_exra) / NULLIF(
			EXTRACT(EPOCH FROM (NOW() - MIN(created_at))) / 86400.0,
		0), 0)
		FROM oracle_mint_queue
		WHERE status IN ('confirmed', 'minted')
		  AND created_at > NOW() - INTERVAL '7 days'
	`).Scan(&rate)
	if err != nil {
		return 0
	}
	return rate
}

func GetNodeEarnings(deviceID string) (*NodeEarningsSummary, error) {
	out := &NodeEarningsSummary{DeviceID: deviceID}
	err := db.DB.QueryRow(
		`SELECT COALESCE(SUM(bytes), 0), COALESCE(SUM(earned_usd), 0)
		 FROM node_earnings
		 WHERE device_id = $1`,
		deviceID,
	).Scan(&out.TotalBytes, &out.TotalUSD)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func BuildPayoutPrecheck(deviceID, recipientWallet string, amountUSD float64, balanceUSD float64, fees PayoutFeeBreakdown) *PayoutPrecheck {
	// In Peaq V2, fees are in USD equivalents for UI consistency or handled on-chain.
	totalFeeUSD := fees.TotalFeeChain // chain fee is now already normalized to USD or 1:1 EXRA
	net := amountUSD - totalFeeUSD
	can := balanceUSD >= totalFeeUSD && net > 0
	alert := ""
	if !can {
		alert = "Баланса недостаточно для оплаты газа"
	}
	return &PayoutPrecheck{
		DeviceID:           deviceID,
		RecipientWallet:    recipientWallet,
		RequestedAmountUSD: amountUSD,
		BalanceUSD:         balanceUSD,
		Fees:               fees,
		TotalFeeUSD:        totalFeeUSD,
		NetAmountUSD:       net,
		CanPayout:          can,
		Alert:              alert,
	}
}

func CreatePayoutRequest(precheck *PayoutPrecheck) (*PayoutRequest, error) {
	req := &PayoutRequest{}
	err := db.DB.QueryRow(
		`INSERT INTO payout_requests (device_id, recipient_wallet, amount_usd, gas_fee_chain, storage_fee_chain, total_fee_chain, net_amount_usd, wallet_initialized, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending')
		 RETURNING id, device_id, recipient_wallet, amount_usd, gas_fee_chain, storage_fee_chain, total_fee_chain, net_amount_usd, wallet_initialized, status, created_at, updated_at`,
		precheck.DeviceID, precheck.RecipientWallet, precheck.RequestedAmountUSD, precheck.Fees.GasFeeChain, precheck.Fees.StorageFeeChain, precheck.Fees.TotalFeeChain, precheck.NetAmountUSD, precheck.Fees.WalletReady,
	).Scan(
		&req.ID, &req.DeviceID, &req.RecipientWallet, &req.AmountUSD, &req.GasFeeChain, &req.StorageFeeChain, &req.TotalFeeChain, &req.NetAmountUSD, &req.WalletInitialized, &req.Status, &req.CreatedAt, &req.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return req, nil
}

// CreatePayoutRequestAtomic checks balance and creates the payout request inside a single
// serializable transaction using SELECT FOR UPDATE to prevent TOCTOU double-withdrawal.
// Also enforces a 24-hour velocity limit: one payout request per device per 24 hours.
func CreatePayoutRequestAtomic(precheck *PayoutPrecheck) (*PayoutRequest, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Velocity limit: block if a payout was requested in the last 24 hours.
	var recentCount int
	err = tx.QueryRow(
		`SELECT COUNT(*) FROM payout_requests
		 WHERE device_id = $1
		   AND status NOT IN ('rejected', 'failed')
		   AND created_at > NOW() - INTERVAL '24 hours'`,
		precheck.DeviceID,
	).Scan(&recentCount)
	if err != nil {
		return nil, err
	}
	if recentCount > 0 {
		return nil, errors.New("payout velocity limit: one withdrawal per 24 hours per device")
	}

	// Lock node_earnings rows for this device to prevent concurrent payouts.
	var totalUSD float64
	err = tx.QueryRow(
		`SELECT COALESCE(SUM(earned_usd), 0)
		 FROM node_earnings
		 WHERE device_id = $1
		 FOR UPDATE`,
		precheck.DeviceID,
	).Scan(&totalUSD)
	if err != nil {
		return nil, err
	}

	if totalUSD < precheck.RequestedAmountUSD {
		return nil, errors.New("insufficient earned balance")
	}

	req := &PayoutRequest{}
	err = tx.QueryRow(
		`INSERT INTO payout_requests (device_id, recipient_wallet, amount_usd, gas_fee_chain, storage_fee_chain, total_fee_chain, net_amount_usd, wallet_initialized, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending')
		 RETURNING id, device_id, recipient_wallet, amount_usd, gas_fee_chain, storage_fee_chain, total_fee_chain, net_amount_usd, wallet_initialized, status, created_at, updated_at`,
		precheck.DeviceID, precheck.RecipientWallet, precheck.RequestedAmountUSD,
		precheck.Fees.GasFeeChain, precheck.Fees.StorageFeeChain, precheck.Fees.TotalFeeChain,
		precheck.NetAmountUSD, precheck.Fees.WalletReady,
	).Scan(
		&req.ID, &req.DeviceID, &req.RecipientWallet, &req.AmountUSD,
		&req.GasFeeChain, &req.StorageFeeChain, &req.TotalFeeChain, &req.NetAmountUSD,
		&req.WalletInitialized, &req.Status, &req.CreatedAt, &req.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return req, nil
}

func UpdatePayoutStatus(id string, status string) error {
	_, err := db.DB.Exec(
		`UPDATE payout_requests
		 SET status = $1, updated_at = NOW()
		 WHERE id = $2`,
		status, id,
	)
	return err
}

func ListPayoutRequests(limit int) ([]PayoutRequest, error) {
	rows, err := db.DB.Query(
		`SELECT id, device_id, recipient_wallet, amount_usd, gas_fee_chain, storage_fee_chain, total_fee_chain, net_amount_usd, wallet_initialized, status, created_at, updated_at
		 FROM payout_requests
		 ORDER BY created_at DESC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PayoutRequest
	for rows.Next() {
		var p PayoutRequest
		if err := rows.Scan(&p.ID, &p.DeviceID, &p.RecipientWallet, &p.AmountUSD, &p.GasFeeChain, &p.StorageFeeChain, &p.TotalFeeChain, &p.NetAmountUSD, &p.WalletInitialized, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func ListPendingOracleMints(limit int) ([]OracleMintItem, error) {
	rows, err := db.DB.Query(
		`SELECT id, reward_event_id, device_id, amount_exra, status
		 FROM oracle_mint_queue
		 WHERE status = 'pending'
		 ORDER BY created_at ASC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OracleMintItem
	for rows.Next() {
		var item OracleMintItem
		if err := rows.Scan(&item.ID, &item.RewardEventID, &item.DeviceID, &item.AmountExra, &item.Status); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func MarkOracleMintResult(id int64, status, signature, errorText string) error {
	_, err := db.DB.Exec(
		`UPDATE oracle_mint_queue
		 SET status = $1, tx_signature = $2, error_text = $3, updated_at = NOW()
		 WHERE id = $4`,
		status, signature, errorText, id,
	)
	return err
}

func ListOracleQueue(limit int) ([]OracleQueueItem, error) {
	rows, err := db.DB.Query(
		`SELECT id, device_id, amount_exra, status, COALESCE(tx_signature,''), COALESCE(error_text,''), COALESCE(retry_count,0), next_retry_at, COALESCE(dlq_reason,''), created_at, updated_at
		 FROM oracle_mint_queue
		 ORDER BY created_at DESC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]OracleQueueItem, 0)
	for rows.Next() {
		var item OracleQueueItem
		if err := rows.Scan(&item.ID, &item.DeviceID, &item.AmountExra, &item.Status, &item.TxSignature, &item.ErrorText, &item.RetryCount, &item.NextRetryAt, &item.DLQReason, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// AuditMintEntry is a public-safe view of an oracle mint — no error details, truncated device ID.
type AuditMintEntry struct {
	ID         int64     `json:"id"`
	DeviceID   string    `json:"device_id"` // truncated: first 8 chars + "***"
	AmountExra float64   `json:"amount_exra"`
	Status     string    `json:"status"`
	TxHash     string    `json:"tx_hash,omitempty"`
	ChainURL   string    `json:"chain_url,omitempty"`
	MintedAt   time.Time `json:"minted_at"`
}

// ListPublicMintAudit returns confirmed/minted entries for the public audit log.
func ListPublicMintAudit(limit int) ([]AuditMintEntry, error) {
	rows, err := db.DB.Query(`
		SELECT id, device_id, amount_exra, status, COALESCE(tx_signature,''), COALESCE(confirmed_at, created_at)
		FROM oracle_mint_queue
		WHERE status IN ('confirmed','minted')
		ORDER BY COALESCE(confirmed_at, created_at) DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AuditMintEntry, 0)
	for rows.Next() {
		var e AuditMintEntry
		var rawDeviceID string
		if err := rows.Scan(&e.ID, &rawDeviceID, &e.AmountExra, &e.Status, &e.TxHash, &e.MintedAt); err != nil {
			return nil, err
		}
		// Truncate device ID for privacy
		if len(rawDeviceID) > 8 {
			e.DeviceID = rawDeviceID[:8] + "***"
		} else {
			e.DeviceID = rawDeviceID
		}
		if e.TxHash != "" {
			e.ChainURL = "https://peaq.subscan.io/extrinsic/" + e.TxHash
		}
		out = append(out, e)
	}
	return out, nil
}

func RetryOracleMintNow(id int64) error {
	_, err := db.DB.Exec(
		`UPDATE oracle_mint_queue
		 SET status='pending', next_retry_at=NOW(), error_text='', dlq_reason='', updated_at=NOW()
		 WHERE id = $1`,
		id,
	)
	return err
}

func RecordBurnEvent(buyerID string, inputCurrency string, inputAmount, ExraBought, ExraBurned float64) error {
	_, err := db.DB.Exec(
		`INSERT INTO burn_events (buyer_id, input_currency, input_amount, exra_bought, exra_burned)
		 VALUES (NULLIF($1, '')::uuid, $2, $3, $4, $5)`,
		buyerID, inputCurrency, inputAmount, ExraBought, ExraBurned,
	)
	return err
}

func GetTokenomicsStats() (*TokenomicsStats, error) {
	out := &TokenomicsStats{}
	err := db.DB.QueryRow(
		`SELECT
		   COALESCE(SUM(amount_exra) FILTER (WHERE status = 'pending'), 0) AS pending,
		   COALESCE(SUM(amount_exra) FILTER (WHERE status = 'minted'), 0) AS minted,
		   COALESCE((SELECT usdc_balance FROM treasury_vault LIMIT 1), 0) AS vault
		 FROM oracle_mint_queue`,
	).Scan(&out.TotalExraPendingMint, &out.TotalExraMinted, &out.TreasuryUSDCBalance)
	if err != nil {
		return nil, err
	}
	// Burned amount compatibility for multiple historical schemas.
	if err := db.DB.QueryRow(`SELECT COALESCE(SUM(exra_burned), 0) FROM burn_events`).Scan(&out.TotalExraBurned); err != nil {
		if err2 := db.DB.QueryRow(`SELECT COALESCE(SUM("EXRA_burned"), 0) FROM burn_events`).Scan(&out.TotalExraBurned); err2 != nil {
			out.TotalExraBurned = 0
		}
	}
	return out, nil
}

func ExecuteSwap(deviceID string, exraAmount float64, priceUSD float64) (*SwapQuote, error) {
	if ok, reason := CheckAndUpdateSwapGuard(priceUSD); !ok {
		return &SwapQuote{DeviceID: deviceID, ExraAmount: exraAmount, TreasuryFloor: true}, errors.New(reason)
	}
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Check current Treasury Floor
	var vaultBalance float64
	err = tx.QueryRow(`SELECT usdc_balance FROM treasury_vault FOR UPDATE`).Scan(&vaultBalance)
	if err != nil {
		return nil, err
	}

	rawUSD := exraAmount * priceUSD
	spread := rawUSD * 0.10 // 10% Treasury spread/profit
	received := rawUSD - spread

	if vaultBalance < received {
		return &SwapQuote{DeviceID: deviceID, ExraAmount: exraAmount, TreasuryFloor: true}, nil
	}

	// 2. Perform Swap
	// Update Vault
	_, err = tx.Exec(`UPDATE treasury_vault SET usdc_balance = usdc_balance - $1`, received)
	if err != nil {
		return nil, err
	}

	// Record Swap Event
	_, err = tx.Exec(
		`INSERT INTO swap_events (device_id, exra_amount, usdc_amount, spread_usd) VALUES ($1, $2, $3, $4)`,
		deviceID, exraAmount, received, spread,
	)
	if err != nil {
		return nil, err
	}

	// Reduce user balance (Burn/Return $EXRA to Treasury)
	// In MVP, we mock the on-chain burn by just recording the swap.
	// Users get USDC added to their withdrawable local balance.
	_, err = tx.Exec(
		`INSERT INTO node_earnings (device_id, bytes, earned_usd) VALUES ($1, 0, $2)`,
		deviceID, received,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &SwapQuote{
		DeviceID:      deviceID,
		ExraAmount:    exraAmount,
		UsdcReceived:  received,
		SpreadUSD:     spread,
		TreasuryFloor: false,
	}, nil
}

// -----------------------------------------------------------------------------
// Payout v2 (PEAQ DePIN Tokenomics)
// -----------------------------------------------------------------------------

func GetPayoutDIDVelocity(did string) (bool, error) {
	var activeLocks int
	err := db.DB.QueryRow(
		`SELECT COUNT(*) FROM did_payout_velocity WHERE did = $1 AND eligible_at > NOW()`,
		did,
	).Scan(&activeLocks)
	if err != nil {
		return false, err
	}
	// Eligible if there are 0 active timelocked payouts. 1 payout per 24h allowed.
	return activeLocks == 0, nil
}

func CalculateAnonTax(amount float64, tier string) float64 {
	if tier == "anon" {
		return amount * 0.25
	}
	return 0.0 // peak pays 0%
}

func GetDecayBoost(did string) (int, error) {
	// Every 7 days of honest_days_streak reduces timelock by 4h
	var streak int
	err := db.DB.QueryRow(`SELECT COALESCE(MAX(honest_days_streak), 0) FROM nodes WHERE did = $1`, did).Scan(&streak)
	if err != nil {
		return 0, err
	}
	boost := (streak / 7) * 4
	return boost, nil
}

func ApplyTimelock(did string) (int, error) {
	var tier string
	var baseTimelock int
	err := db.DB.QueryRow(
		`SELECT identity_tier, MAX(timelock_hours) FROM nodes WHERE did = $1 GROUP BY identity_tier LIMIT 1`,
		did,
	).Scan(&tier, &baseTimelock)
	
	if err != nil {
		return 24, err
	}
	
	if tier == "peak" {
		return 0, nil
	}
	
	boost, err := GetDecayBoost(did)
	if err != nil {
		boost = 0
	}
	
	finalTimelock := baseTimelock - boost
	if finalTimelock < 0 {
		finalTimelock = 0
	}
	return finalTimelock, nil
}

// ClaimPayout wraps the atomic payout creation with the V2 DID velocity, tax, and timelock logic.
func ClaimPayout(did string, amountUSD float64, recipientWallet string) (*PayoutRequest, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Velocity limit check
	var activeLocks int
	err = tx.QueryRow(`SELECT COUNT(*) FROM did_payout_velocity WHERE did = $1 AND eligible_at > NOW() FOR UPDATE`, did).Scan(&activeLocks)
	if err != nil {
		return nil, err
	}
	if activeLocks > 0 {
		return nil, errors.New("velocity limit: DID has a locked payout window")
	}

	// 2. Fetch tier & node ID
	var tier string
	var deviceID string
	err = tx.QueryRow(`SELECT identity_tier, device_id FROM nodes WHERE did = $1 LIMIT 1`, did).Scan(&tier, &deviceID)
	if err != nil {
		return nil, errors.New("invalid DID or no nodes attached")
	}

	// 3. Tax & Timelock
	taxAmount := CalculateAnonTax(amountUSD, tier)
	netAmount := amountUSD - taxAmount

	boost, _ := GetDecayBoost(did)
	baseTimelock := 24
	if tier == "peak" {
		baseTimelock = 0
	}
	finalTimelock := baseTimelock - boost
	if finalTimelock < 0 {
		finalTimelock = 0
	}

	// 4. Check and Lock Balance
	var totalUSD float64
	err = tx.QueryRow(
		`SELECT COALESCE(SUM(earned_usd), 0)
		 FROM node_earnings
		 WHERE device_id IN (SELECT device_id FROM nodes WHERE did = $1)
		 FOR UPDATE`, did,
	).Scan(&totalUSD)
	if err != nil {
		return nil, err
	}
	if totalUSD < amountUSD {
		return nil, errors.New("insufficient earned balance across DID")
	}

	// 5. Create Payout Request
	req := &PayoutRequest{}
	err = tx.QueryRow(
		`INSERT INTO payout_requests (device_id, recipient_wallet, amount_usd, gas_fee_chain, storage_fee_chain, total_fee_chain, net_amount_usd, wallet_initialized, status)
		 VALUES ($1, $2, $3, 0, 0, 0, $4, true, 'pending')
		 RETURNING id, device_id, recipient_wallet, amount_usd, gas_fee_chain, storage_fee_chain, total_fee_chain, net_amount_usd, wallet_initialized, status, created_at, updated_at`,
		deviceID, recipientWallet, amountUSD, netAmount,
	).Scan(&req.ID, &req.DeviceID, &req.RecipientWallet, &req.AmountUSD, &req.GasFeeChain, &req.StorageFeeChain, &req.TotalFeeChain, &req.NetAmountUSD, &req.WalletInitialized, &req.Status, &req.CreatedAt, &req.UpdatedAt)
	if err != nil {
		return nil, err
	}

	// 6. Deduct balance from primary device
	_, err = tx.Exec(
		`INSERT INTO node_earnings (device_id, bytes, earned_usd) VALUES ($1, 0, $2)`,
		deviceID, -amountUSD,
	)
	if err != nil {
		return nil, err
	}

	// 7. Insert Velocity Lock
	eligibleAt := time.Now().Add(time.Duration(finalTimelock) * time.Hour)
	_, err = tx.Exec(
		`INSERT INTO did_payout_velocity (did, payout_id, amount_before_tax, tax_amount, net_amount, tier_at_payout, timelock_hours_at_payout, eligible_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		did, req.ID, amountUSD, taxAmount, netAmount, tier, finalTimelock, eligibleAt,
	)
	if err != nil {
		return nil, err
	}

	return req, tx.Commit()
}
