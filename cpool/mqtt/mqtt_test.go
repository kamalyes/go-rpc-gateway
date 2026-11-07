package mqtt

import (
	"context"
	"testing"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/config/queue"
)

func TestDisabledClient(t *testing.T) {
	for _, cfg := range []*queue.Mqtt{nil, {}, queue.Default()} {
		client, err := NewClient(context.Background(), cfg, nil)
		if client != nil || err != nil {
			t.Fatalf("disabled config: client=%v err=%v", client, err)
		}
		if DefaultMqtt(context.Background(), &gwconfig.Gateway{Mqtt: cfg}, nil) != nil {
			t.Fatal("legacy wrapper returned pointer to a nil client")
		}
	}
}
