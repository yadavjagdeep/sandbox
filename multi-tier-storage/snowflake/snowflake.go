package snowflake

import (
	"sync"
	"time"
)

const (
	epoch          = 1700000000000 // Nov 2023 in ms
	machineBites   = 10
	sequenceBits   = 12
	machineShift   = sequenceBits
	timestampShift = sequenceBits + machineBites
	sequenceMask   = (1 << sequenceBits) - 1
)

type Generator struct {
	mu        sync.Mutex
	machineID int64
	sequence  int64
	lastTime  int64
}

func New(machineID int64) *Generator {
	return &Generator{machineID: machineID}
}

func (g *Generator) Generate() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now().UnixMilli()

	if now == g.lastTime {
		g.sequence = (g.sequence + 1) & sequenceMask
		if g.sequence == 0 {
			for now <= g.lastTime {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		g.sequence = 0
	}

	g.lastTime = now
	return (now-epoch)<<timestampShift | g.machineID<<machineShift | g.sequence
}

// returns the creation time from snowflake ID
func ExtractTime(id int64) time.Time {
	ms := (id >> timestampShift) + epoch
	return time.UnixMilli(ms)
}

// returns how old the ID is
func Age(id int64) time.Duration {
	created := ExtractTime(id)
	return time.Since(created)
}

// GenerateWithTime creates a snowflake ID with a specific timestamp (for testing)
func GenerateWithTime(machineID int64, t time.Time) int64 {
	ms := t.UnixMilli()
	return (ms-epoch)<<timestampShift | machineID<<machineShift | 0
}
