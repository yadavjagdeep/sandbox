package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "time"

    "github.com/segmentio/kafka-go"

    "view-counter/models"
    "view-counter/rule"
)

const BatchSize = 100

func main() {
    ctx := context.Background()

    reader := kafka.NewReader(kafka.ReaderConfig{
        Brokers:  []string{"localhost:9092"},
        Topic:    "raw-views",
        GroupID:  "validator",
        MinBytes: 1,
        MaxBytes: 10e6,
    })
    defer reader.Close()

    writer := &kafka.Writer{
        Addr:     kafka.TCP("localhost:9092"),
        Topic:    "valid-views",
        Balancer: &kafka.Hash{},
    }
    defer writer.Close()

    fmt.Println("Validator started. Consuming raw-views...")

    var batch []models.ViewEvent
    var keys []string

    for {
        msg, err := reader.ReadMessage(ctx)
        if err != nil {
            log.Printf("read error: %v", err)
            continue
        }

        var event models.ViewEvent
        if err := json.Unmarshal(msg.Value, &event); err != nil {
            continue
        }

        batch = append(batch, event)
        keys = append(keys, string(msg.Key))

        if len(batch) >= BatchSize {
            valid := rule.Validate(batch)

            var messages []kafka.Message
            for _, v := range valid {
                body, _ := json.Marshal(v)
                messages = append(messages, kafka.Message{
                    Key:   []byte(fmt.Sprintf("%s_%d", v.VideoID, time.Now().UnixNano()%10)),
                    Value: body,
                })
            }

            if len(messages) > 0 {
                if err := writer.WriteMessages(ctx, messages...); err != nil {
                    log.Printf("write error: %v", err)
                } else {
                    fmt.Printf("Validated: %d/%d views passed\n", len(valid), len(batch))
                }
            }

            batch = batch[:0]
            keys = keys[:0]
        }
    }
}
