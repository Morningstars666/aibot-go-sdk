package aibot

import (
	"crypto/rand"
	"fmt"
)

const maxReqIDLen = 256

func GenerateReqID(prefix ...string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	id := fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:16])
	if len(prefix) > 0 && prefix[0] != "" {
		id = prefix[0] + "-" + id
	}
	if len(id) > maxReqIDLen {
		id = id[:maxReqIDLen]
	}
	return id
}
