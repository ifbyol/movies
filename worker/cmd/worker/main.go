package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"fmt"

	_ "github.com/lib/pq"

	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/Shopify/sarama"
	"github.com/okteto/movies/pkg/database"
	"github.com/okteto/movies/pkg/kafka"
)

var (
	topic             = kingpin.Flag("topic", "Topic name").Default("rentals").String()
	messageCountStart = kingpin.Flag("messageCountStart", "Message counter start from:").Int()
)

func main() {
	kingpin.Parse()

	db := database.Open()
	defer db.Close()

	database.Ping(db)

	dropTableStmt := `DROP TABLE IF EXISTS rentals`
	if _, err := db.Exec(dropTableStmt); err != nil {
		log.Panic(err)
	}

	createTableStmt := `CREATE TABLE IF NOT EXISTS rentals (id VARCHAR(255) NOT NULL UNIQUE, price VARCHAR(255) NOT NULL)`
	if _, err := db.Exec(createTableStmt); err != nil {
		log.Panic(err)
	}

	// Get a consumer group
	consumerGroup, err := kafka.GetConsumerGroup()
	if err != nil {
		log.Panicf("Error creating consumer group: %v", err)
	}
	defer consumerGroup.Close()

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a wait group to manage the goroutines
	var wg sync.WaitGroup
	wg.Add(1)

	// Handle consumer group errors
	go func() {
		defer wg.Done()
		for err := range consumerGroup.Errors() {
			log.Printf("Consumer group error: %v", err)
		}
	}()

	// Create a channel to signal when the consumer is ready
	ready := make(chan bool)

	// Create a handler for processing messages
	handler := &kafka.ConsumerGroupHandler{
		Ready: ready,
		MessageHandler: func(msg *sarama.ConsumerMessage) {
			*messageCountStart++
			log.Printf("Received message: topic=%s, partition=%d, offset=%d, key=%s, value=%s\n",
				msg.Topic, msg.Partition, msg.Offset, string(msg.Key), string(msg.Value))

			price, err := strconv.ParseFloat(string(msg.Value), 64)
			if err != nil {
				log.Printf("Error parsing price value '%s': %v", string(msg.Value), err)
				return
			}

			insertDynStmt := `insert into "rentals"("id", "price") values($1, $2) on conflict(id) do update set price = $2`
			if _, err := db.Exec(insertDynStmt, string(msg.Key), fmt.Sprintf("%f", price)); err != nil {
				log.Printf("Error inserting into database: %v", err)
			} else {
				log.Printf("Successfully inserted/updated rental: id=%s, price=%f", string(msg.Key), price)
			}
		},
	}

	// Setup signal handling
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	// Start consuming in a goroutine
	topics := []string{*topic}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			// Consume joins a consumer group, consuming from the specified topics
			if err := consumerGroup.Consume(ctx, topics, handler); err != nil {
				log.Printf("Error from consumer group: %v", err)
			}
			// Check if context was cancelled, signaling that the consumer should stop
			if ctx.Err() != nil {
				log.Println("Context cancelled, stopping consumer")
				return
			}
			// Reset the ready channel for the next consume session
			handler.Ready = make(chan bool)
		}
	}()

	// Wait until the consumer is ready
	select {
	case <-ready:
		log.Println("Consumer is ready")
	case <-time.After(10 * time.Second):
		log.Println("Timeout waiting for consumer to be ready, but continuing anyway")
	}

	fmt.Println("Worker started. Press Ctrl+C to exit...")

	// Wait for a signal
	<-signals
	fmt.Println("Interrupt is detected, shutting down...")

	// Cancel the context to stop the consumer group
	cancel()

	// Wait for the consumer group to finish
	wg.Wait()

	log.Println("Processed", *messageCountStart, "messages")
}
