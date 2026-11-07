//go:build !mq

package mqtt

import (
	"context"
	"strings"
	"testing"

	"github.com/kamalyes/go-rpc-gateway/config/queue"
)

func TestEnabledWithoutBuildTag(t *testing.T) {
	client, err := NewClient(context.Background(), queue.Default().WithEnabled(true), nil)
	if client != nil || err == nil || !strings.Contains(err.Error(), "-tags mq") {
		t.Fatalf("client=%v, error=%v", client, err)
	}
}
