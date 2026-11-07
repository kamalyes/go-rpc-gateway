package cpool

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/kamalyes/go-rpc-gateway/config/cache"
	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/config/queue"
	"github.com/kamalyes/go-toolbox/pkg/logger"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestManagerMQTTDisabled(t *testing.T) {
	m := NewManager(logger.NewEmptyLogger())
	t.Cleanup(func() { _ = m.Close() })
	if err := m.Initialize(context.Background(), &gwconfig.Gateway{}); err != nil {
		t.Fatal(err)
	}
	if m.GetMQTT() != nil {
		t.Fatal("MQTT initialized without config")
	}
	if _, ok := m.HealthCheck()["mqtt"]; ok {
		t.Fatal("disabled MQTT reported as connected")
	}
	if err := m.Initialize(context.Background(), &gwconfig.Gateway{}); err == nil || !m.initialized {
		t.Fatal("duplicate initialization destroyed existing manager")
	}
}

func TestMQTTFailureClosesPartialInitialization(t *testing.T) {
	server := miniredis.RunT(t)
	m := NewManager(logger.NewEmptyLogger())
	t.Cleanup(func() { _ = m.Close() })
	cfg := &gwconfig.Gateway{
		Cache: &cache.Cache{Enabled: true, Redis: cache.Redis{URL: "redis://" + server.Addr()}},
		Mqtt:  queue.Default().WithEnabled(true).WithDSN("invalid"),
	}
	if err := m.Initialize(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "mqtt") {
		t.Fatalf("expected MQTT startup error, got %v", err)
	}
	if m.GetRedis() != nil || m.GetMQTT() != nil || m.initialized {
		t.Fatal("partial initialization was retained")
	}
	require.Eventually(t, func() bool { return server.CurrentConnectionCount() == 0 }, time.Second, time.Millisecond,
		"partial initialization leaked Redis connections")
	cfg.Mqtt.Enabled = false
	if err := m.Initialize(context.Background(), cfg); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if m.GetRedis() == nil {
		t.Fatal("retry did not initialize Redis")
	}
}

func TestCloseBeforeInitializeReleasesInjectedClient(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	m := NewManager(logger.NewEmptyLogger())
	m.SetRedis(client)
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if err := client.Ping(context.Background()).Err(); err != redis.ErrClosed {
		t.Fatalf("injected client not closed: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
}
