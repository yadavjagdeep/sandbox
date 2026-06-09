package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func PareseCommand(reader *bufio.Reader) (Command, error) {
	line, err := reader.ReadString('\n')

	if err != nil {
		return Command{}, err
	}
	line = strings.TrimSpace(line)

	parts := strings.SplitN(line, " ", 4)
	if len(parts) == 0 {
		return Command{}, fmt.Errorf("empty command")
	}

	op := strings.ToUpper(parts[0])

	switch op {
	case "GET":
		if len(parts) < 2 {
			return Command{}, fmt.Errorf("GET requires a key")
		}
		return Command{Op: "GET", Key: parts[1]}, nil

	case "DEL":

		if len(parts) < 2 {
			return Command{}, fmt.Errorf("DEL requires a key")
		}
		return Command{Op: "DEL", Key: parts[1]}, nil

	case "PUT":
		if len(parts) < 3 {
			return Command{}, fmt.Errorf("PUT requires a key and value")
		}
		ttl, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return Command{}, fmt.Errorf("invalid TTL: %v", parts[2])
		}
		vlen, err := strconv.Atoi(parts[3])
		if err != nil {
			return Command{}, fmt.Errorf("invalid value length: %s", parts[3])
		}

		// read exactly vlen bytes for the value
		value := make([]byte, vlen)

		_, err = reader.Read(value)

		if err != nil {
			return Command{}, fmt.Errorf("failed to read value: %v", err)
		}
		// consume trailing \r\n
		reader.ReadString('\n')
		return Command{Op: "PUT", Key: parts[1], Value: value, TTL: ttl}, nil

	case "JOIN", "JOINED", "LEFT":
		if len(parts) < 2 {
			return Command{}, fmt.Errorf("%s requires a node ID", op)
		}
		return Command{Op: op, Key: parts[1]}, nil

	case "PING":
		return Command{Op: "PING"}, nil

	case "PONG":
		return Command{Op: "PONG"}, nil

	default:
		return Command{}, fmt.Errorf("unknown command: %s", op)
	}
}

func ParseResponse(reader *bufio.Reader) Result {
	line, err := reader.ReadString('\n')
	if err != nil {
		return Result{Err: fmt.Errorf("failed to read response: %w", err)}
	}
	line = strings.TrimSpace(line)

	switch {
	case strings.HasPrefix(line, "+OK"):
		return Result{Found: true, Value: nil}

	case strings.HasPrefix(line, "$-1"):
		return Result{Found: false}

	case strings.HasPrefix(line, "$"):
		lenStr := line[1:]
		vlen, err := strconv.Atoi(lenStr)
		if err != nil {
			return Result{Err: fmt.Errorf("invalid length: %s", lenStr)}
		}
		value := make([]byte, vlen)
		_, err = io.ReadFull(reader, value)
		if err != nil {
			return Result{Err: fmt.Errorf("failed to read value: %w", err)}
		}
		reader.ReadString('\n') // consume trailing \r\n
		return Result{Found: true, Value: value}

	case strings.HasPrefix(line, "-ERR"):
		return Result{Err: fmt.Errorf("%s", line[5:])}

	default:
		return Result{Err: fmt.Errorf("unknown response: %s", line)}
	}
}

func WriteResponse(writer *bufio.Writer, result Result) error {
	if result.Err != nil {
		fmt.Fprintf(writer, "-ERR %s\r\n", result.Err.Error())
	} else if !result.Found {
		writer.WriteString("$-1\r\n")
	} else if result.Value == nil {
		// PUT/DEL success
		writer.WriteString("+OK\r\n")
	} else {
		// GET hit
		writer.WriteString(fmt.Sprintf("$%d\r\n", len(result.Value)))
		writer.Write(result.Value)
		writer.WriteString("\r\n")
	}
	return writer.Flush()
}
