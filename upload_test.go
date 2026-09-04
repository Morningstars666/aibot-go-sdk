package aibot

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/coder/websocket"
)

func TestValidateUpload(t *testing.T) {
	if err := validateUpload("unknown", "a.bin", 100, 1); !errors.Is(err, ErrInvalidMediaType) {
		t.Fatalf("expected ErrInvalidMediaType, got %v", err)
	}
	if err := validateUpload(MediaTypeFile, "", 100, 1); !errors.Is(err, ErrInvalidFilename) {
		t.Fatalf("expected ErrInvalidFilename, got %v", err)
	}
	if err := validateUpload(MediaTypeFile, "a.bin", 0, 0); !errors.Is(err, ErrEmptyData) {
		t.Fatalf("expected ErrEmptyData, got %v", err)
	}
	if err := validateUpload(MediaTypeFile, "a.bin", 3, 1); err == nil {
		t.Fatal("expected error for size below minimum")
	}
	if err := validateUpload(MediaTypeImage, "a.png", 10<<20+1, 21); !errors.Is(err, ErrDataTooLarge) {
		t.Fatalf("expected ErrDataTooLarge, got %v", err)
	}
	if err := validateUpload(MediaTypeVoice, "a.amr", 2<<20, 4); err != nil {
		t.Fatalf("voice at limit should pass: %v", err)
	}
	if err := validateUpload(MediaTypeFile, "a.bin", 20<<20, 40); err != nil {
		t.Fatalf("file at limit should pass: %v", err)
	}
}

func TestSplitChunks(t *testing.T) {
	data := make([]byte, 1200)
	chunks := splitChunks(data, 512)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 512 || len(chunks[1]) != 512 || len(chunks[2]) != 176 {
		t.Fatalf("unexpected chunk sizes: %d %d %d", len(chunks[0]), len(chunks[1]), len(chunks[2]))
	}
	if splitChunks(nil, 512) != nil {
		t.Fatal("empty data should produce no chunks")
	}
}

func TestUploadMediaFlow(t *testing.T) {
	var chunkBodies []UploadChunkBody
	var initBody UploadInitBody
	finishCh := make(chan struct{}, 1)

	srv := newWSServer(t, func(ctx context.Context, conn *websocket.Conn) {
		for {
			f, err := readFrame(ctx, t, conn)
			if err != nil {
				return
			}
			switch f["cmd"] {
			case CmdSubscribe:
				if err := writeFrame(ctx, t, conn, ackFrame(frameReqID(f))); err != nil {
					return
				}
			case CmdPing:
				if err := writeFrame(ctx, t, conn, ackFrame(frameReqID(f))); err != nil {
					return
				}
			case CmdUploadInit:
				raw, _ := json.Marshal(f["body"])
				if err := json.Unmarshal(raw, &initBody); err != nil {
					t.Errorf("parse init body: %v", err)
				}
				if err := writeFrame(ctx, t, conn, ackBody(frameReqID(f), map[string]any{"upload_id": "UP1"})); err != nil {
					return
				}
			case CmdUploadChunk:
				raw, _ := json.Marshal(f["body"])
				var cb UploadChunkBody
				if err := json.Unmarshal(raw, &cb); err != nil {
					t.Errorf("parse chunk body: %v", err)
				}
				chunkBodies = append(chunkBodies, cb)
				if err := writeFrame(ctx, t, conn, ackFrame(frameReqID(f))); err != nil {
					return
				}
			case CmdUploadFinish:
				if err := writeFrame(ctx, t, conn, ackBody(frameReqID(f), map[string]any{
					"type":       "file",
					"media_id":   "MEDIA1",
					"created_at": "1380000000",
				})); err != nil {
					return
				}
				finishCh <- struct{}{}
			}
		}
	})
	c := NewWSClient(testOptions(srv.URL))
	defer c.Close()
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	data := make([]byte, 1200*1024)
	for i := range data {
		data[i] = byte(i % 251)
	}
	result, err := c.UploadMedia(context.Background(), MediaTypeFile, "test.bin", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("upload media: %v", err)
	}
	if result.MediaID != "MEDIA1" || result.Type != MediaTypeFile || result.CreatedAt != 1380000000 {
		t.Fatalf("unexpected result: %+v", result)
	}
	select {
	case <-finishCh:
	default:
		t.Fatal("finish not acknowledged")
	}
	if initBody.Filename != "test.bin" || initBody.TotalSize != 1200*1024 || initBody.TotalChunks != 3 || initBody.Type != MediaTypeFile {
		t.Fatalf("unexpected init body: %+v", initBody)
	}
	if len(chunkBodies) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunkBodies))
	}
	var rebuilt []byte
	for i, cb := range chunkBodies {
		if cb.UploadID != "UP1" {
			t.Fatalf("chunk %d upload_id = %s", i, cb.UploadID)
		}
		if cb.ChunkIndex != i {
			t.Fatalf("chunk index = %d, want %d", cb.ChunkIndex, i)
		}
		decoded, err := base64.StdEncoding.DecodeString(cb.Base64Data)
		if err != nil {
			t.Fatalf("decode chunk %d: %v", i, err)
		}
		rebuilt = append(rebuilt, decoded...)
	}
	if !bytes.Equal(rebuilt, data) {
		t.Fatal("rebuilt data mismatch")
	}
}

func TestUploadMediaValidationErrors(t *testing.T) {
	srv := newWSServer(t, func(ctx context.Context, conn *websocket.Conn) {
		serveAckLoop(ctx, t, conn, nil)
	})
	c := NewWSClient(testOptions(srv.URL))
	defer c.Close()
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := c.UploadMedia(context.Background(), "exe", "a.exe", bytes.NewReader(make([]byte, 100))); !errors.Is(err, ErrInvalidMediaType) {
		t.Fatalf("expected ErrInvalidMediaType, got %v", err)
	}
	if _, err := c.UploadMedia(context.Background(), MediaTypeImage, "a.png", bytes.NewReader([]byte("hi"))); err == nil {
		t.Fatal("expected error for size below minimum")
	}
	if _, err := c.UploadMedia(context.Background(), MediaTypeVoice, "a.amr", bytes.NewReader(make([]byte, 2<<20+1))); !errors.Is(err, ErrDataTooLarge) {
		t.Fatalf("expected ErrDataTooLarge, got %v", err)
	}
}
