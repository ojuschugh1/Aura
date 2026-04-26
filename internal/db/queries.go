package db

import (
	"database/sql"
	"time"
)

// ISO 8601 format used for all SQLite TEXT timestamps.
const timeFormat = "2006-01-02T15:04:05.000Z"

// TimeNow returns the current UTC time formatted as an ISO 8601 string.
func TimeNow() string {
	return time.Now().UTC().Format(timeFormat)
}

// ScanNullableTime scans a nullable TEXT timestamp column into a *time.Time.
func ScanNullableTime(col sql.NullString) (*time.Time, error) {
	if !col.Valid || col.String == "" {
		return nil, nil
	}
	t, err := time.Parse(timeFormat, col.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
