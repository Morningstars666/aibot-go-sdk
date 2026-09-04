package aibot

import (
	"errors"
	"fmt"
)

var (
	ErrClosed             = errors.New("aibot: client closed")
	ErrNotConnected       = errors.New("aibot: not connected")
	ErrAlreadyConnected   = errors.New("aibot: already connected")
	ErrConnClosed         = errors.New("aibot: connection closed")
	ErrRequestTimeout     = errors.New("aibot: request timeout")
	ErrInvalidBotID       = errors.New("aibot: bot_id is required")
	ErrInvalidSecret      = errors.New("aibot: secret is required")
	ErrInvalidMediaType   = errors.New("aibot: invalid media type")
	ErrInvalidFilename    = errors.New("aibot: invalid filename")
	ErrEmptyData          = errors.New("aibot: empty data")
	ErrDataTooLarge       = errors.New("aibot: data too large")
	ErrTooManyChunks      = errors.New("aibot: too many chunks")
	ErrInvalidMessageBody = errors.New("aibot: message body must be a json object")
	ErrEmptyURL           = errors.New("aibot: url is empty")
	ErrDecrypt            = errors.New("aibot: decrypt failed")
	ErrInvalidKey         = errors.New("aibot: invalid aes key")
	ErrInvalidPadding     = errors.New("aibot: invalid pkcs7 padding")
)

type RespError struct {
	Errcode int
	Errmsg  string
}

func (e *RespError) Error() string {
	return fmt.Sprintf("aibot: server error errcode=%d errmsg=%s", e.Errcode, e.Errmsg)
}
