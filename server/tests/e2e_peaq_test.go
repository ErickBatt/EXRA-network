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

// MockPeaqClient implements peaq.BlockchainClient for v2.4.1 interface.
type MockPeaqClient struct {
	mock.Mock
}

func (m *MockPeaqClient) SendBatchMint(batchID [32]byte, claims []peaq.ClaimEntry, sigs []peaq.IndexedSignature) (string, error) {
	args := m.Called(batchID, claims, sigs)
	return args.String(0), args.Error(1)
}

func (m *MockPeaqClient) SendUpdateStats(batchID [32]byte, entries []peaq.StatEntry, sigs []peaq.IndexedSignature) (string, error) {
	args := m.Called(batchID, entries, sigs)
	return args.String(0), args.Error(1)
}

func (m *MockPeaqClient) GetOracleSet() ([][32]byte, error) {
	args := m.Called()
	return args.Get(0).([][32]byte), args.Error(1)
}

func TestE2EPeaqConsensusAndMint(t *testing.T) {
	mockDB, mockSQL, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	mockPeaq := new(MockPeaqClient)
	models.SetPeaqClient(mockPeaq)

	batchDate := "2026-04-15"
	payloadHash := "abc123hash"
	mockTxHash := "0xextrinsic_hash_123"

	// Oracle set: two mock oracle public keys at index 0 and 1.
	oracleKey0 := [32]byte{0x01}
	oracleKey1 := [32]byte{0x02}
	mockPeaq.On("GetOracleSet").Return([][32]byte{oracleKey0, oracleKey1}, nil)

	// 1. Fetch batch
	mockSQL.ExpectQuery(regexp.QuoteMeta("SELECT id, batch_json FROM oracle_batches")).
		WithArgs(batchDate, payloadHash).
		WillReturnRows(sqlmock.NewRows([]string{"id", "batch_json"}).
			AddRow(1, []byte(`{"did1": 10.5, "did2": 5.0}`)))

	// 2. Fetch signatures — DIDs match oracle key hex encodings.
	mockSQL.ExpectQuery(regexp.QuoteMeta("SELECT oracle_did, signature FROM oracle_signatures")).
		WillReturnRows(sqlmock.NewRows([]string{"oracle_did", "signature"}).
			AddRow(hex.EncodeToString(oracleKey0[:]), hex.EncodeToString(make([]byte, 64))).
			AddRow(hex.EncodeToString(oracleKey1[:]), hex.EncodeToString(make([]byte, 64))))

	// 3. batchID is now sha256([batchDate]) — use mock.Anything to avoid computing it here.
	mockPeaq.On("SendBatchMint", mock.Anything, mock.Anything, mock.Anything).Return(mockTxHash, nil)

	// 4. Update earnings
	mockSQL.ExpectExec(regexp.QuoteMeta("UPDATE node_earnings")).
		WithArgs(int64(1), batchDate).
		WillReturnResult(sqlmock.NewResult(0, 2))

	// 5. Update batch status
	mockSQL.ExpectExec(regexp.QuoteMeta("UPDATE oracle_batches")).
		WithArgs(mockTxHash, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	models.TriggerBatchMint(batchDate, payloadHash)

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

	mockPeaq := new(MockPeaqClient)
	models.SetPeaqClient(mockPeaq)

	batchDate := "2026-04-16"
	payloadHash := "hash-789"

	// Threshold: ceil(2*3/3) = 2 for N=3. Scenario: 2 signatures received.

	// 1. CheckOracleConsensus count
	mockSQL.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM oracle_batches")).
		WithArgs(batchDate, payloadHash).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	// 2. Mark consensus
	mockSQL.ExpectExec(regexp.QuoteMeta("UPDATE oracle_batches")).
		WithArgs(batchDate, payloadHash).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// 3. TriggerBatchMint chain
	oracleKey0 := [32]byte{0x01}
	mockPeaq.On("GetOracleSet").Return([][32]byte{oracleKey0}, nil)

	mockSQL.ExpectQuery(regexp.QuoteMeta("SELECT id, batch_json FROM oracle_batches")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "batch_json"}).AddRow(100, []byte(`{}`)))
	mockSQL.ExpectQuery(regexp.QuoteMeta("SELECT oracle_did, signature FROM oracle_signatures")).
		WillReturnRows(sqlmock.NewRows([]string{"oracle_did", "signature"}).
			AddRow(hex.EncodeToString(oracleKey0[:]), hex.EncodeToString(make([]byte, 64))))

	mockPeaq.On("SendBatchMint", mock.Anything, mock.Anything, mock.Anything).Return("0xhash", nil)

	mockSQL.ExpectExec(regexp.QuoteMeta("UPDATE node_earnings")).WillReturnResult(sqlmock.NewResult(0, 0))
	mockSQL.ExpectExec(regexp.QuoteMeta("UPDATE oracle_batches")).WillReturnResult(sqlmock.NewResult(0, 1))

	models.CheckOracleConsensus(batchDate, payloadHash, 3)

	assert.NoError(t, mockSQL.ExpectationsWereMet())
}
