package tests

import (
	"encoding/hex"
	"exra/db"
	"exra/models"
	"exra/peaq"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPeaqClient is a mock implementation of peaq.BlockchainClient
type MockPeaqClient struct {
	mock.Mock
}

func (m *MockPeaqClient) SendBatchMint(batchID []byte, rewards []peaq.RewardEntry, sigs []peaq.OracleSignature) (string, error) {
	args := m.Called(batchID, rewards, sigs)
	return args.String(0), args.Error(1)
}

func TestE2EPeaqConsensusAndMint(t *testing.T) {
	// 1. Setup DB Mock
	mockDB, mockSQL, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	// 2. Setup Peaq Client Mock
	mockPeaq := new(MockPeaqClient)
	models.SetPeaqClient(mockPeaq)

	batchDate := "2026-04-15"
	payloadHash := "abc123hash"
	mockTxHash := "0xextrinsic_hash_123"

	// EXPECTATIONS for TriggerBatchMint
	// 1. Fetch batch
	mockSQL.ExpectQuery(regexp.QuoteMeta("SELECT id, batch_json FROM oracle_batches")).
		WithArgs(batchDate, payloadHash).
		WillReturnRows(sqlmock.NewRows([]string{"id", "batch_json"}).
			AddRow(1, []byte(`{"did1": 10.5, "did2": 5.0}`)))

	// 2. Fetch signatures
	mockSQL.ExpectQuery(regexp.QuoteMeta("SELECT oracle_did, signature FROM oracle_signatures")).
		WillReturnRows(sqlmock.NewRows([]string{"oracle_did", "signature"}).
			AddRow("0x01", hex.EncodeToString(make([]byte, 64))).
			AddRow("0x02", hex.EncodeToString(make([]byte, 64))))

	// 3. Mock Blockchain call
	mockPeaq.On("SendBatchMint", []byte(batchDate), mock.Anything, mock.Anything).Return(mockTxHash, nil)

	// 4. Update earnings
	mockSQL.ExpectExec(regexp.QuoteMeta("UPDATE node_earnings")).
		WithArgs(int64(1), batchDate).
		WillReturnResult(sqlmock.NewResult(0, 2))

	// 5. Update batch status
	mockSQL.ExpectExec(regexp.QuoteMeta("UPDATE oracle_batches")).
		WithArgs(mockTxHash, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// EXECUTE
	models.TriggerBatchMint(batchDate, payloadHash)

	// ASSERTIONS
	assert.NoError(t, mockSQL.ExpectationsWereMet())
	mockPeaq.AssertExpectations(t)
}

func TestE2EConsensusTrigger(t *testing.T) {
	mockDB, mockSQL, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()
	
	// Mock Peaq to avoid failures, but we don't necessarily call it if we don't reach threshold
	mockPeaq := new(MockPeaqClient)
	models.SetPeaqClient(mockPeaq)

	batchDate := "2026-04-16"
	payloadHash := "hash-789"
	
	// Set threshold for 3 nodes: (3*2)/3 = 2.
	// Scenario: We have 2 signatures received.
	
	// 1. CheckOracleConsensus verifies count
	mockSQL.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM oracle_batches")).
		WithArgs(batchDate, payloadHash).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	// 2. Mark as consensus
	mockSQL.ExpectExec(regexp.QuoteMeta("UPDATE oracle_batches")).
		WithArgs(batchDate, payloadHash).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// 3. TriggerBatchMint will be called inside, so we need its expectations too
	mockSQL.ExpectQuery(regexp.QuoteMeta("SELECT id, batch_json FROM oracle_batches")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "batch_json"}).AddRow(100, []byte(`{}`)))
	mockSQL.ExpectQuery(regexp.QuoteMeta("SELECT oracle_did, signature FROM oracle_signatures")).
		WillReturnRows(sqlmock.NewRows([]string{"oracle_did", "signature"}).AddRow("0x01", hex.EncodeToString(make([]byte, 64))))
	
	mockPeaq.On("SendBatchMint", mock.Anything, mock.Anything, mock.Anything).Return("0xhash", nil)
	
	mockSQL.ExpectExec(regexp.QuoteMeta("UPDATE node_earnings")).WillReturnResult(sqlmock.NewResult(0, 0))
	mockSQL.ExpectExec(regexp.QuoteMeta("UPDATE oracle_batches")).WillReturnResult(sqlmock.NewResult(0, 1))

	// RUN
	models.CheckOracleConsensus(batchDate, payloadHash, 3)

	assert.NoError(t, mockSQL.ExpectationsWereMet())
}
