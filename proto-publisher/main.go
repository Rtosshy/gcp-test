package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	eventv1 "gcp-test/gen/event/v1"
)

func main() {
	projectID := flag.String("project", "gcp-learning-507706", "GCP project ID")
	topicID := flag.String("topic", "user-event-topic", "Pub/Sub topic ID")
	userID := flag.String("user-id", "user-123", "UserEvent.user_id")
	action := flag.String("action", "login", "UserEvent.action")
	device := flag.String("device", "", "UserEvent.device（空なら未設定）")
	encoding := flag.String("encoding", "binary", "メッセージのエンコーディング（binary / json）。トピックの設定に合わせる")
	raw := flag.String("raw", "", "proto を使わずこの文字列をそのまま送る（スキーマ違反の確認用）")
	invalid := flag.Bool("invalid", false, "proto として解釈できないバイト列を送る（スキーマ違反の確認用）")
	flag.Parse()

	data, err := buildData(*userID, *action, *device, *encoding, *raw, *invalid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build message failed: %v\n", err)
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := pubsub.NewClient(ctx, *projectID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init client failed: %v\n", err)
		os.Exit(2)
	}
	defer client.Close()

	publisher := client.Publisher(*topicID)
	defer publisher.Stop()

	serverID, err := publisher.Publish(ctx, &pubsub.Message{Data: data}).Get(ctx)
	if err != nil {
		// スキーマ検証に失敗すると InvalidArgument が返る
		if s, ok := status.FromError(err); ok && s.Code() == codes.InvalidArgument {
			fmt.Printf("REJECTED: %s\n", s.Message())
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "publish failed: %v\n", err)
		os.Exit(2)
	}

	fmt.Printf("ACCEPTED: message_id=%s\n", serverID)
}

func buildData(userID, action, device, encoding, raw string, invalid bool) ([]byte, error) {
	switch {
	case invalid:
		// 終端しない varint なので proto のバイナリとしてパースできない
		return []byte{0xff, 0xff, 0xff}, nil
	case raw != "":
		return []byte(raw), nil
	}

	event := &eventv1.UserEvent{
		UserId:         proto.String(userID),
		Action:         proto.String(action),
		OccurredAtUnix: proto.Int64(time.Now().Unix()),
	}
	if device != "" {
		event.Device = proto.String(device)
	}

	switch encoding {
	case "binary":
		return proto.Marshal(event)
	case "json":
		return protojson.MarshalOptions{UseProtoNames: true}.Marshal(event)
	default:
		return nil, fmt.Errorf("unknown encoding: %s", encoding)
	}
}
