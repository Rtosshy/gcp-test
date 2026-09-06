package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/pubsub/v2"
)

func main() {
	projectID := "gcp-learning-507706"
	topicID := "my-topic"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("init client failed: %v", err)
	}
	defer client.Close()

	publisher := client.Publisher(topicID)
	defer publisher.Stop()

	msg := &pubsub.Message{
		Data: []byte("Hello go"),
	}

	res := publisher.Publish(ctx, msg)
	serverID, err := res.Get(ctx)
	if err != nil {
		log.Fatalf("publish failed: %v", err)
	}

	fmt.Println(serverID)
}
