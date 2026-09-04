package aibot

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *WSClient) Reply(ctx context.Context, frame *Frame, body any, cmds ...string) (*Frame, error) {
	reqID := frame.ReqID()
	if reqID == "" {
		reqID = GenerateReqID()
	}
	command := CmdRespondMsg
	if len(cmds) > 0 && cmds[0] != "" {
		command = cmds[0]
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("aibot: marshal reply body: %w", err)
	}
	unlock := c.replyLocks.Lock(reqID)
	defer unlock()
	return c.request(ctx, reqID, &Frame{
		Cmd:     command,
		Headers: Headers{ReqID: reqID},
		Body:    bodyBytes,
	})
}

type StreamOptions struct {
	MsgItem  []MixedItem
	Feedback *Feedback
}

func (c *WSClient) ReplyStream(ctx context.Context, frame *Frame, streamID, content string, finish bool, opts ...*StreamOptions) (*Frame, error) {
	if streamID == "" {
		return nil, fmt.Errorf("aibot: stream id is required")
	}
	body := StreamMsg{
		MsgType: MsgTypeStream,
		Stream: StreamBody{
			ID:      streamID,
			Finish:  finish,
			Content: content,
		},
	}
	if len(opts) > 0 && opts[0] != nil {
		body.Stream.MsgItem = opts[0].MsgItem
		body.Stream.Feedback = opts[0].Feedback
	}
	return c.Reply(ctx, frame, body)
}

func (c *WSClient) ReplyWelcome(ctx context.Context, frame *Frame, body any) (*Frame, error) {
	if frame.ReqID() == "" {
		return nil, fmt.Errorf("aibot: welcome reply requires the event callback frame req_id")
	}
	return c.Reply(ctx, frame, body, CmdRespondWelcome)
}

type StreamWithCardOptions struct {
	MsgItem        []MixedItem
	StreamFeedback *Feedback
	TemplateCard   any
	CardFeedback   *Feedback
}

func (c *WSClient) ReplyStreamWithCard(ctx context.Context, frame *Frame, streamID, content string, finish bool, opts *StreamWithCardOptions) (*Frame, error) {
	if streamID == "" {
		return nil, fmt.Errorf("aibot: stream id is required")
	}
	body := StreamWithCardMsg{
		MsgType: MsgTypeStream,
		Stream: StreamBody{
			ID:      streamID,
			Finish:  finish,
			Content: content,
		},
	}
	if opts != nil {
		body.Stream.MsgItem = opts.MsgItem
		body.Stream.Feedback = opts.StreamFeedback
		if opts.TemplateCard != nil {
			cardBytes, err := marshalWithFeedback(opts.TemplateCard, opts.CardFeedback)
			if err != nil {
				return nil, err
			}
			body.TemplateCard = cardBytes
		}
	}
	return c.Reply(ctx, frame, body)
}

func (c *WSClient) ReplyTemplateCard(ctx context.Context, frame *Frame, card any, feedback ...*Feedback) (*Frame, error) {
	var fb *Feedback
	if len(feedback) > 0 {
		fb = feedback[0]
	}
	cardBytes, err := marshalWithFeedback(card, fb)
	if err != nil {
		return nil, err
	}
	body := TemplateCardMsg{
		MsgType:      MsgTypeTemplateCard,
		TemplateCard: cardBytes,
	}
	return c.Reply(ctx, frame, body)
}

type UpdateCardOptions struct {
	UserIDs []string
}

func (c *WSClient) UpdateTemplateCard(ctx context.Context, frame *Frame, card any, userids ...string) (*Frame, error) {
	if frame.ReqID() == "" {
		return nil, fmt.Errorf("aibot: update template card requires the event callback frame req_id")
	}
	cardBytes, err := marshalWithFeedback(card, nil)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"response_type": ResponseTypeUpdateTemplateCard,
		"template_card": json.RawMessage(cardBytes),
	}
	if len(userids) > 0 {
		body["userids"] = userids
	}
	return c.Reply(ctx, frame, body, CmdRespondUpdate)
}

func (c *WSClient) SendMessage(ctx context.Context, chatid string, body any, chatType ...int) (*Frame, error) {
	if chatid == "" {
		return nil, fmt.Errorf("aibot: chatid is required")
	}
	ct := ChatTypeAuto
	if len(chatType) > 0 {
		ct = chatType[0]
	}
	bodyBytes, err := mergeSendBody(chatid, ct, body)
	if err != nil {
		return nil, err
	}
	reqID := GenerateReqID()
	resp, err := c.request(ctx, reqID, &Frame{
		Cmd:     CmdSendMsg,
		Headers: Headers{ReqID: reqID},
		Body:    bodyBytes,
	})
	if err != nil {
		return nil, err
	}
	if aerr := resp.AsError(); aerr != nil {
		return resp, aerr
	}
	return resp, nil
}

func marshalWithFeedback(card any, fb *Feedback) (json.RawMessage, error) {
	if fb == nil {
		b, err := json.Marshal(card)
		if err != nil {
			return nil, fmt.Errorf("aibot: marshal template card: %w", err)
		}
		return b, nil
	}
	var m map[string]any
	switch v := card.(type) {
	case map[string]any:
		m = make(map[string]any, len(v)+1)
		for k, vv := range v {
			m[k] = vv
		}
	case json.RawMessage:
		if err := json.Unmarshal(v, &m); err != nil {
			return nil, fmt.Errorf("aibot: template card must be a json object: %w", err)
		}
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("aibot: marshal template card: %w", err)
		}
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, fmt.Errorf("aibot: template card must be a json object: %w", err)
		}
	}
	m["feedback"] = fb
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("aibot: marshal template card: %w", err)
	}
	return b, nil
}

func mergeSendBody(chatid string, chatType int, body any) (json.RawMessage, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("aibot: marshal message body: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, ErrInvalidMessageBody
	}
	m["chatid"] = chatid
	if chatType != ChatTypeAuto {
		m["chat_type"] = chatType
	}
	out, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("aibot: marshal message body: %w", err)
	}
	return out, nil
}
