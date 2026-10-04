package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafka.Reader
}

func NewConsumer(brokerAddr, topic, groupID string) (*Consumer, error) {
	if brokerAddr == "" {
		return nil, fmt.Errorf("kafka broker address must not be empty")
	}
	if topic == "" {
		return nil, fmt.Errorf("kafka topic must not be empty")
	}
	if groupID == "" {
		return nil, fmt.Errorf("kafka consumer groupID must not be empty")
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{brokerAddr},
		Topic:   topic,
		GroupID: groupID,
	})

	return &Consumer{reader: r}, nil
}

func (c *Consumer) ReadMessage(ctx context.Context) (kafka.Message, error) {
	return c.reader.ReadMessage(ctx)
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
