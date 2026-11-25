# services/search/internal/client/elastic/client.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/375-client.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`Client`、`New`、`Close`、`request`、`Check`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`78139b05ffaad78d699913dfec3fca968dbc3c300c11e435de0cab7a2f513644`。[参考实现](../../../../services/search/internal/client/elastic/client.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
