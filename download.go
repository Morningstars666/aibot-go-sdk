package aibot

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
)

const maxDownloadSize int64 = 1 << 27

func (c *WSClient) DownloadFile(ctx context.Context, rawURL, aesKey string) ([]byte, error) {
	if rawURL == "" {
		return nil, ErrEmptyURL
	}
	hctx, hcancel := context.WithTimeout(ctx, c.opts.HTTPTimeout)
	defer hcancel()
	req, err := http.NewRequestWithContext(hctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("aibot: build download request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("aibot: download file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("aibot: download file: http status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadSize))
	if err != nil {
		return nil, fmt.Errorf("aibot: read download body: %w", err)
	}
	if aesKey != "" {
		return DecryptAES(data, aesKey)
	}
	return data, nil
}

func ExtractFilenameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	for _, key := range []string{"filename", "file_name", "fname", "name"} {
		if v := u.Query().Get(key); v != "" {
			return v
		}
	}
	base := path.Base(u.Path)
	if base == "." || base == "/" {
		return ""
	}
	return base
}
