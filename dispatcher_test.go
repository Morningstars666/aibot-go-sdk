package aibot

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDispatchMessageEvents(t *testing.T) {
	c := NewWSClient(testOptions("ws://127.0.0.1:1"))
	defer c.Close()
	got := make(chan string, 8)
	c.OnMessage(func(f *Frame) { got <- "message" })
	c.OnMessageText(func(f *Frame) { got <- "message.text" })
	raw, err := json.Marshal(msgCallbackFrame("R1"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	c.handleRaw(raw)
	deadline := time.After(3 * time.Second)
	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case name := <-got:
			seen[name] = true
		case <-deadline:
			t.Fatalf("expected both handlers, got %v", seen)
		}
	}
}

func TestDispatchEventEvents(t *testing.T) {
	c := NewWSClient(testOptions("ws://127.0.0.1:1"))
	defer c.Close()
	got := make(chan string, 8)
	c.OnEvent(func(f *Frame) { got <- "event" })
	c.OnEventEnterChat(func(f *Frame) { got <- "event.enter_chat" })
	raw, err := json.Marshal(eventCallbackFrame("R2", EventTypeEnterChat))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	c.handleRaw(raw)
	deadline := time.After(3 * time.Second)
	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case name := <-got:
			seen[name] = true
		case <-deadline:
			t.Fatalf("expected both handlers, got %v", seen)
		}
	}
}

func TestDispatchUnknownMsgTypeOnlyBase(t *testing.T) {
	c := NewWSClient(testOptions("ws://127.0.0.1:1"))
	defer c.Close()
	got := make(chan string, 8)
	c.OnMessage(func(f *Frame) { got <- "message" })
	c.OnMessageText(func(f *Frame) { got <- "message.text" })
	frame := map[string]any{
		"cmd":     CmdMsgCallback,
		"headers": map[string]any{"req_id": "R3"},
		"body":    map[string]any{"msgtype": "weird"},
	}
	raw, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	c.handleRaw(raw)
	select {
	case name := <-got:
		if name != "message" {
			t.Fatalf("unexpected handler: %s", name)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for base handler")
	}
	select {
	case name := <-got:
		t.Fatalf("unexpected extra handler: %s", name)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestDispatchResponseNotDispatched(t *testing.T) {
	c := NewWSClient(testOptions("ws://127.0.0.1:1"))
	defer c.Close()
	got := make(chan *Frame, 8)
	c.OnMessage(func(f *Frame) { got <- f })
	c.OnEvent(func(f *Frame) { got <- f })
	raw, err := json.Marshal(ackFrame("R4"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	c.handleRaw(raw)
	select {
	case f := <-got:
		t.Fatalf("response frame should not be dispatched: %+v", f)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHandlerPanicIsolated(t *testing.T) {
	c := NewWSClient(testOptions("ws://127.0.0.1:1"))
	defer c.Close()
	got := make(chan struct{}, 8)
	c.OnMessageText(func(f *Frame) {
		got <- struct{}{}
		panic("boom")
	})
	c.OnMessageText(func(f *Frame) {
		got <- struct{}{}
	})
	raw, err := json.Marshal(msgCallbackFrame("R5"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	c.handleRaw(raw)
	deadline := time.After(3 * time.Second)
	n := 0
	for n < 2 {
		select {
		case <-got:
			n++
		case <-deadline:
			t.Fatalf("expected 2 handler calls, got %d", n)
		}
	}
}

func TestLifecycleHandlers(t *testing.T) {
	c := NewWSClient(testOptions("ws://127.0.0.1:1"))
	defer c.Close()
	got := make(chan string, 8)
	c.OnConnected(func() { got <- "connected" })
	c.OnAuthenticated(func() { got <- "authenticated" })
	c.OnDisconnected(func(reason string) { got <- "disconnected" })
	c.OnReconnecting(func(attempt int) { got <- "reconnecting" })
	c.OnError(func(err error) { got <- "error" })
	c.emitConnected()
	c.emitAuthenticated()
	c.emitDisconnected("test")
	c.emitReconnecting(1)
	c.emitError(ErrNotConnected)
	deadline := time.After(3 * time.Second)
	seen := map[string]bool{}
	for len(seen) < 5 {
		select {
		case name := <-got:
			seen[name] = true
		case <-deadline:
			t.Fatalf("expected all lifecycle events, got %v", seen)
		}
	}
}
