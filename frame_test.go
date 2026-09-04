package aibot

import (
	"encoding/json"
	"testing"
)

const msgCallbackJSON = `{"cmd":"aibot_msg_callback","headers":{"req_id":"REQUEST_ID"},"body":{"msgid":"MSGID","aibotid":"AIBOTID","chatid":"CHATID","chattype":"group","from":{"userid":"USERID"},"msgtype":"text","text":{"content":"@RobotA hello robot"}}}`

const imageCallbackJSON = `{"cmd":"aibot_msg_callback","headers":{"req_id":"REQUEST_ID"},"body":{"msgid":"MSGID","aibotid":"AIBOTID","chattype":"single","from":{"userid":"USERID"},"msgtype":"image","image":{"url":"https://example.com/aes.bin","aeskey":"AESKEY"}}}`

const eventCallbackJSON = `{"cmd":"aibot_event_callback","headers":{"req_id":"REQUEST_ID"},"body":{"msgid":"MSGID","create_time":1700000000,"aibotid":"AIBOTID","from":{"userid":"USERID"},"msgtype":"event","event":{"eventtype":"enter_chat","extra":"data"}}}`

const subscribeRespJSON = `{"headers":{"req_id":"REQUEST_ID"},"errcode":0,"errmsg":"ok"}`

const uploadFinishRespJSON = `{"headers":{"req_id":"REQUEST_ID"},"body":{"type":"file","media_id":"1G6nrLmr5EC3MMb_-zK1dDdzmd0p7cNliYu9V5w7o8K0","created_at":"1380000000"},"errcode":0,"errmsg":"ok"}`

func TestUnmarshalMsgCallback(t *testing.T) {
	var f Frame
	if err := json.Unmarshal([]byte(msgCallbackJSON), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if f.Cmd != CmdMsgCallback || f.ReqID() != "REQUEST_ID" {
		t.Fatalf("unexpected frame: %+v", f)
	}
	var body MsgBody
	if err := f.ParseBody(&body); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if body.MsgID != "MSGID" || body.AibotID != "AIBOTID" || body.ChatID != "CHATID" {
		t.Fatalf("unexpected body: %+v", body)
	}
	if body.ChatType != ChatGroup || body.From.UserID != "USERID" || body.MsgType != MsgTypeText {
		t.Fatalf("unexpected body: %+v", body)
	}
	if body.Text == nil || body.Text.Content != "@RobotA hello robot" {
		t.Fatalf("unexpected text: %+v", body.Text)
	}
}

func TestUnmarshalImageCallback(t *testing.T) {
	var f Frame
	if err := json.Unmarshal([]byte(imageCallbackJSON), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var body MsgBody
	if err := f.ParseBody(&body); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if body.Image == nil || body.Image.URL != "https://example.com/aes.bin" || body.Image.AESKey != "AESKEY" {
		t.Fatalf("unexpected image: %+v", body.Image)
	}
}

func TestUnmarshalEventCallback(t *testing.T) {
	var f Frame
	if err := json.Unmarshal([]byte(eventCallbackJSON), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var body EventBody
	if err := f.ParseBody(&body); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if body.MsgType != MsgTypeEvent || body.CreateTime != 1700000000 {
		t.Fatalf("unexpected body: %+v", body)
	}
	if body.Event.EventType != EventTypeEnterChat {
		t.Fatalf("unexpected eventtype: %s", body.Event.EventType)
	}
	if len(body.Event.Raw) == 0 {
		t.Fatal("expected raw event content")
	}
	var extra map[string]any
	if err := json.Unmarshal(body.Event.Raw, &extra); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if extra["extra"] != "data" {
		t.Fatalf("unexpected raw content: %v", extra)
	}
}

func TestUnmarshalSubscribeResponse(t *testing.T) {
	var f Frame
	if err := json.Unmarshal([]byte(subscribeRespJSON), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !f.OK() || f.AsError() != nil {
		t.Fatalf("expected ok frame: %+v", f)
	}
}

func TestMarshalStreamMsgKeepsFinishFalse(t *testing.T) {
	data, err := json.Marshal(StreamMsg{
		MsgType: MsgTypeStream,
		Stream:  StreamBody{ID: "S1", Finish: false, Content: "partial"},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	stream, _ := m["stream"].(map[string]any)
	if v, ok := stream["finish"].(bool); !ok || v {
		t.Fatalf("finish=false must be serialized: %s", data)
	}
}

func TestUnmarshalUploadFinishResp(t *testing.T) {
	var f Frame
	if err := json.Unmarshal([]byte(uploadFinishRespJSON), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var body UploadFinishRespBody
	if err := f.ParseBody(&body); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if body.Type != MediaTypeFile || body.MediaID != "1G6nrLmr5EC3MMb_-zK1dDdzmd0p7cNliYu9V5w7o8K0" {
		t.Fatalf("unexpected body: %+v", body)
	}
	if body.CreatedAt != 1380000000 {
		t.Fatalf("created_at = %d, want 1380000000", body.CreatedAt)
	}
}

func TestUnmarshalUploadFinishRespNumericCreatedAt(t *testing.T) {
	var f Frame
	raw := `{"headers":{"req_id":"R"},"body":{"type":"image","media_id":"M1","created_at":1380000000},"errcode":0,"errmsg":"ok"}`
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var body UploadFinishRespBody
	if err := f.ParseBody(&body); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if body.CreatedAt != 1380000000 {
		t.Fatalf("created_at = %d, want 1380000000", body.CreatedAt)
	}
}

func TestMarshalWithFeedback(t *testing.T) {
	out, err := marshalWithFeedback(map[string]any{"card_type": "text_notice"}, &Feedback{ID: "F1"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["card_type"] != "text_notice" {
		t.Fatalf("unexpected card: %v", m)
	}
	fb, _ := m["feedback"].(map[string]any)
	if fb == nil || fb["id"] != "F1" {
		t.Fatalf("unexpected feedback: %v", m["feedback"])
	}
	type card struct {
		CardType string `json:"card_type"`
	}
	out, err = marshalWithFeedback(card{CardType: "button_interaction"}, &Feedback{ID: "F2"})
	if err != nil {
		t.Fatalf("marshal struct: %v", err)
	}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	fb, _ = m["feedback"].(map[string]any)
	if m["card_type"] != "button_interaction" || fb == nil || fb["id"] != "F2" {
		t.Fatalf("unexpected merged card: %v", m)
	}
	out, err = marshalWithFeedback(map[string]any{"card_type": "text_notice"}, nil)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	m2 := map[string]any{}
	if err := json.Unmarshal(out, &m2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m2["feedback"]; ok {
		t.Fatal("feedback should be omitted when nil")
	}
}

func TestMergeSendBody(t *testing.T) {
	out, err := mergeSendBody("CHAT1", ChatTypeSingle, map[string]any{
		"msgtype":  MsgTypeMarkdown,
		"markdown": map[string]any{"content": "hi"},
	})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["chatid"] != "CHAT1" || m["chat_type"] != float64(1) || m["msgtype"] != MsgTypeMarkdown {
		t.Fatalf("unexpected merged body: %v", m)
	}
	out, err = mergeSendBody("CHAT1", ChatTypeAuto, map[string]any{"msgtype": MsgTypeMarkdown})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	m2 := map[string]any{}
	if err := json.Unmarshal(out, &m2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m2["chat_type"]; ok {
		t.Fatal("chat_type should be omitted when auto")
	}
	if _, err := mergeSendBody("CHAT1", ChatTypeAuto, "not-an-object"); !errorsIs(err, ErrInvalidMessageBody) {
		t.Fatalf("expected ErrInvalidMessageBody, got %v", err)
	}
}

func errorsIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
