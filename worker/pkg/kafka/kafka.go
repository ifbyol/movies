package kafka

import (
	"fmt"
	"log"
	"time"

	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/Shopify/sarama"
)

var (
	brokerList = kingpin.Flag("brokerList", "List of brokers to connect").Default("kafka:9092").Strings()
	groupID    = kingpin.Flag("groupID", "Consumer group ID").Default("worker-group").String()
)

// GetMaster returns a legacy consumer (kept for backward compatibility)
func GetMaster() sarama.Consumer {
	kingpin.Parse()
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	brokers := *brokerList
	fmt.Println("Waiting for kafka...")
	for {
		master, err := sarama.NewConsumer(brokers, config)
		if err == nil {
			fmt.Println("Kafka connected!")
			return master
		}
		time.Sleep(1 * time.Second)
	}
}

// GetConsumerGroup returns a consumer group
func GetConsumerGroup() (sarama.ConsumerGroup, error) {
	kingpin.Parse()
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Version = sarama.V2_0_0_0 // Kafka 2.0.0 is required for consumer groups

	// Configure for single broker environment
	config.Consumer.Offsets.AutoCommit.Enable = true
	config.Consumer.Offsets.AutoCommit.Interval = 1 * time.Second

	// These configurations are critical for single-broker environments
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRange
	config.Consumer.Offsets.Retention = 1 * time.Hour

	// Set to 1 for single broker environments (default is 3)
	config.Consumer.Offsets.Retry.Max = 5

	brokers := *brokerList
	groupID := *groupID

	fmt.Printf("Waiting for kafka consumer group with groupID: %s...\n", groupID)

	// Retry connection to Kafka
	var consumerGroup sarama.ConsumerGroup
	var err error
	for i := 0; i < 10; i++ {
		consumerGroup, err = sarama.NewConsumerGroup(brokers, groupID, config)
		if err == nil {
			break
		}
		log.Printf("Failed to connect to Kafka consumer group: %v. Retrying in 1 second...", err)
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to Kafka after multiple attempts: %v", err)
	}

	fmt.Println("Kafka consumer group connected!")
	return consumerGroup, nil
}

// ConsumerGroupHandler represents a Sarama consumer group handler
type ConsumerGroupHandler struct {
	Ready          chan bool
	MessageHandler func(message *sarama.ConsumerMessage)
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (h *ConsumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	// Mark the consumer as ready
	close(h.Ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (h *ConsumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (h *ConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	// NOTE: Do not move the code below to a goroutine.
	// The `ConsumeClaim` itself is called within a goroutine, see:
	// https://github.com/Shopify/sarama/blob/main/consumer_group.go#L27-L29
	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				log.Println("Message channel was closed")
				return nil
			}
			if h.MessageHandler != nil {
				h.MessageHandler(message)
			}
			// Mark the message as processed
			session.MarkMessage(message, "")
			log.Printf("Message claimed: value = %s, timestamp = %v, topic = %s, partition = %d, offset = %d",
				string(message.Value), message.Timestamp, message.Topic, message.Partition, message.Offset)
		case <-session.Context().Done():
			return nil
		}
	}
}
