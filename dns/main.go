package main

import (
	"fmt"
	"log"

	"github.com/miekg/dns"
)

// CMD - dig @8.8.8.8 delhivery.com

func main() {
	registory.mu.Lock()
	registory.data["server1.service.internal."] = "127.0.0.1" // test-server1 :8080
	registory.data["server2.service.internal."] = "127.0.0.1" // test-server2 :8081
	registory.data["server3.service.internal."] = "127.0.0.1" // test-server3 :8082
	registory.data["lb.service.internal."] = "127.0.0.1"      // load balancer :9090
	registory.mu.Unlock()

	// register handler for all DNS queries
	dns.HandleFunc(".", HandleDNSRequest)

	// start UDP server
	server := &dns.Server{Addr: ":8053", Net: "udp"}
	fmt.Println("DNS server listening on :8053")

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("failed to start DNS server: %v", err)
	}
}
