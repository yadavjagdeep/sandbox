package main

import (
	"fmt"
	"sync"

	"github.com/miekg/dns"
)


type DNSRegistry struct {
	mu sync.RWMutex
	data map[string]string // domain <> IP
}

var registory = DNSRegistry{
	data: make(map[string]string),
} 

// DNS Handler

func HandleDNSRequest(w dns.ResponseWriter, r *dns.Msg)  {
	msg := dns.Msg{}
	msg.SetReply(r)

	for _, q := range r.Question {
		if q.Qtype == dns.TypeA {

			registory.mu.RLock()
			ip, exists := registory.data[q.Name]
			registory.mu.RUnlock()

			if exists {
				rr, err := dns.NewRR(fmt.Sprintf("%s A %s", q.Name, ip))

				if err == nil {
					msg.Answer = append(msg.Answer, rr)
				}
			}
		}	
	}

	if len(msg.Answer) == 0 {
		msg.Rcode = dns.RcodeNameError
		fmt.Println("Not found:", r.Question[0].Name)
	}

	w.WriteMsg(&msg)
}

