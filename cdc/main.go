package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
)

const (
	connStr        = "postgres://cdc:cdc@localhost:5432/cdcdb?replication=database&sslmode=disable"
	slotName       = "cdc_slot"
	pubName        = "cdc_pub"
	standbyTimeout = 10 * time.Second
)

var relations = map[uint32]*pglogrepl.RelationMessage{}

func main() {
	ctx := context.Background()

	conn, err := pgconn.Connect(ctx, connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close(ctx)

	// Create replication slot (ignore "already exists" error)
	pglogrepl.CreateReplicationSlot(ctx, conn, slotName, "pgoutput",
		pglogrepl.CreateReplicationSlotOptions{Temporary: false})

	// Get current WAL position
	sysIdent, err := pglogrepl.IdentifySystem(ctx, conn)
	if err != nil {
		log.Fatal(err)
	}
	startLSN := sysIdent.XLogPos

	// Start streaming
	err = pglogrepl.StartReplication(ctx, conn, slotName, startLSN,
		pglogrepl.StartReplicationOptions{
			PluginArgs: []string{
				"proto_version '1'",
				fmt.Sprintf("publication_names '%s'", pubName),
			},
		})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("streaming from LSN %s", startLSN)

	clientPos := startLSN
	nextHeartbeat := time.Now().Add(standbyTimeout)

	for {
		if time.Now().After(nextHeartbeat) {
			pglogrepl.SendStandbyStatusUpdate(ctx, conn,
				pglogrepl.StandbyStatusUpdate{WALWritePosition: clientPos})
			nextHeartbeat = time.Now().Add(standbyTimeout)
		}

		receiveCtx, cancel := context.WithDeadline(ctx, nextHeartbeat)
		rawMsg, err := conn.ReceiveMessage(receiveCtx)
		cancel()

		if err != nil {
			if pgconn.Timeout(err) {
				continue
			}
			log.Fatal(err)
		}

		copyData, ok := rawMsg.(*pgproto3.CopyData)
		if !ok {
			continue
		}

		switch copyData.Data[0] {
		case pglogrepl.PrimaryKeepaliveMessageByteID:
			pkm, _ := pglogrepl.ParsePrimaryKeepaliveMessage(copyData.Data[1:])
			if pkm.ReplyRequested {
				nextHeartbeat = time.Time{}
			}

		case pglogrepl.XLogDataByteID:
			xld, _ := pglogrepl.ParseXLogData(copyData.Data[1:])
			handleMessage(xld.WALData)
			clientPos = xld.WALStart + pglogrepl.LSN(len(xld.WALData))
		}
	}
}

func handleMessage(data []byte) {
	logicalMsg, err := pglogrepl.Parse(data)
	if err != nil {
		log.Fatalf("parse logical replication message: %s", err)
	}

	switch msg := logicalMsg.(type) {
	case *pglogrepl.RelationMessage:
		relations[msg.RelationID] = msg
		log.Printf("[RELATION] %s.%s", msg.Namespace, msg.RelationName)

	case *pglogrepl.BeginMessage:
		log.Printf("[BEGIN] xid=%d", msg.Xid)

	case *pglogrepl.CommitMessage:
		log.Printf("[COMMIT]")

	case *pglogrepl.InsertMessage:
		rel := relations[msg.RelationID]
		log.Printf("[INSERT] %s → %v", rel.RelationName, decodeTuple(msg.Tuple, rel))

	case *pglogrepl.UpdateMessage:
		rel := relations[msg.RelationID]
		log.Printf("[UPDATE] %s old=%v → new=%v", rel.RelationName,
			decodeTuple(msg.OldTuple, rel), decodeTuple(msg.NewTuple, rel))

	case *pglogrepl.DeleteMessage:
		rel := relations[msg.RelationID]
		log.Printf("[DELETE] %s → %v", rel.RelationName, decodeTuple(msg.OldTuple, rel))

	default:
		log.Printf("[UNKNOWN] %T", logicalMsg)
	}
}

func decodeTuple(tuple *pglogrepl.TupleData, rel *pglogrepl.RelationMessage) map[string]string {
	values := map[string]string{}
	for i, col := range tuple.Columns {
		colName := rel.Columns[i].Name
		switch col.DataType {
		case 'n': // null
			values[colName] = "NULL"
		case 't': // text
			values[colName] = string(col.Data)
		case 'u': // unchanged TOAST
			values[colName] = "(unchanged)"
		}
	}
	return values
}
