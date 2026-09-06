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
	subID := "my-sub"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("init client failed: %v", err)
	}
	defer client.Close()

	subscriber := client.Subscriber(subID)
	err = subscriber.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		fmt.Println(string(msg.Data))
		msg.Ack()
	})
	if err != nil {
		log.Fatalf("subscribe failed: %v", err)
	}
}
