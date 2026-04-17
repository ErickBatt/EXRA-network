package models

import (
	"errors"
	"exra/db"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkPayoutPaid_RequiresTxHash(t *testing.T) {
	_, err := MarkPayoutPaid("p1", "", "stripe", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tx_hash")
}

func TestMarkPayoutPaid_EmptyID(t *testing.T) {
	_, err := MarkPayoutPaid("", "0xabc", "", "")
	require.ErrorIs(t, err, ErrPayoutNotFound)
}

func TestMarkPayoutPaid_NotFound(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	mock.ExpectExec(`UPDATE payout_requests`).
		WithArgs("0xabc", nil, nil, "missing").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT status FROM payout_requests WHERE id = \$1`).
		WithArgs("missing").
		WillReturnError(errors.New("sql: no rows in result set"))

	_, err = MarkPayoutPaid("missing", "0xabc", "", "")
	require.ErrorIs(t, err, ErrPayoutNotFound)
}

func TestMarkPayoutPaid_NotApproved(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	mock.ExpectExec(`UPDATE payout_requests`).
		WithArgs("0xabc", nil, nil, "p-pending").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT status FROM payout_requests WHERE id = \$1`).
		WithArgs("p-pending").
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("pending"))

	_, err = MarkPayoutPaid("p-pending", "0xabc", "", "")
	require.ErrorIs(t, err, ErrPayoutNotApproved)
}
