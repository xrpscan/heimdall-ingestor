package store

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// Timestamp represents an epoch timestamp that gets stored in the database in TIMESTAMP format.
type Timestamp int64

func (t *Timestamp) Scan(value any) error {
	switch asserted := value.(type) {
	case time.Time:
		*t = Timestamp(asserted.UnixMilli())
	case int64:
		*t = Timestamp(asserted)
	case uint64:
		*t = Timestamp(asserted)
	default:
		return fmt.Errorf("unknown type: %T", asserted)
	}
	return nil
}

func (t Timestamp) Value() (driver.Value, error) {
	return time.UnixMilli(int64(t)), nil
}

func (t Timestamp) ToTime() time.Time {
	return time.UnixMilli(int64(t))
}
