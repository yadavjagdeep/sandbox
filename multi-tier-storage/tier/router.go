package tier

import (
	"multi-tier-storage/snowflake"
	"time"
)

const (
	HotThreshold  = 6 * 30 * 24 * time.Hour
	WarmThreshold = 2 * 365 * 24 * time.Hour
)

type Tier int

const (
	TierHot  Tier = iota // SQL
	TierWarm             // DynamoDB
	TierCold             // S3
)

func Route(orderID int64) Tier {
	age := snowflake.Age(orderID)

	switch {
	case age < HotThreshold:
		return TierHot
	case age < WarmThreshold:
		return TierWarm
	default:
		return TierCold
	}
}

func (t Tier) String()string  {
	switch t {
	case TierHot:
		return "hot (SQL)"
	case TierWarm:
		return "warm (DynamoDB)"
	case  TierCold:
		return "cold (S3)"
	default:
		return "unknown"

	}
}