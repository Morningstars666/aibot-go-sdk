package aibot

import (
	"regexp"
	"testing"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func TestGenerateReqID(t *testing.T) {
	id := GenerateReqID()
	if !uuidRe.MatchString(id) {
		t.Fatalf("id %q does not match uuid format", id)
	}
	if len(id) > maxReqIDLen {
		t.Fatalf("id too long: %d", len(id))
	}
}

func TestGenerateReqIDPrefix(t *testing.T) {
	id := GenerateReqID("stream")
	if len(id) < 7 || id[:7] != "stream-" {
		t.Fatalf("expected stream- prefix, got %q", id)
	}
	if !uuidRe.MatchString(id[7:]) {
		t.Fatalf("suffix is not uuid: %q", id[7:])
	}
}

func TestGenerateReqIDUnique(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		id := GenerateReqID()
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate id: %s", id)
		}
		seen[id] = struct{}{}
	}
}
