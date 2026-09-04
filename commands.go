package aibot

const (
	CmdSubscribe      = "aibot_subscribe"
	CmdPing           = "ping"
	CmdRespondMsg     = "aibot_respond_msg"
	CmdRespondWelcome = "aibot_respond_welcome_msg"
	CmdRespondUpdate  = "aibot_respond_update_msg"
	CmdSendMsg        = "aibot_send_msg"
	CmdUploadInit     = "aibot_upload_media_init"
	CmdUploadChunk    = "aibot_upload_media_chunk"
	CmdUploadFinish   = "aibot_upload_media_finish"
	CmdMsgCallback    = "aibot_msg_callback"
	CmdEventCallback  = "aibot_event_callback"
)

const (
	MsgTypeText         = "text"
	MsgTypeImage        = "image"
	MsgTypeMixed        = "mixed"
	MsgTypeVoice        = "voice"
	MsgTypeFile         = "file"
	MsgTypeVideo        = "video"
	MsgTypeStream       = "stream"
	MsgTypeMarkdown     = "markdown"
	MsgTypeTemplateCard = "template_card"
	MsgTypeEvent        = "event"
)

const (
	EventTypeEnterChat    = "enter_chat"
	EventTypeTemplateCard = "template_card_event"
	EventTypeFeedback     = "feedback_event"
	EventTypeDisconnected = "disconnected_event"
)

const (
	ChatTypeAuto   = 0
	ChatTypeSingle = 1
	ChatTypeGroup  = 2
)

const (
	ChatSingle = "single"
	ChatGroup  = "group"
)

const (
	MediaTypeFile  = "file"
	MediaTypeImage = "image"
	MediaTypeVoice = "voice"
	MediaTypeVideo = "video"
)

const ResponseTypeUpdateTemplateCard = "update_template_card"

const (
	EvConnected     = "connected"
	EvAuthenticated = "authenticated"
	EvDisconnected  = "disconnected"
	EvReconnecting  = "reconnecting"
	EvError         = "error"
	EvMessage       = "message"
	EvEvent         = "event"
)
