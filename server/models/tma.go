package models

import (
	"exra/db"
	"fmt"
	"github.com/google/uuid"
)

// UUID generates a new version 4 UUID string.
func UUID() string {
	return uuid.New().String()
}

// CompleteTmaLink marks a pending link request as 'linked' after physical device approval.
func CompleteTmaLink(requestID, deviceID string) error {
	res, err := db.DB.Exec(`
		UPDATE tma_devices 
		SET status = 'linked' 
		WHERE device_id = $1 AND request_id = $2 AND status = 'pending'`,
		deviceID, requestID,
	)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no pending link request found for device %s and request %s", deviceID, requestID)
	}
	return nil
}

// RejectTmaLink removes a pending link request if the user denied it on the device.
func RejectTmaLink(requestID, deviceID string) error {
	_, err := db.DB.Exec(`
		DELETE FROM tma_devices 
		WHERE device_id = $1 AND request_id = $2 AND status = 'pending'`,
		deviceID, requestID,
	)
	return err
}
