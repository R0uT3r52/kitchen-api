package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokerAddr, topic string) (*Producer, error) {
	if brokerAddr == "" {
		return nil, fmt.Errorf("kafka broker address must not be empty")
	}
	if topic == "" {
		return nil, fmt.Errorf("kafka topic must not be empty")
	}

	w := &kafka.Writer{
		Addr:         kafka.TCP(brokerAddr),
		Topic:        topic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireAll,
		MaxAttempts:  3,
		WriteTimeout: 10 * time.Second, // defaults
		ReadTimeout:  10 * time.Second, // defaults
	}

	return &Producer{writer: w}, nil
}

func (p *Producer) Publish(ctx context.Context, key string, payload []byte) error {
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: payload,
	})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
