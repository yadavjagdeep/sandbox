package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

type Cluster struct {
	selfID string
	ring   *Ring
	peers  []string
	missed map[string]int
}

func NewCluster(selfID string, ring *Ring) *Cluster {

	c := &Cluster{
		selfID: selfID,
		ring:   ring,
		missed: make(map[string]int),
		peers:  []string{},
	}
	ring.AddNode(selfID) // seeding the self node
	return c
}

// Join connects to seed, sends JOIN, receives NODES list

func (c *Cluster) Join(seedAddr string) error {
	conn, err := net.Dial("tcp", seedAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to seed: %w", err)
	}
	defer conn.Close()

	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(conn)

	// send JOIN
	fmt.Fprintf(writer, "JOIN %s\r\n", c.selfID)
	writer.Flush()

	// read NODES response
	line, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read NODES: %w", err)
	}
	line = strings.TrimSpace(line)

	// parse: "NODES localhost:7071,localhost:7072,localhost:7073"
	parts := strings.SplitN(line, " ", 2)
	if len(parts) != 2 || parts[0] != "NODES" {
		return fmt.Errorf("unexpected NODES response: %s", line)
	}

	nodes := strings.Split(parts[1], ",")
	for _, node := range nodes {
		if node != c.selfID {
			c.ring.AddNode(node)
			c.peers = append(c.peers, node)
		}
	}
	return nil
}

// HandleJoin is called when another node sends JOIN to us

func (c *Cluster) HandleJoin(nodeID string) string {
	c.ring.AddNode(nodeID)
	c.peers = append(c.peers, nodeID)

	// build nodes reponse all nodes including the new one
	allNodes := append(c.peers, c.selfID)
	response := "NODES " + strings.Join(allNodes, ",") + "\r\n"

	// notify existing peers about the new node
	c.NotifyPeers(fmt.Sprintf("JOINED %s\r\n", nodeID))
	return response
}

func (c *Cluster) HandleJoined(nodeID string) {
	for _, p := range c.peers {
		if p == nodeID {
			return
		}
	}
	c.ring.AddNode(nodeID)
	c.peers = append(c.peers, nodeID)
}

// HandleLeft is called when a node is declared dead
func (c *Cluster) HandleLeft(nodeID string) {
	c.ring.RemoveNode(nodeID)
	newPeers := []string{}
	for _, p := range c.peers {
		if p != nodeID {
			newPeers = append(newPeers, p)
		}
	}
	c.peers = newPeers
	delete(c.missed, nodeID)
}

// NotifyPeers sends a message to all known peers
func (c *Cluster) NotifyPeers(msg string) {
	for _, peer := range c.peers {
		go func(addr string) {
			conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
			if err != nil {
				return
			}
			defer conn.Close()
			writer := bufio.NewWriter(conn)
			writer.WriteString(msg)
			writer.Flush()
		}(peer)
	}
}

// StartHeartbeat pings all peers every 2 seconds
func (c *Cluster) StartHeartbeat() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			for _, peer := range c.peers {
				alive := c.ping(peer)
				if !alive {
					c.missed[peer]++
					if c.missed[peer] >= 3 {
						fmt.Printf("node %s declared dead\n", peer)
						c.HandleLeft(peer)
						c.NotifyPeers(fmt.Sprintf("LEFT %s\r\n", peer))
					}
				} else {
					c.missed[peer] = 0
				}
			}
		}

	}()
}

func (c *Cluster) ping(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()

	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(conn)

	fmt.Fprintf(writer, "PING\r\n")
	writer.Flush()

	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	return strings.TrimSpace(line) == "PONG"
}
