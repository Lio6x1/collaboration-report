# services/document/internal/client/edge_output.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/110-edge-output.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`EdgeOutput`、`NewEdgeOutput`、`Call`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`0a84cccdefd5566fa32966fb879c112fcb4363dffcddc4ce34bd58fbff6e7255`。[参考实现](../../../../services/document/internal/client/edge_output.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
