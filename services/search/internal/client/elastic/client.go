package elastic

import (
	"bytes"
	"collaboration/platform"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Client struct {
	endpoint string
	owner    string
	http     *http.Client
}

var physicalName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,159}$`)
var ownerName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

func New(configuration *platform.SearchConfig) (*Client, error) {
	if configuration == nil || !ownerName.MatchString(configuration.Owner) {
		return nil, errors.New("SEARCH_ES_CONFIG_INVALID")
	}
	endpoint, err := url.Parse(configuration.ElasticEndpoint)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" || endpoint.User != nil ||
		(endpoint.Path != "" && endpoint.Path != "/") || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, errors.New("SEARCH_ES_CONFIG_INVALID")
	}
	// 新后台索引请求有独立有限预算，用户HTTP/RPC原期限不变；Ready调用仍受一秒父context限制。
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = 4
	return &Client{endpoint: strings.TrimSuffix(configuration.ElasticEndpoint, "/"), owner: configuration.Owner,
		http: &http.Client{Timeout: 5 * time.Second, Transport: transport}}, nil
}

func (c *Client) Close() { c.http.CloseIdleConnections() }

func (c *Client) request(ctx context.Context, method, path string, value any, maximum int64) (int, []byte, error) {
	var body bytes.Buffer
	if value != nil {
		encoder := json.NewEncoder(&body)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(value); err != nil || body.Len() > 64<<20 {
			return 0, nil, errors.New("SEARCH_ES_REQUEST_LIMIT")
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, &body)
	if err != nil {
		return 0, nil, errors.New("SEARCH_ES_REQUEST_INVALID")
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return 0, nil, errors.New("SEARCH_ES_UNAVAILABLE")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maximum+1))
	if err != nil || int64(len(data)) > maximum {
		return response.StatusCode, nil, errors.New("SEARCH_ES_RESPONSE_INCOMPLETE")
	}
	return response.StatusCode, data, nil
}

func (c *Client) Check(ctx context.Context) error {
	status, data, err := c.request(ctx, "GET", "/_cluster/health?wait_for_status=yellow&timeout=500ms", nil, 1<<20)
	var health struct {
		Status   string `json:"status"`
		TimedOut bool   `json:"timed_out"`
	}
	if err != nil || status != 200 || json.Unmarshal(data, &health) != nil || health.TimedOut ||
		(health.Status != "yellow" && health.Status != "green") {
		return errors.New("SEARCH_ES_NOT_READY")
	}
	return nil
}
