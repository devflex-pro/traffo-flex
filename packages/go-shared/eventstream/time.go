package eventstream

import "time"

const clickHouseDateTimeLayout = "2006-01-02 15:04:05"

func ClickHouseDateTime(value time.Time) string {
	return value.UTC().Format(clickHouseDateTimeLayout)
}
