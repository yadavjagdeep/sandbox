package snowflake

import (
	"sync"
	"time"
)


const (
	epoc = 1700000000000 // custom epoc (Nov 2023 in ms) - keeps IDs smaller
	machineBits = 10  // supports 1024 machines
	sequenceBits = 12 // 4096 IDs per milliseconds per machine
	machineShift = sequenceBits
	timestampShift = machineBits + sequenceBits
	sequenceMask = (1 << sequenceBits) - 1 // 4095
)

type Genrator struct {
	mu sync.Mutex
	machineID int64
	sequence int64
	lastTime int64
}

func New(machineID int64) *Genrator {
	return &Genrator{machineID: machineID}

}

func (g *Genrator) Genrate() int64  {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	now := time.Now().UnixMilli()

	if now == g.lastTime {
		g.sequence  = (g.sequence + 1) & sequenceMask
		if g.sequence == 0 {
			// Sequence exceeded, wait for next millisecond
			for now <= g.lastTime {
				now = time.Now().UnixMilli()
			}
		}
	}else {
		g.sequence = 0
	}

	g.lastTime = now

	id := (now-epoc) << timestampShift | g.machineID << machineShift | g.sequence
	return id
}

func Parse(id int64) (timestamp int64, machineID int64, sequence int64)  {
	sequence = id & sequenceMask
	machineID = (id >> machineShift) & ((1 << machineBits) -1)
	timestamp = (id >> timestampShift) + epoc
	return
}