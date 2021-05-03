package id

import (
	"github.com/influxdata/influxdb/v2/chronograf"
	uuid "github.com/satori/go.uuid"
)

var _ chronograf.ID = &UUID{}

// UUID generates a V4 uuid
type UUID struct{}

// Generate creates a UUID v4 string
func (i *UUID) Generate() (string, error) {
	uuid, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	return uuid.String(), nil
}
