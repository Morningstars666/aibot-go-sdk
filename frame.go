package aibot

import (
	"encoding/json"
	"errors"
)

type Headers struct {
	ReqID string `json:"req_id,omitempty"`
}

type Frame struct {
	Cmd     string          `json:"cmd,omitempty"`
	Headers Headers         `json:"headers"`
	Body    json.RawMessage `json:"body,omitempty"`
	Errcode int             `json:"errcode,omitempty"`
	Errmsg  string          `json:"errmsg,omitempty"`
}

func (f *Frame) ReqID() string {
	if f == nil {
		return ""
	}
	return f.Headers.ReqID
}

func (f *Frame) OK() bool {
	return f != nil && f.Errcode == 0
}

func (f *Frame) AsError() error {
	if f == nil {
		return errors.New("aibot: nil response frame")
	}
	if f.Errcode != 0 {
		return &RespError{Errcode: f.Errcode, Errmsg: f.Errmsg}
	}
	return nil
}

func (f *Frame) ParseBody(v any) error {
	if f == nil || len(f.Body) == 0 {
		return nil
	}
	return json.Unmarshal(f.Body, v)
}

type From struct {
	UserID string `json:"userid,omitempty"`
}

type TextContent struct {
	Content string `json:"content,omitempty"`
}

type MediaInfo struct {
	MediaID string `json:"media_id,omitempty"`
	URL     string `json:"url,omitempty"`
	AESKey  string `json:"aeskey,omitempty"`
}

type VideoInfo struct {
	MediaID     string `json:"media_id,omitempty"`
	URL         string `json:"url,omitempty"`
	AESKey      string `json:"aeskey,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type VoiceInfo struct {
	MediaID string `json:"media_id,omitempty"`
	URL     string `json:"url,omitempty"`
	AESKey  string `json:"aeskey,omitempty"`
	Content string `json:"content,omitempty"`
	Format  string `json:"format,omitempty"`
}

type MixedItem struct {
	MsgType string       `json:"msgtype,omitempty"`
	Text    *TextContent `json:"text,omitempty"`
	Image   *MediaInfo   `json:"image,omitempty"`
}

type MixedInfo struct {
	Items []MixedItem `json:"items,omitempty"`
}

type EventContent struct {
	EventType string          `json:"eventtype,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

func (e *EventContent) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	var probe struct {
		EventType string `json:"eventtype"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}
	e.EventType = probe.EventType
	e.Raw = append(e.Raw[:0], data...)
	return nil
}

func (e EventContent) MarshalJSON() ([]byte, error) {
	if len(e.Raw) > 0 {
		return e.Raw, nil
	}
	return json.Marshal(struct {
		EventType string `json:"eventtype,omitempty"`
	}{EventType: e.EventType})
}

type MsgBody struct {
	MsgID      string       `json:"msgid,omitempty"`
	CreateTime int64        `json:"create_time,omitempty"`
	AibotID    string       `json:"aibotid,omitempty"`
	ChatID     string       `json:"chatid,omitempty"`
	ChatType   string       `json:"chattype,omitempty"`
	From       *From        `json:"from,omitempty"`
	MsgType    string       `json:"msgtype,omitempty"`
	Text       *TextContent `json:"text,omitempty"`
	Image      *MediaInfo   `json:"image,omitempty"`
	Voice      *VoiceInfo   `json:"voice,omitempty"`
	Video      *VideoInfo   `json:"video,omitempty"`
	File       *MediaInfo   `json:"file,omitempty"`
	Mixed      *MixedInfo   `json:"mixed,omitempty"`
}

type EventBody struct {
	MsgID      string       `json:"msgid,omitempty"`
	CreateTime int64        `json:"create_time,omitempty"`
	AibotID    string       `json:"aibotid,omitempty"`
	ChatID     string       `json:"chatid,omitempty"`
	ChatType   string       `json:"chattype,omitempty"`
	From       *From        `json:"from,omitempty"`
	MsgType    string       `json:"msgtype,omitempty"`
	Event      EventContent `json:"event,omitempty"`
}

type Feedback struct {
	ID string `json:"id,omitempty"`
}

type StreamBody struct {
	ID       string      `json:"id"`
	Finish   bool        `json:"finish"`
	Content  string      `json:"content,omitempty"`
	MsgItem  []MixedItem `json:"msg_item,omitempty"`
	Feedback *Feedback   `json:"feedback,omitempty"`
}

type StreamMsg struct {
	MsgType string     `json:"msgtype"`
	Stream  StreamBody `json:"stream"`
}

type StreamWithCardMsg struct {
	MsgType      string          `json:"msgtype"`
	Stream       StreamBody      `json:"stream"`
	TemplateCard json.RawMessage `json:"template_card,omitempty"`
}

type TemplateCardMsg struct {
	MsgType      string `json:"msgtype"`
	TemplateCard any    `json:"template_card"`
}

type subscribeBody struct {
	BotID  string `json:"bot_id"`
	Secret string `json:"secret"`
}
