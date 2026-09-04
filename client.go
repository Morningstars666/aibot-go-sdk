package aibot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

const maxFrameSize int64 = 1 << 26

type connState struct {
	conn     *websocket.Conn
	closed   chan struct{}
	suppress atomic.Bool
	once     sync.Once
}

func (st *connState) markClosed() {
	st.once.Do(func() { close(st.closed) })
}

type pendingResult struct {
	frame *Frame
	err   error
}

type WSClient struct {
	opts *Options
	log  Logger
	http *http.Client

	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once

	closeOnce sync.Once
	closed    atomic.Bool
	authed    atomic.Bool
	fatal     atomic.Pointer[error]
	cur       atomic.Pointer[connState]
	writeMu   sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]chan pendingResult

	replyLocks *keyedMutex

	handlersMu     sync.RWMutex
	frameHandlers  map[string][]func(*Frame)
	connectedH     []func()
	authenticatedH []func()
	disconnectedH  []func(string)
	reconnectingH  []func(int)
	errorH         []func(error)

	reconnMu     sync.Mutex
	reconnecting bool
}

func NewWSClient(opts *Options) *WSClient {
	if opts == nil {
		opts = DefaultOptions()
	}
	if opts.Logger == nil {
		opts.Logger = &DefaultLogger{}
	}
	o := *opts
	c := &WSClient{
		opts:          &o,
		log:           o.Logger,
		http:          &http.Client{},
		pending:       make(map[string]chan pendingResult),
		replyLocks:    newKeyedMutex(),
		frameHandlers: make(map[string][]func(*Frame)),
	}
	c.initCtx()
	return c
}

func (c *WSClient) initCtx() {
	c.once.Do(func() {
		c.ctx, c.cancel = context.WithCancel(context.Background())
	})
}

func (c *WSClient) IsConnected() bool {
	return c.cur.Load() != nil
}

func (c *WSClient) IsAuthenticated() bool {
	return c.authed.Load()
}

func (c *WSClient) Connect(ctx context.Context) error {
	if c.closed.Load() {
		return ErrClosed
	}
	if err := c.opts.normalize(); err != nil {
		return err
	}
	c.log = c.opts.Logger
	if c.cur.Load() != nil {
		return ErrAlreadyConnected
	}
	return c.dial(ctx)
}

func (c *WSClient) Run(ctx context.Context) error {
	err := c.Connect(ctx)
	if err != nil {
		if !c.opts.AutoReconnect {
			return err
		}
		if errors.Is(err, ErrClosed) || errors.Is(err, ErrInvalidBotID) || errors.Is(err, ErrInvalidSecret) {
			return err
		}
		if derr := c.retryDial(ctx); derr != nil {
			return derr
		}
	}
	select {
	case <-c.ctx.Done():
	case <-ctx.Done():
		c.Close()
	}
	if p := c.fatal.Load(); p != nil {
		return *p
	}
	if c.closed.Load() {
		return nil
	}
	return c.ctx.Err()
}

func (c *WSClient) Close() {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		if c.cancel != nil {
			c.cancel()
		}
		if st := c.cur.Swap(nil); st != nil {
			_ = st.conn.Close(websocket.StatusNormalClosure, "client closed")
			st.markClosed()
		}
		c.failAllPending(ErrClosed)
	})
}

func (c *WSClient) dial(ctx context.Context) error {
	dctx, dcancel := context.WithTimeout(ctx, c.opts.RequestTimeout)
	defer dcancel()
	conn, _, err := websocket.Dial(dctx, c.opts.WsURL, nil)
	if err != nil {
		return fmt.Errorf("aibot: dial %s: %w", c.opts.WsURL, err)
	}
	conn.SetReadLimit(maxFrameSize)
	st := &connState{conn: conn, closed: make(chan struct{})}
	if prev := c.cur.Swap(st); prev != nil {
		prev.suppress.Store(true)
		_ = prev.conn.CloseNow()
		prev.markClosed()
	}
	go c.readPump(st)
	c.emitConnected()
	reqID := GenerateReqID()
	body, merr := json.Marshal(subscribeBody{BotID: c.opts.BotID, Secret: c.opts.Secret})
	if merr != nil {
		c.abortConn(st)
		return fmt.Errorf("aibot: marshal subscribe: %w", merr)
	}
	resp, rerr := c.request(ctx, reqID, &Frame{
		Cmd:     CmdSubscribe,
		Headers: Headers{ReqID: reqID},
		Body:    body,
	})
	if rerr != nil {
		c.abortConn(st)
		return fmt.Errorf("aibot: subscribe: %w", rerr)
	}
	if resp.Errcode != 0 {
		c.abortConn(st)
		return &RespError{Errcode: resp.Errcode, Errmsg: resp.Errmsg}
	}
	c.authed.Store(true)
	go c.heartbeatLoop(st)
	c.emitAuthenticated()
	c.log.Infof("connected and authenticated (bot_id=%s)", c.opts.BotID)
	return nil
}

func (c *WSClient) abortConn(st *connState) {
	st.suppress.Store(true)
	_ = st.conn.Close(websocket.StatusNormalClosure, "subscribe failed")
	st.markClosed()
	if c.cur.CompareAndSwap(st, nil) {
		c.failAllPending(ErrConnClosed)
	}
}

func (c *WSClient) readPump(st *connState) {
	reason := "connection closed"
	for {
		_, data, err := st.conn.Read(c.ctx)
		if err != nil {
			reason = err.Error()
			break
		}
		c.handleRaw(data)
	}
	st.markClosed()
	if !c.cur.CompareAndSwap(st, nil) {
		return
	}
	c.failAllPending(ErrConnClosed)
	if c.closed.Load() || st.suppress.Load() {
		return
	}
	c.authed.Store(false)
	c.log.Warnf("connection lost: %s", reason)
	c.emitDisconnected(reason)
	go c.reconnectLoop()
}

func (c *WSClient) heartbeatLoop(st *connState) {
	ticker := time.NewTicker(c.opts.HeartbeatInterval)
	defer ticker.Stop()
	misses := 0
	for {
		select {
		case <-st.closed:
			return
		case <-c.ctx.Done():
			return
		case <-ticker.C:
		}
		if c.cur.Load() != st {
			return
		}
		reqID := GenerateReqID()
		resp, err := c.request(c.ctx, reqID, &Frame{
			Cmd:     CmdPing,
			Headers: Headers{ReqID: reqID},
		})
		if err == nil && resp.OK() {
			misses = 0
			c.log.Debugf("heartbeat ok")
			continue
		}
		misses++
		if err != nil {
			c.log.Warnf("heartbeat miss (%d/%d): %v", misses, c.opts.HeartbeatMaxMisses, err)
		} else {
			c.log.Warnf("heartbeat miss (%d/%d): errcode=%d errmsg=%s", misses, c.opts.HeartbeatMaxMisses, resp.Errcode, resp.Errmsg)
		}
		if misses >= c.opts.HeartbeatMaxMisses {
			c.log.Warnf("heartbeat failed %d times, closing connection", misses)
			_ = st.conn.CloseNow()
			return
		}
	}
}

func (c *WSClient) reconnectLoop() {
	c.reconnMu.Lock()
	if c.reconnecting {
		c.reconnMu.Unlock()
		return
	}
	c.reconnecting = true
	c.reconnMu.Unlock()

	err := c.retryDial(c.ctx)
	c.reconnMu.Lock()
	c.reconnecting = false
	c.reconnMu.Unlock()

	if err != nil && !c.closed.Load() {
		p := err
		c.fatal.CompareAndSwap(nil, &p)
		c.emitError(err)
		c.log.Errorf("reconnect loop exits: %v", err)
		c.cancel()
	}
}

func (c *WSClient) retryDial(ctx context.Context) error {
	for attempt := 1; ; attempt++ {
		if c.opts.MaxReconnectAttempts >= 0 && attempt > c.opts.MaxReconnectAttempts {
			return fmt.Errorf("aibot: reconnect attempts exhausted (%d)", c.opts.MaxReconnectAttempts)
		}
		c.emitReconnecting(attempt)
		delay := c.backoff(attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.ctx.Done():
			return ErrClosed
		case <-time.After(delay):
		}
		if err := c.dial(c.ctx); err != nil {
			c.log.Warnf("reconnect attempt %d failed: %v", attempt, err)
			continue
		}
		return nil
	}
}

func (c *WSClient) backoff(attempt int) time.Duration {
	d := c.opts.ReconnectBaseInterval << (attempt - 1)
	if attempt-1 > 16 || d > c.opts.ReconnectMaxInterval || d <= 0 {
		d = c.opts.ReconnectMaxInterval
	}
	return d
}

func (c *WSClient) getConn() *websocket.Conn {
	if st := c.cur.Load(); st != nil {
		return st.conn
	}
	return nil
}

func (c *WSClient) addPending(reqID string, ch chan pendingResult) bool {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if c.closed.Load() {
		return false
	}
	c.pending[reqID] = ch
	return true
}

func (c *WSClient) removePending(reqID string) {
	c.pendingMu.Lock()
	delete(c.pending, reqID)
	c.pendingMu.Unlock()
}

func (c *WSClient) resolvePending(f *Frame) {
	reqID := f.Headers.ReqID
	if reqID == "" {
		return
	}
	c.pendingMu.Lock()
	ch, ok := c.pending[reqID]
	c.pendingMu.Unlock()
	if !ok {
		c.log.Debugf("no pending request for req_id=%s", reqID)
		return
	}
	select {
	case ch <- pendingResult{frame: f}:
	default:
	}
}

func (c *WSClient) failAllPending(err error) {
	c.pendingMu.Lock()
	for id, ch := range c.pending {
		select {
		case ch <- pendingResult{err: err}:
		default:
		}
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()
}

func (c *WSClient) request(ctx context.Context, reqID string, f *Frame) (*Frame, error) {
	if c.getConn() == nil {
		return nil, ErrNotConnected
	}
	ch := make(chan pendingResult, 1)
	if !c.addPending(reqID, ch) {
		return nil, ErrClosed
	}
	defer c.removePending(reqID)
	data, err := json.Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("aibot: marshal frame: %w", err)
	}
	if err := c.writeData(ctx, data); err != nil {
		return nil, err
	}
	timer := time.NewTimer(c.opts.RequestTimeout)
	defer timer.Stop()
	select {
	case r := <-ch:
		return r.frame, r.err
	case <-timer.C:
		return nil, ErrRequestTimeout
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, ErrClosed
	}
}

func (c *WSClient) writeData(ctx context.Context, data []byte) error {
	st := c.cur.Load()
	if st == nil {
		return ErrNotConnected
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	wctx, wcancel := context.WithTimeout(ctx, c.opts.RequestTimeout)
	defer wcancel()
	return st.conn.Write(wctx, websocket.MessageText, data)
}

type keyedMutex struct {
	mu    sync.Mutex
	locks map[string]*keyedLock
}

type keyedLock struct {
	refs int
	mu   sync.Mutex
}

func newKeyedMutex() *keyedMutex {
	return &keyedMutex{locks: make(map[string]*keyedLock)}
}

func (k *keyedMutex) Lock(key string) func() {
	k.mu.Lock()
	l := k.locks[key]
	if l == nil {
		l = &keyedLock{}
		k.locks[key] = l
	}
	l.refs++
	k.mu.Unlock()

	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		k.mu.Lock()
		l.refs--
		if l.refs == 0 {
			delete(k.locks, key)
		}
		k.mu.Unlock()
	}
}
