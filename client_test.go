package aibot

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func testOptions(rawURL string) *Options {
	o := DefaultOptions()
	o.BotID = "test-bot"
	o.Secret = "test-secret"
	o.WsURL = "ws" + strings.TrimPrefix(rawURL, "http")
	o.HeartbeatInterval = time.Hour
	o.RequestTimeout = 2 * time.Second
	o.HTTPTimeout = 2 * time.Second
	o.ReconnectBaseInterval = 5 * time.Millisecond
	o.ReconnectMaxInterval = 50 * time.Millisecond
	o.Logger = NopLogger{}
	return o
}

func newWSServer(t *testing.T, handler func(ctx context.Context, conn *websocket.Conn)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		conn.SetReadLimit(1 << 26)
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handler(ctx, conn)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func readFrame(ctx context.Context, t *testing.T, conn *websocket.Conn) (map[string]any, error) {
	t.Helper()
	_, data, err := conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	var f map[string]any
	if err := json.Unmarshal(data, &f); err != nil {
		t.Errorf("server: invalid frame: %v", err)
		return nil, err
	}
	return f, nil
}

func writeFrame(ctx context.Context, t *testing.T, conn *websocket.Conn, v any) error {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

func frameReqID(f map[string]any) string {
	if h, ok := f["headers"].(map[string]any); ok {
		if id, ok := h["req_id"].(string); ok {
			return id
		}
	}
	return ""
}

func ackFrame(reqID string) map[string]any {
	return map[string]any{
		"headers": map[string]any{"req_id": reqID},
		"errcode": 0,
		"errmsg":  "ok",
	}
}

func ackBody(reqID string, body map[string]any) map[string]any {
	f := ackFrame(reqID)
	f["body"] = body
	return f
}

func msgCallbackFrame(reqID string) map[string]any {
	return map[string]any{
		"cmd":     CmdMsgCallback,
		"headers": map[string]any{"req_id": reqID},
		"body": map[string]any{
			"msgid":    "MSGID",
			"aibotid":  "AIBOTID",
			"chatid":   "CHATID",
			"chattype": ChatGroup,
			"from":     map[string]any{"userid": "USERID"},
			"msgtype":  MsgTypeText,
			"text":     map[string]any{"content": "hello robot"},
		},
	}
}

func eventCallbackFrame(reqID, eventType string) map[string]any {
	return map[string]any{
		"cmd":     CmdEventCallback,
		"headers": map[string]any{"req_id": reqID},
		"body": map[string]any{
			"msgid":       "MSGID",
			"create_time": 1700000000,
			"aibotid":     "AIBOTID",
			"msgtype":     MsgTypeEvent,
			"event":       map[string]any{"eventtype": eventType},
		},
	}
}

func serveAckLoop(ctx context.Context, t *testing.T, conn *websocket.Conn, capture func(f map[string]any)) {
	for {
		f, err := readFrame(ctx, t, conn)
		if err != nil {
			return
		}
		if capture != nil {
			capture(f)
		}
		switch f["cmd"] {
		case CmdSubscribe, CmdPing, CmdRespondMsg, CmdRespondWelcome, CmdRespondUpdate, CmdSendMsg:
			if err := writeFrame(ctx, t, conn, ackFrame(frameReqID(f))); err != nil {
				return
			}
		}
	}
}

func TestConnectSubscribeAck(t *testing.T) {
	subCh := make(chan map[string]any, 1)
	srv := newWSServer(t, func(ctx context.Context, conn *websocket.Conn) {
		f, err := readFrame(ctx, t, conn)
		if err != nil {
			return
		}
		if f["cmd"] != CmdSubscribe {
			t.Errorf("expected subscribe, got %v", f["cmd"])
			return
		}
		subCh <- f
		if err := writeFrame(ctx, t, conn, ackFrame(frameReqID(f))); err != nil {
			return
		}
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	})
	c := NewWSClient(testOptions(srv.URL))
	defer c.Close()
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if !c.IsAuthenticated() {
		t.Fatal("expected authenticated")
	}
	if !c.IsConnected() {
		t.Fatal("expected connected")
	}
	select {
	case f := <-subCh:
		body, _ := f["body"].(map[string]any)
		if body["bot_id"] != "test-bot" || body["secret"] != "test-secret" {
			t.Fatalf("unexpected subscribe body: %v", body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for subscribe frame")
	}
}

func TestConnectSubscribeRejected(t *testing.T) {
	var count atomic.Int32
	srv := newWSServer(t, func(ctx context.Context, conn *websocket.Conn) {
		f, err := readFrame(ctx, t, conn)
		if err != nil {
			return
		}
		if f["cmd"] != CmdSubscribe {
			return
		}
		count.Add(1)
		if err := writeFrame(ctx, t, conn, map[string]any{
			"headers": map[string]any{"req_id": frameReqID(f)},
			"errcode": 90001,
			"errmsg":  "invalid secret",
		}); err != nil {
			return
		}
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	})
	c := NewWSClient(testOptions(srv.URL))
	defer c.Close()
	err := c.Connect(context.Background())
	var respErr *RespError
	if !errors.As(err, &respErr) || respErr.Errcode != 90001 {
		t.Fatalf("expected RespError 90001, got %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if n := count.Load(); n != 1 {
		t.Fatalf("expected exactly 1 subscribe, got %d", n)
	}
	if c.IsConnected() {
		t.Fatal("expected disconnected after reject")
	}
}

func TestHeartbeatPing(t *testing.T) {
	pingCh := make(chan struct{}, 64)
	srv := newWSServer(t, func(ctx context.Context, conn *websocket.Conn) {
		for {
			f, err := readFrame(ctx, t, conn)
			if err != nil {
				return
			}
			switch f["cmd"] {
			case CmdSubscribe:
				if err := writeFrame(ctx, t, conn, ackFrame(frameReqID(f))); err != nil {
					return
				}
			case CmdPing:
				select {
				case pingCh <- struct{}{}:
				default:
				}
				if err := writeFrame(ctx, t, conn, ackFrame(frameReqID(f))); err != nil {
					return
				}
			}
		}
	})
	opts := testOptions(srv.URL)
	opts.HeartbeatInterval = 30 * time.Millisecond
	c := NewWSClient(opts)
	defer c.Close()
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	deadline := time.After(3 * time.Second)
	n := 0
	for n < 3 {
		select {
		case <-pingCh:
			n++
		case <-deadline:
			t.Fatalf("expected at least 3 pings, got %d", n)
		}
	}
}

func TestReplyStreamPassthrough(t *testing.T) {
	respondCh := make(chan map[string]any, 4)
	srv := newWSServer(t, func(ctx context.Context, conn *websocket.Conn) {
		f, err := readFrame(ctx, t, conn)
		if err != nil {
			return
		}
		if f["cmd"] != CmdSubscribe {
			return
		}
		if err := writeFrame(ctx, t, conn, ackFrame(frameReqID(f))); err != nil {
			return
		}
		if err := writeFrame(ctx, t, conn, msgCallbackFrame("req-1")); err != nil {
			return
		}
		serveAckLoop(ctx, t, conn, func(f map[string]any) {
			if f["cmd"] == CmdRespondMsg {
				respondCh <- f
			}
		})
	})
	c := NewWSClient(testOptions(srv.URL))
	defer c.Close()
	done := make(chan struct{})
	c.OnMessageText(func(frame *Frame) {
		_, err := c.ReplyStream(context.Background(), frame, "stream-1", "hi there", true)
		if err != nil {
			t.Errorf("reply stream: %v", err)
		}
		close(done)
	})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for reply")
	}
	select {
	case f := <-respondCh:
		if frameReqID(f) != "req-1" {
			t.Fatalf("req_id not passed through: %v", f)
		}
		body, _ := f["body"].(map[string]any)
		if body["msgtype"] != MsgTypeStream {
			t.Fatalf("expected stream msgtype, got %v", body["msgtype"])
		}
		stream, _ := body["stream"].(map[string]any)
		if stream["id"] != "stream-1" || stream["content"] != "hi there" || stream["finish"] != true {
			t.Fatalf("unexpected stream body: %v", stream)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for respond frame")
	}
}

func TestReplyWelcomeAndUpdateCard(t *testing.T) {
	welcomeCh := make(chan map[string]any, 4)
	updateCh := make(chan map[string]any, 4)
	srv := newWSServer(t, func(ctx context.Context, conn *websocket.Conn) {
		f, err := readFrame(ctx, t, conn)
		if err != nil {
			return
		}
		if f["cmd"] != CmdSubscribe {
			return
		}
		if err := writeFrame(ctx, t, conn, ackFrame(frameReqID(f))); err != nil {
			return
		}
		if err := writeFrame(ctx, t, conn, eventCallbackFrame("evt-1", EventTypeEnterChat)); err != nil {
			return
		}
		if err := writeFrame(ctx, t, conn, eventCallbackFrame("evt-2", EventTypeTemplateCard)); err != nil {
			return
		}
		serveAckLoop(ctx, t, conn, func(f map[string]any) {
			switch f["cmd"] {
			case CmdRespondWelcome:
				welcomeCh <- f
			case CmdRespondUpdate:
				updateCh <- f
			}
		})
	})
	c := NewWSClient(testOptions(srv.URL))
	defer c.Close()
	welcomeDone := make(chan error, 1)
	updateDone := make(chan error, 1)
	c.OnEventEnterChat(func(frame *Frame) {
		_, err := c.ReplyWelcome(context.Background(), frame, map[string]any{
			"msgtype": MsgTypeText,
			"text":    map[string]any{"content": "welcome"},
		})
		welcomeDone <- err
	})
	c.OnEventTemplateCard(func(frame *Frame) {
		_, err := c.UpdateTemplateCard(context.Background(), frame, map[string]any{
			"card_type": "text_notice",
		}, "USERID")
		updateDone <- err
	})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	select {
	case err := <-welcomeDone:
		if err != nil {
			t.Fatalf("reply welcome: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for welcome handler")
	}
	select {
	case err := <-updateDone:
		if err != nil {
			t.Fatalf("update template card: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for update handler")
	}
	select {
	case f := <-welcomeCh:
		if frameReqID(f) != "evt-1" {
			t.Fatalf("welcome req_id mismatch: %v", f)
		}
		body, _ := f["body"].(map[string]any)
		if body["msgtype"] != MsgTypeText {
			t.Fatalf("unexpected welcome body: %v", body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for welcome frame")
	}
	select {
	case f := <-updateCh:
		if frameReqID(f) != "evt-2" {
			t.Fatalf("update req_id mismatch: %v", f)
		}
		body, _ := f["body"].(map[string]any)
		if body["response_type"] != ResponseTypeUpdateTemplateCard {
			t.Fatalf("unexpected update body: %v", body)
		}
		if ids, ok := body["userids"].([]any); !ok || len(ids) != 1 || ids[0] != "USERID" {
			t.Fatalf("unexpected userids: %v", body["userids"])
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for update frame")
	}
}

func TestSendMessageMergesBody(t *testing.T) {
	sendCh := make(chan map[string]any, 4)
	srv := newWSServer(t, func(ctx context.Context, conn *websocket.Conn) {
		serveAckLoop(ctx, t, conn, func(f map[string]any) {
			if f["cmd"] == CmdSendMsg {
				sendCh <- f
			}
		})
	})
	c := NewWSClient(testOptions(srv.URL))
	defer c.Close()
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := c.SendMessage(context.Background(), "chat-1", map[string]any{
		"msgtype":  MsgTypeMarkdown,
		"markdown": map[string]any{"content": "push"},
	}, ChatTypeGroup); err != nil {
		t.Fatalf("send message: %v", err)
	}
	select {
	case f := <-sendCh:
		body, _ := f["body"].(map[string]any)
		if body["chatid"] != "chat-1" {
			t.Fatalf("unexpected chatid: %v", body["chatid"])
		}
		if ct, ok := body["chat_type"].(float64); !ok || ct != float64(ChatTypeGroup) {
			t.Fatalf("unexpected chat_type: %v", body["chat_type"])
		}
		if body["msgtype"] != MsgTypeMarkdown {
			t.Fatalf("unexpected msgtype: %v", body["msgtype"])
		}
		if frameReqID(f) == "" {
			t.Fatal("expected non-empty req_id")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for send frame")
	}
	if _, err := c.SendMessage(context.Background(), "chat-1", map[string]any{
		"msgtype":  MsgTypeMarkdown,
		"markdown": map[string]any{"content": "push"},
	}); err != nil {
		t.Fatalf("send message: %v", err)
	}
	select {
	case f := <-sendCh:
		body, _ := f["body"].(map[string]any)
		if _, ok := body["chat_type"]; ok {
			t.Fatalf("chat_type should be omitted when auto: %v", body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for second send frame")
	}
}

func TestReconnectAfterDrop(t *testing.T) {
	subCh := make(chan struct{}, 8)
	var n atomic.Int32
	srv := newWSServer(t, func(ctx context.Context, conn *websocket.Conn) {
		f, err := readFrame(ctx, t, conn)
		if err != nil {
			return
		}
		if f["cmd"] != CmdSubscribe {
			return
		}
		subCh <- struct{}{}
		if err := writeFrame(ctx, t, conn, ackFrame(frameReqID(f))); err != nil {
			return
		}
		if n.Add(1) == 1 {
			return
		}
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	})
	reconnecting := make(chan int, 16)
	c := NewWSClient(testOptions(srv.URL))
	defer c.Close()
	c.OnReconnecting(func(attempt int) { reconnecting <- attempt })
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	deadline := time.After(5 * time.Second)
	subs := 0
	for subs < 2 {
		select {
		case <-subCh:
			subs++
		case <-deadline:
			t.Fatalf("expected 2 subscribes, got %d", subs)
		}
	}
	if !c.IsConnected() {
		t.Fatal("expected reconnected state")
	}
	select {
	case <-reconnecting:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected reconnecting event")
	}
}

func TestRunWithCanceledContext(t *testing.T) {
	o := testOptions("ws://127.0.0.1:1")
	o.MaxReconnectAttempts = 2
	c := NewWSClient(o)
	defer c.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.Run(ctx); err == nil {
		t.Fatal("expected error from Run with canceled context")
	}
}

func TestRunWithInvalidOptions(t *testing.T) {
	o := testOptions("ws://127.0.0.1:1")
	o.BotID = ""
	c := NewWSClient(o)
	defer c.Close()
	if err := c.Run(context.Background()); !errors.Is(err, ErrInvalidBotID) {
		t.Fatalf("expected ErrInvalidBotID, got %v", err)
	}
}

func TestBackoff(t *testing.T) {
	c := NewWSClient(DefaultOptions())
	cases := map[int]time.Duration{
		1: 1 * time.Second,
		2: 2 * time.Second,
		3: 4 * time.Second,
		4: 8 * time.Second,
		5: 16 * time.Second,
		6: 30 * time.Second,
		7: 30 * time.Second,
	}
	for attempt, want := range cases {
		if got := c.backoff(attempt); got != want {
			t.Errorf("backoff(%d) = %v, want %v", attempt, got, want)
		}
	}
}

func TestKeyedMutexSerializes(t *testing.T) {
	km := newKeyedMutex()
	var counter int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				unlock := km.Lock("same-key")
				mu.Lock()
				counter++
				mu.Unlock()
				unlock()
			}
		}()
	}
	wg.Wait()
	if counter != 800 {
		t.Fatalf("counter = %d, want 800", counter)
	}
}
