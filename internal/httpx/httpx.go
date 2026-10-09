// Package httpx is the shared HTTP plumbing for provider adapters.
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	paynp "github.com/mukezhz/pay-np"
)

const maxBody = 1 << 20

func DefaultClient(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	return &http.Client{Timeout: 30 * time.Second}
}

type Request struct {
	Method  string
	URL     string
	Header  http.Header
	JSON    any
	Form    []byte
	Allow4x bool // return 4xx bodies to the caller instead of an APIError
	Close   bool // send Connection: close (servers that drop keep-alive sockets)
}

// Do sends req and returns the body; non-2xx (and 4xx unless Allow4x) becomes *paynp.APIError.
func Do(ctx context.Context, c *http.Client, p paynp.ProviderName, req Request) (int, []byte, error) {
	var body io.Reader
	header := req.Header.Clone()
	if header == nil {
		header = http.Header{}
	}
	switch {
	case req.JSON != nil:
		b, err := json.Marshal(req.JSON)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(b)
		header.Set("Content-Type", "application/json")
	case req.Form != nil:
		body = bytes.NewReader(req.Form)
		header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	hr, err := http.NewRequestWithContext(ctx, req.Method, req.URL, body)
	if err != nil {
		return 0, nil, err
	}
	hr.Header = header
	hr.Close = req.Close
	hr.Header.Set("Accept", "application/json")

	resp, err := c.Do(hr)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	ok := resp.StatusCode < 300 || (req.Allow4x && resp.StatusCode < 500)
	if !ok {
		return resp.StatusCode, b, &paynp.APIError{Provider: p, StatusCode: resp.StatusCode, Body: b}
	}
	return resp.StatusCode, b, nil
}
