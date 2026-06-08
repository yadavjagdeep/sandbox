package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

type Server struct {
	port    string
	cache   *Cache
	ring    *Ring
	cluster *Cluster
	selfID  string
}

func NewServer(port string, cache *Cache, ring *Ring, cluster *Cluster, selfID string) *Server {
	return &Server{
		port:    port,
		cache:   cache,
		ring:    ring,
		cluster: cluster,
		selfID:  selfID,
	}
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", ":"+s.port)
	if err != nil {
		return err
	}
	fmt.Printf("mem-cache listening on :%s\n", s.port)
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("accept error: %v\n", err)
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		cmd, err := PareseCommand(reader)
		if err != nil {
			// EOF = client disconnected
			if err.Error() == "EOF" || strings.Contains(err.Error(), "EOF") {
				return
			}
			// send error and continue
			WriteResponse(writer, Result{Err: err})
			continue
		}

		switch cmd.Op {
		// --- Client commands: route via ring ---
		case "GET", "PUT", "DEL":
			targetNode := s.ring.GetNode(cmd.Key)

			if targetNode == s.selfID {
				cmd.Response = make(chan Result, 1)
				s.cache.commandChan <- cmd
				result := <-cmd.Response
				if err := WriteResponse(writer, result); err != nil {
					return
				}
			} else {
				// Another node owns it - forward
				result := s.forwardToNode(targetNode, cmd)
				if err := WriteResponse(writer, result); err != nil {
					return
				}
			}

		// --- Cluster commands ---
		case "JOIN":
			response := s.cluster.HandleJoin(cmd.Key)
			writer.WriteString(response)
			writer.Flush()

		case "JOINED":
			s.cluster.HandleJoined(cmd.Key)

		case "LEFT":
			s.cluster.HandleLeft(cmd.Key)

		case "PING":
			writer.WriteString("PONG\r\n")
			writer.Flush()

		default:
			WriteResponse(writer, Result{Err: fmt.Errorf("unkown command: %s", cmd.Op)})
		}

		// // create response channel, send to cache
		// cmd.Response = make(chan Result, 1)
		// s.cache.commandChan <- cmd

		// // wait for result
		// result := <-cmd.Response

		// // write back to client
		// if err := WriteResponse(writer, result); err != nil {
		// 	return // write failed, client disconnedted
		// }
	}
}

func (s *Server) forwardToNode(nodeID string, cmd Command) Result {
	conn, err := net.Dial("tcp", nodeID)
	if err != nil {
		return Result{Err: fmt.Errorf("node unreachable: %s", nodeID)}
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// send command using same protcol
	switch cmd.Op {
	case "GET":
		fmt.Fprintf(writer, "GET %s\r\n", cmd.Key)
	case "DEL":
		fmt.Fprintf(writer, "DEL %s\r\n", cmd.Key)
	case "PUT":
		fmt.Fprintf(writer, "PUT %s %d %d\r\n", cmd.Key, cmd.TTL, len(cmd.Value))
		writer.Write(cmd.Value)
		writer.WriteString("\r\n")
	}

	writer.Flush()

	return ParseResponse(reader)
}
