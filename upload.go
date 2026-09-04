package aibot

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

const (
	uploadChunkSize = 512 << 10
	maxUploadChunks = 100
	minUploadSize   = 5
	maxFilenameLen  = 256
)

var mediaSizeLimits = map[string]int64{
	MediaTypeImage: 10 << 20,
	MediaTypeVoice: 2 << 20,
	MediaTypeVideo: 10 << 20,
	MediaTypeFile:  20 << 20,
}

type UploadInitBody struct {
	Type        string `json:"type"`
	Filename    string `json:"filename"`
	TotalSize   int64  `json:"total_size"`
	TotalChunks int    `json:"total_chunks"`
	MD5         string `json:"md5,omitempty"`
}

type UploadChunkBody struct {
	UploadID   string `json:"upload_id"`
	ChunkIndex int    `json:"chunk_index"`
	Base64Data string `json:"base64_data"`
}

type UploadFinishBody struct {
	UploadID string `json:"upload_id"`
}

type UploadInitRespBody struct {
	UploadID string `json:"upload_id"`
}

type UploadFinishRespBody struct {
	Type      string
	MediaID   string
	CreatedAt int64
}

func (b *UploadFinishRespBody) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type      string          `json:"type"`
		MediaID   string          `json:"media_id"`
		CreatedAt json.RawMessage `json:"created_at"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	b.Type = raw.Type
	b.MediaID = raw.MediaID
	if len(raw.CreatedAt) > 0 && string(raw.CreatedAt) != "null" {
		if raw.CreatedAt[0] == '"' {
			var s string
			if err := json.Unmarshal(raw.CreatedAt, &s); err != nil {
				return err
			}
			var n int64
			if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
				return err
			}
			b.CreatedAt = n
		} else {
			if err := json.Unmarshal(raw.CreatedAt, &b.CreatedAt); err != nil {
				return err
			}
		}
	}
	return nil
}

type UploadMediaResult struct {
	Type      string
	MediaID   string
	CreatedAt int64
}

func (c *WSClient) UploadMedia(ctx context.Context, mediaType, filename string, r io.Reader) (*UploadMediaResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("aibot: read media data: %w", err)
	}
	chunks := splitChunks(data, uploadChunkSize)
	if err := validateUpload(mediaType, filename, int64(len(data)), len(chunks)); err != nil {
		return nil, err
	}
	sum := md5.Sum(data)
	initReqID := GenerateReqID()
	initBody, err := json.Marshal(UploadInitBody{
		Type:        mediaType,
		Filename:    filename,
		TotalSize:   int64(len(data)),
		TotalChunks: len(chunks),
		MD5:         hex.EncodeToString(sum[:]),
	})
	if err != nil {
		return nil, err
	}
	initResp, err := c.request(ctx, initReqID, &Frame{
		Cmd:     CmdUploadInit,
		Headers: Headers{ReqID: initReqID},
		Body:    initBody,
	})
	if err != nil {
		return nil, err
	}
	if aerr := initResp.AsError(); aerr != nil {
		return nil, aerr
	}
	var initBodyResp UploadInitRespBody
	if err := initResp.ParseBody(&initBodyResp); err != nil {
		return nil, fmt.Errorf("aibot: parse upload init response: %w", err)
	}
	if initBodyResp.UploadID == "" {
		return nil, fmt.Errorf("aibot: upload init response missing upload_id")
	}
	for i, chunk := range chunks {
		chunkReqID := GenerateReqID()
		chunkBody, err := json.Marshal(UploadChunkBody{
			UploadID:   initBodyResp.UploadID,
			ChunkIndex: i,
			Base64Data: base64.StdEncoding.EncodeToString(chunk),
		})
		if err != nil {
			return nil, err
		}
		chunkResp, err := c.request(ctx, chunkReqID, &Frame{
			Cmd:     CmdUploadChunk,
			Headers: Headers{ReqID: chunkReqID},
			Body:    chunkBody,
		})
		if err != nil {
			return nil, err
		}
		if aerr := chunkResp.AsError(); aerr != nil {
			return nil, aerr
		}
	}
	finishReqID := GenerateReqID()
	finishBody, err := json.Marshal(UploadFinishBody{UploadID: initBodyResp.UploadID})
	if err != nil {
		return nil, err
	}
	finishResp, err := c.request(ctx, finishReqID, &Frame{
		Cmd:     CmdUploadFinish,
		Headers: Headers{ReqID: finishReqID},
		Body:    finishBody,
	})
	if err != nil {
		return nil, err
	}
	if aerr := finishResp.AsError(); aerr != nil {
		return nil, aerr
	}
	var finishBodyResp UploadFinishRespBody
	if err := finishResp.ParseBody(&finishBodyResp); err != nil {
		return nil, fmt.Errorf("aibot: parse upload finish response: %w", err)
	}
	return &UploadMediaResult{
		Type:      finishBodyResp.Type,
		MediaID:   finishBodyResp.MediaID,
		CreatedAt: finishBodyResp.CreatedAt,
	}, nil
}

func validateUpload(mediaType, filename string, size int64, chunkCount int) error {
	limit, ok := mediaSizeLimits[mediaType]
	if !ok {
		return ErrInvalidMediaType
	}
	if filename == "" || len(filename) > maxFilenameLen {
		return ErrInvalidFilename
	}
	if size <= 0 {
		return ErrEmptyData
	}
	if size < minUploadSize {
		return fmt.Errorf("aibot: data size %d is below minimum %d bytes", size, minUploadSize)
	}
	if size > limit {
		return fmt.Errorf("%w: %d > %d for type %s", ErrDataTooLarge, size, limit, mediaType)
	}
	if chunkCount > maxUploadChunks {
		return fmt.Errorf("%w: %d > %d", ErrTooManyChunks, chunkCount, maxUploadChunks)
	}
	return nil
}

func splitChunks(data []byte, chunkSize int) [][]byte {
	if len(data) == 0 {
		return nil
	}
	var chunks [][]byte
	for off := 0; off < len(data); off += chunkSize {
		end := off + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunks = append(chunks, data[off:end])
	}
	return chunks
}
