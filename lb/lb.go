package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Backend struct {
	Address string
	Alive bool
}

type LoadBalancer struct {
	backends []*Backend
	current  uint64
}

func NewLoadBalancer(addrs []string) *LoadBalancer {

	var backends []*Backend

	for _, addr := range addrs {
		backends = append(backends, &Backend{Address: addr, Alive: true})
	}

	return &LoadBalancer{backends: backends }
}


func (lb *LoadBalancer) HealthCheck(interval time.Duration){
	client := &http.Client{Timeout: 2* time.Second}

	for {
		for _, b := range lb.backends {
			resp, err := client.Get("http://" + b.Address + "/health")

			if err != nil || resp.StatusCode != http.StatusOK {
				b.Alive = false
				log.Printf("backend server %s is DOWN", b.Address)
			}else {
				b.Alive = true
			}

			if resp != nil {
				resp.Body.Close()
			}
		}
		time.Sleep(interval)
	}
}

// round robin algo to select backend server
func (lb *LoadBalancer) NextBackend() *Backend {
	total := uint64(len(lb.backends))

	for  i := uint64(0); i < total; i++ {
		n := atomic.AddUint64(&lb.current, 1)
		idx := n % total
		if lb.backends[idx].Alive {
			return lb.backends[idx]
		}
	}

	return nil
}

func (lb *LoadBalancer) Start(addr string) error  {
	listener, err := net.Listen("tcp", addr)

	if err != nil{
		return err
	}

	defer listener.Close()

	for {
		clientConn, err := listener.Accept()
		if err != nil{
			continue
		}
		
		go lb.handleConnection(clientConn)

	}

}

func (lb *LoadBalancer) handleConnection(clientConn net.Conn)  {
	defer clientConn.Close()


	backend := lb.NextBackend()

	if backend == nil {
		log.Println("no healthy backend server avilable")
		clientConn.Close()
		return
	}
	log.Printf("forwording to %s", backend.Address)

	backendConn, err := net.Dial("tcp", backend.Address)

	if err != nil{
		log.Printf("failed to connect to backend %s: %v", backend.Address, err)
	}

	defer backendConn.Close()

	// Pipe data both ways concurrently

	var wg sync.WaitGroup
	wg.Add(2)

	// client to backend
	go func() {
		defer wg.Done()
		io.Copy(backendConn, clientConn)
	}()

	// backend to client

	go func() {
		defer wg.Done()
		io.Copy(clientConn, backendConn)
	}()
	wg.Wait()

	log.Printf("connection between %s and %s closed", clientConn.RemoteAddr(), backend.Address)
}
