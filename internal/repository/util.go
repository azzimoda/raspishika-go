package repository

import (
	"fmt"
	"time"
)

func sqlPeriod(dur time.Duration) string {
	sqlPeriod := fmt.Sprintf("-%d seconds", int(dur.Seconds()))
	return sqlPeriod
}
