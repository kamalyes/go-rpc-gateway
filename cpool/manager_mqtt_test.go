//go:build mq

package cpool

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/eclipse/paho.mqtt.golang/packets"
	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/config/queue"
	"github.com/kamalyes/go-toolbox/pkg/logger"
)

// Exercise the real Paho wire protocol without requiring an external broker.
func TestManagerMQTTLifecycle(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	connectPacket := make(chan *packets.ConnectPacket, 1)
	done := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		packet, err := packets.ReadPacket(conn)
		if err != nil {
			done <- err
			return
		}
		cp, ok := packet.(*packets.ConnectPacket)
		if !ok {
			done <- fmt.Errorf("expected CONNECT, got %T", packet)
			return
		}
		connectPacket <- cp
		if err := packets.NewControlPacket(packets.Connack).Write(conn); err != nil {
			done <- err
			return
		}
		for {
			packet, err := packets.ReadPacket(conn)
			if err != nil {
				done <- err
				return
			}
			switch p := packet.(type) {
			case *packets.SubscribePacket:
				ack := packets.NewControlPacket(packets.Suback).(*packets.SubackPacket)
				ack.MessageID = p.MessageID
				ack.ReturnCodes = p.Qoss
				err = ack.Write(conn)
			case *packets.PublishPacket:
				ack := packets.NewControlPacket(packets.Puback).(*packets.PubackPacket)
				ack.MessageID = p.MessageID
				if err = ack.Write(conn); err == nil {
					out := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
					out.TopicName, out.Payload = p.TopicName, p.Payload
					err = out.Write(conn)
				}
			case *packets.PingreqPacket:
				err = packets.NewControlPacket(packets.Pingresp).Write(conn)
			case *packets.DisconnectPacket:
				done <- nil
				return
			default:
				err = fmt.Errorf("unexpected packet %T", packet)
			}
			if err != nil {
				done <- err
				return
			}
		}
	}()
	m := NewManager(logger.NewEmptyLogger())
	t.Cleanup(func() { _ = m.Close() })
	cfg := &gwconfig.Gateway{Mqtt: queue.Default().WithEnabled(true).WithDSN("tcp://user:p%40ss@" + ln.Addr().String() + "?client_id=test-gateway&keep_alive=10&connect_timeout=1s")}
	if err := m.Initialize(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	client := m.GetMQTT()
	if client == nil || !m.HealthCheck()["mqtt"] {
		t.Fatal("Manager did not expose a connected MQTT client")
	}
	cp := <-connectPacket
	if cp.ClientIdentifier != "test-gateway" || cp.Username != "user" || string(cp.Password) != "p@ss" || cp.ProtocolVersion != 4 || cp.Keepalive != 10 || cp.WillFlag {
		t.Fatal("DSN was not applied to the actual CONNECT packet")
	}
	received := make(chan string, 1)
	waitMQTTToken(t, client.Subscribe("test/topic", 1, func(_ paho.Client, msg paho.Message) { received <- string(msg.Payload()) }))
	waitMQTTToken(t, client.Publish("test/topic", 1, false, "hello"))
	select {
	case got := <-received:
		if got != "hello" {
			t.Fatalf("received %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("MQTT subscriber received no message")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if m.GetMQTT() != nil {
		t.Fatal("Manager retained MQTT after Close")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Manager.Close did not disconnect MQTT")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
}

func waitMQTTToken(t *testing.T, token paho.Token) {
	t.Helper()
	if !token.WaitTimeout(2 * time.Second) {
		t.Fatal("MQTT operation timed out")
	}
	if err := token.Error(); err != nil {
		t.Fatal(err)
	}
}

// Paho can report disconnected while a reconnect loop still owns resources.
func TestCloseStopsDisconnectedMQTT(t *testing.T) {
	m := NewManager(logger.NewEmptyLogger())
	client := &disconnectedMQTT{closed: make(chan struct{})}
	m.SetMQTT(client)
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-client.closed:
	default:
		t.Fatal("Disconnect was skipped for a client between reconnect attempts")
	}
}

type disconnectedMQTT struct {
	paho.Client
	closed chan struct{}
}

func (c *disconnectedMQTT) IsConnected() bool { return false }
func (c *disconnectedMQTT) Disconnect(uint)   { close(c.closed) }
