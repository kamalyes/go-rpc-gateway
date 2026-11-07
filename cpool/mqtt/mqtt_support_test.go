//go:build mq

package mqtt

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/kamalyes/go-rpc-gateway/config/queue"
)

func TestDocumentedDSNs(t *testing.T) {
	for _, dsn := range []string{
		"tcp://127.0.0.1:1883?client_id=my-service",
		"ssl://gateway:mqtt_pwd@mqtt.example.com:8883?client_id=my-service&connect_timeout=10s",
		"ws://gateway:mqtt_pwd@mqtt.example.com:8083/mqtt?client_id=my-service&keep_alive=60s",
		"tcp://127.0.0.1:1883?client_id=my-service&clean_session=false&will_topic=status%2Fmy-service&will_payload=offline&will_qos=1",
	} {
		t.Run(dsn, func(t *testing.T) {
			opts, err := buildOptions(dsn)
			if err != nil {
				t.Fatal(err)
			}
			if opts.ClientID != "my-service" || opts.ProtocolVersion != 4 || !opts.AutoReconnect || opts.KeepAlive != 60 {
				t.Fatalf("documented defaults did not reach Paho options")
			}
			if opts.Servers[0].User != nil || opts.Servers[0].RawQuery != "" {
				t.Fatal("MQTT options or credentials leaked to the transport URL")
			}
			if strings.Contains(dsn, "gateway:") && (opts.Username != "gateway" || opts.Password != "mqtt_pwd") {
				t.Fatal("credentials not applied")
			}
			if strings.Contains(dsn, "ssl:") && (opts.ConnectTimeout != 10*time.Second || opts.Dialer.Timeout != 10*time.Second) {
				t.Fatal("connect_timeout must constrain both dial and handshake")
			}
			if strings.Contains(dsn, "will_topic") {
				if opts.CleanSession || !opts.WillEnabled || opts.WillTopic != "status/my-service" || string(opts.WillPayload) != "offline" || opts.WillQos != 1 {
					t.Fatal("session/will options not applied")
				}
			} else if opts.WillEnabled {
				t.Fatal("an empty will_topic must not enable a will")
			}
		})
	}
}

func TestOptionsAndWebSocketQuery(t *testing.T) {
	opts, err := buildOptions("wss://user%40corp:p%40ss%26word@localhost:8883/mqtt?token=a%2Bb&client_id=stable&protocol_version=3&clean_session=false&auto_reconnect=false&keep_alive=0&max_reconnect_interval=15&ping_timeout=2s&write_timeout=0&connect_timeout=500ms&will_topic=status&will_payload=&will_qos=2&will_retained")
	if err != nil {
		t.Fatal(err)
	}
	if opts.Username != "user@corp" || opts.Password != "p@ss&word" || opts.Servers[0].String() != "wss://localhost:8883/mqtt?token=a%2Bb" {
		t.Fatal("credentials, path, or transport query changed")
	}
	if opts.ProtocolVersion != 3 || opts.CleanSession || opts.AutoReconnect || opts.KeepAlive != 0 || opts.MaxReconnectInterval != 15*time.Second || opts.PingTimeout != 2*time.Second || opts.WriteTimeout != 0 || opts.ConnectTimeout != 500*time.Millisecond {
		t.Fatal("explicit parameters were not applied")
	}
	if !opts.WillEnabled || len(opts.WillPayload) != 0 || opts.WillQos != 2 || !opts.WillRetained {
		t.Fatal("explicit empty will payload or flags lost")
	}
	for _, scheme := range []string{"tcp", "ssl", "tls", "tcps", "ws", "wss"} {
		if _, err := buildOptions(scheme + "://[::1]:1883"); err != nil {
			t.Errorf("supported scheme %s: %v", scheme, err)
		}
	}
}

func TestRejectInvalidDSNs(t *testing.T) {
	for _, dsn := range []string{
		"", "http://localhost:1883", "tcp://localhost", "tcp://:1883", "tcp://localhost:0", "tcp://localhost:65536",
		"tcp://localhost:1883/topic", "tcp://localhost:1883#fragment", "tcp://localhost:1883?typo=1",
		"tcp://user:supersecret@localhost:1883?ping_timeout=%ZZ", "tcp://localhost:1883?keep_alive=1;ping_timeout=2",
		"tcp://localhost:1883?protocol_version=5", "tcp://localhost:1883?protocol_version=oops",
		"tcp://localhost:1883?client_id=&clean_session=false", "tcp://localhost:1883?clean_session=oops",
		"tcp://localhost:1883?auto_reconnect=oops", "tcp://localhost:1883?keep_alive=-1", "tcp://localhost:1883?keep_alive=65536s",
		"tcp://localhost:1883?keep_alive=500ms", "tcp://localhost:1883?ping_timeout=0", "tcp://localhost:1883?connect_timeout=0",
		"tcp://localhost:1883?write_timeout=-1", "tcp://localhost:1883?connect_timeout=oops", "tcp://localhost:1883?max_reconnect_interval=0",
		"tcp://localhost:1883?will_topic=foo%2F%23", "tcp://localhost:1883?will_topic=a&will_qos=3", "tcp://localhost:1883?will_payload=orphan",
		"tcp://localhost:1883?connect_timeout=1&connect_timeout=2", "ws://localhost:8080/mqtt?client_id=a&client_id=b",
	} {
		t.Run(dsn, func(t *testing.T) {
			if _, err := buildOptions(dsn); err == nil {
				t.Fatal("invalid DSN accepted")
			} else if strings.Contains(err.Error(), "supersecret") {
				t.Fatal("credentials leaked into parse error")
			}
		})
	}
}

func TestConnectFailureAndCancellation(t *testing.T) {
	for _, mode := range []string{"rejected", "timeout", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = ln.Close() })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				conn, err := ln.Accept()
				if err != nil {
					done <- err
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				if _, err := packets.ReadPacket(conn); err != nil {
					done <- err
					return
				}
				if mode == "rejected" {
					ack := packets.NewControlPacket(packets.Connack).(*packets.ConnackPacket)
					ack.ReturnCode = packets.ErrRefusedNotAuthorised
					done <- ack.Write(conn)
					return
				}
				if mode == "cancelled" {
					cancel()
				}
				_, err = packets.ReadPacket(conn) // client must eventually close the stalled handshake
				if e, ok := err.(net.Error); ok && e.Timeout() {
					done <- err
					return
				}
				done <- nil
			}()
			client, err := NewClient(ctx, queue.Default().WithEnabled(true).WithDSN("tcp://"+ln.Addr().String()+"?connect_timeout=150ms"), nil)
			if err == nil || client != nil {
				t.Fatal("connection failure returned a client or no error")
			}
			if mode == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("got %v, want context cancellation", err)
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(4 * time.Second):
				t.Fatal("connection attempt leaked after failure")
			}
		})
	}
}
