package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type EdgeOutput struct {
	base string
	http *http.Client
}

func NewEdgeOutput(base string, httpClient *http.Client) *EdgeOutput {
	return &EdgeOutput{base: base, http: httpClient}
}
func (c *EdgeOutput) Call(ctx context.Context, path string, input any, result any) error {
	var body io.Reader
	if input != nil {
		raw, _ := json.Marshal(input)
		body = bytes.NewReader(raw)
	}
	method := "POST"
	if input == nil {
		method = "GET"
	}
	url := strings.Replace(c.base, "localhost", "127.0.0.1", 1) + path
	r, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(r)
	if err != nil {
		return errors.New("EDGE_OUTPUT_UNAVAILABLE")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return errors.New("EDGE_OUTPUT_NOT_CONFIRMED")
	}
	return json.NewDecoder(io.LimitReader(res.Body, 16384)).Decode(result)
}
