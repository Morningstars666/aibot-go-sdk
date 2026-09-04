package aibot

import "time"

const DefaultWsURL = "wss://openws.work.weixin.qq.com"

type Options struct {
	BotID                 string
	Secret                string
	WsURL                 string
	HeartbeatInterval     time.Duration
	HeartbeatMaxMisses    int
	ReconnectBaseInterval time.Duration
	ReconnectMaxInterval  time.Duration
	MaxReconnectAttempts  int
	RequestTimeout        time.Duration
	HTTPTimeout           time.Duration
	AutoReconnect         bool
	Logger                Logger
}

func DefaultOptions() *Options {
	return &Options{
		WsURL:                 DefaultWsURL,
		HeartbeatInterval:     30 * time.Second,
		HeartbeatMaxMisses:    3,
		ReconnectBaseInterval: 1 * time.Second,
		ReconnectMaxInterval:  30 * time.Second,
		MaxReconnectAttempts:  10,
		RequestTimeout:        10 * time.Second,
		HTTPTimeout:           60 * time.Second,
		AutoReconnect:         true,
	}
}

func (o *Options) normalize() error {
	if o.BotID == "" {
		return ErrInvalidBotID
	}
	if o.Secret == "" {
		return ErrInvalidSecret
	}
	def := DefaultOptions()
	if o.WsURL == "" {
		o.WsURL = def.WsURL
	}
	if o.HeartbeatInterval <= 0 {
		o.HeartbeatInterval = def.HeartbeatInterval
	}
	if o.HeartbeatMaxMisses <= 0 {
		o.HeartbeatMaxMisses = def.HeartbeatMaxMisses
	}
	if o.ReconnectBaseInterval <= 0 {
		o.ReconnectBaseInterval = def.ReconnectBaseInterval
	}
	if o.ReconnectMaxInterval <= 0 {
		o.ReconnectMaxInterval = def.ReconnectMaxInterval
	}
	if o.RequestTimeout <= 0 {
		o.RequestTimeout = def.RequestTimeout
	}
	if o.HTTPTimeout <= 0 {
		o.HTTPTimeout = def.HTTPTimeout
	}
	if o.Logger == nil {
		o.Logger = &DefaultLogger{}
	}
	return nil
}
