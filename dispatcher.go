package aibot

import (
	"encoding/json"
)

func (c *WSClient) On(event string, h func(*Frame)) {
	if h == nil || event == "" {
		return
	}
	c.handlersMu.Lock()
	c.frameHandlers[event] = append(c.frameHandlers[event], h)
	c.handlersMu.Unlock()
}

func (c *WSClient) OnMessage(h func(*Frame)) {
	c.On(EvMessage, h)
}

func (c *WSClient) OnMessageText(h func(*Frame)) {
	c.On(EvMessage+"."+MsgTypeText, h)
}

func (c *WSClient) OnMessageImage(h func(*Frame)) {
	c.On(EvMessage+"."+MsgTypeImage, h)
}

func (c *WSClient) OnMessageMixed(h func(*Frame)) {
	c.On(EvMessage+"."+MsgTypeMixed, h)
}

func (c *WSClient) OnMessageVoice(h func(*Frame)) {
	c.On(EvMessage+"."+MsgTypeVoice, h)
}

func (c *WSClient) OnMessageFile(h func(*Frame)) {
	c.On(EvMessage+"."+MsgTypeFile, h)
}

func (c *WSClient) OnMessageVideo(h func(*Frame)) {
	c.On(EvMessage+"."+MsgTypeVideo, h)
}

func (c *WSClient) OnEvent(h func(*Frame)) {
	c.On(EvEvent, h)
}

func (c *WSClient) OnEventEnterChat(h func(*Frame)) {
	c.On(EvEvent+"."+EventTypeEnterChat, h)
}

func (c *WSClient) OnEventTemplateCard(h func(*Frame)) {
	c.On(EvEvent+"."+EventTypeTemplateCard, h)
}

func (c *WSClient) OnEventFeedback(h func(*Frame)) {
	c.On(EvEvent+"."+EventTypeFeedback, h)
}

func (c *WSClient) OnEventDisconnected(h func(*Frame)) {
	c.On(EvEvent+"."+EventTypeDisconnected, h)
}

func (c *WSClient) OnConnected(h func()) {
	if h == nil {
		return
	}
	c.handlersMu.Lock()
	c.connectedH = append(c.connectedH, h)
	c.handlersMu.Unlock()
}

func (c *WSClient) OnAuthenticated(h func()) {
	if h == nil {
		return
	}
	c.handlersMu.Lock()
	c.authenticatedH = append(c.authenticatedH, h)
	c.handlersMu.Unlock()
}

func (c *WSClient) OnDisconnected(h func(reason string)) {
	if h == nil {
		return
	}
	c.handlersMu.Lock()
	c.disconnectedH = append(c.disconnectedH, h)
	c.handlersMu.Unlock()
}

func (c *WSClient) OnReconnecting(h func(attempt int)) {
	if h == nil {
		return
	}
	c.handlersMu.Lock()
	c.reconnectingH = append(c.reconnectingH, h)
	c.handlersMu.Unlock()
}

func (c *WSClient) OnError(h func(err error)) {
	if h == nil {
		return
	}
	c.handlersMu.Lock()
	c.errorH = append(c.errorH, h)
	c.handlersMu.Unlock()
}

func (c *WSClient) handleRaw(data []byte) {
	var f Frame
	if err := json.Unmarshal(data, &f); err != nil {
		c.log.Warnf("invalid frame: %v", err)
		return
	}
	if f.Cmd == "" {
		c.resolvePending(&f)
		return
	}
	switch f.Cmd {
	case CmdMsgCallback:
		msgType := ""
		var b MsgBody
		if json.Unmarshal(f.Body, &b) == nil {
			msgType = b.MsgType
		}
		if msgType != "" {
			c.dispatch(&f, EvMessage, EvMessage+"."+msgType)
		} else {
			c.dispatch(&f, EvMessage)
		}
	case CmdEventCallback:
		evType := ""
		var b EventBody
		if json.Unmarshal(f.Body, &b) == nil {
			evType = b.Event.EventType
		}
		if evType == EventTypeDisconnected {
			c.log.Warnf("received disconnected_event, this connection has been replaced by a new one")
		}
		if evType != "" {
			c.dispatch(&f, EvEvent, EvEvent+"."+evType)
		} else {
			c.dispatch(&f, EvEvent)
		}
	default:
		c.log.Debugf("unknown cmd: %s", f.Cmd)
	}
}

func (c *WSClient) dispatch(f *Frame, names ...string) {
	c.handlersMu.RLock()
	var hs []func(*Frame)
	for _, n := range names {
		hs = append(hs, c.frameHandlers[n]...)
	}
	c.handlersMu.RUnlock()
	if len(hs) == 0 {
		return
	}
	go func() {
		for _, h := range hs {
			c.safeFrameCall(h, f)
		}
	}()
}

func (c *WSClient) safeFrameCall(h func(*Frame), f *Frame) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Errorf("message handler panic: %v", r)
		}
	}()
	h(f)
}

func (c *WSClient) emitConnected() {
	c.handlersMu.RLock()
	hs := append([]func(){}, c.connectedH...)
	c.handlersMu.RUnlock()
	for _, h := range hs {
		go c.safeCall0(h)
	}
}

func (c *WSClient) emitAuthenticated() {
	c.handlersMu.RLock()
	hs := append([]func(){}, c.authenticatedH...)
	c.handlersMu.RUnlock()
	for _, h := range hs {
		go c.safeCall0(h)
	}
}

func (c *WSClient) emitDisconnected(reason string) {
	c.handlersMu.RLock()
	hs := append([]func(string){}, c.disconnectedH...)
	c.handlersMu.RUnlock()
	for _, h := range hs {
		go func(h func(string)) {
			defer func() {
				if r := recover(); r != nil {
					c.log.Errorf("disconnected handler panic: %v", r)
				}
			}()
			h(reason)
		}(h)
	}
}

func (c *WSClient) emitReconnecting(attempt int) {
	c.handlersMu.RLock()
	hs := append([]func(int){}, c.reconnectingH...)
	c.handlersMu.RUnlock()
	for _, h := range hs {
		go func(h func(int)) {
			defer func() {
				if r := recover(); r != nil {
					c.log.Errorf("reconnecting handler panic: %v", r)
				}
			}()
			h(attempt)
		}(h)
	}
}

func (c *WSClient) emitError(err error) {
	c.handlersMu.RLock()
	hs := append([]func(error){}, c.errorH...)
	c.handlersMu.RUnlock()
	for _, h := range hs {
		go func(h func(error)) {
			defer func() {
				if r := recover(); r != nil {
					c.log.Errorf("error handler panic: %v", r)
				}
			}()
			h(err)
		}(h)
	}
}

func (c *WSClient) safeCall0(h func()) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Errorf("handler panic: %v", r)
		}
	}()
	h()
}
