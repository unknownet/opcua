// +build integration

package uatest

import (
	"context"
	"testing"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/monitor"
	"github.com/gopcua/opcua/ua"
)

const (
	currentTimeNodeID   = "ns=0;i=2258"
	reconnectionTimeout = 10 * time.Second
)

// TestSecureChannelReconnection performs an integration test the secure channel reconnection
// from an OPC/UA server.
func TestSecureChannelReconnection(t *testing.T) {

	srv := NewServer("reconnection_server.py")
	defer srv.Close()

	c := opcua.NewClient(srv.Endpoint, srv.Opts...)
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	m, err := monitor.NewNodeMonitor(c)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tests := []struct {
		name string
		req  *ua.CallMethodRequest
	}{
		{
			name: "connection_failure",
			req: &ua.CallMethodRequest{
				ObjectID:       ua.NewStringNodeID(2, "simulations"),
				MethodID:       ua.NewStringNodeID(2, "simulate_connection_failure"),
				InputArguments: []*ua.Variant{},
			},
		},
		{
			name: "securechannel_failure",
			req: &ua.CallMethodRequest{
				ObjectID:       ua.NewStringNodeID(2, "simulations"),
				MethodID:       ua.NewStringNodeID(2, "simulate_securechannel_failure"),
				InputArguments: []*ua.Variant{},
			},
		},
		{
			name: "session_failure",
			req: &ua.CallMethodRequest{
				ObjectID:       ua.NewStringNodeID(2, "simulations"),
				MethodID:       ua.NewStringNodeID(2, "simulate_session_failure"),
				InputArguments: []*ua.Variant{},
			},
		},
		{
			name: "subscription_failure",
			req: &ua.CallMethodRequest{
				ObjectID:       ua.NewStringNodeID(2, "simulations"),
				MethodID:       ua.NewStringNodeID(2, "simulate_subscription_failure"),
				InputArguments: []*ua.Variant{},
			},
		},
	}

	t.Run("reconnection", func(t *testing.T) {
		ch := make(chan *monitor.DataChangeMessage, 5)
		sctx, cancel := context.WithCancel(ctx)
		defer cancel()

		sub, err := m.ChanSubscribe(
			sctx,
			&opcua.SubscriptionParameters{Interval: opcua.DefaultSubscriptionInterval},
			ch,
			currentTimeNodeID,
		)
		if err != nil {
			t.Fatal(err)
		}
		defer sub.Unsubscribe()

		for _, tt := range tests {
			ok := t.Run(tt.name, func(t *testing.T) {

				if msg := <-ch; msg.Error != nil {
					t.Fatalf("No error expected for first value: %s", msg.Error)
				}

				_, err := c.Call(tt.req)
				if err != nil {
					t.Fatal(err)
				}

				// make sure the connection is down
				for c.State() == opcua.Open {
					time.Sleep(10 * time.Millisecond)
				}

				timeout := time.NewTimer(reconnectionTimeout)

				select {
				case <-timeout.C:
					t.Fatal("Reconnection failed, timeout reached!")
				case msg := <-ch:
					if err := msg.Error; err != nil {
						t.Fatal(err)
					}
				}
			})

			if !ok {
				t.FailNow()
			}
		}
	})
}
