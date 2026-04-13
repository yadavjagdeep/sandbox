package central

import (
	"sync"
)

// Server-side: hands out ID ranges
type IDService struct {
	mu sync.Mutex
	counter int64
}

func NewIDService() *IDService  {
	return &IDService{}
}

func (s *IDService) AllocateBatch(size int64) (start int64, end int64)  {
	s.mu.Lock()
	defer s.mu.Unlock()

	start = s.counter + 1
	s.counter += size
	end = s.counter
	return
}

// Client-side: uses a batch locally
type BatchClient struct {
	mu sync.Mutex
	current int64
	end int64
	service *IDService
	batch int64
}

func NewBatchClient(service *IDService, batchSize int64) *BatchClient  {
	return &BatchClient{service: service, batch: batchSize}
}

func (c *BatchClient) NextID() int64  {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.current >= c.end {
		// Batch exhasted - get a new one
		c.current, c.end = c.service.AllocateBatch(c.batch)
	}

	id := c.current
	c.current++
	return id
}