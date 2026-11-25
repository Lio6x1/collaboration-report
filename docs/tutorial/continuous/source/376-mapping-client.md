# services/search/internal/client/elastic/mapping_client.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/376-mapping-client.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`properties`、`verifyMapping`、`EnsureIndex`、`DeleteOwnedIndex`、`RefreshCount`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package elastic

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
)

// schema.go：保留原实现，按业务合并文件。

const schemaVersion = "publication-index-v1-uint64"

func properties() map[string]any {
	keyword := func() any { return map[string]any{"type": "keyword"} }
	text := func() any { return map[string]any{"type": "text"} }
	return map[string]any{
		"document_id": keyword(), "sequence": keyword(), "sequence_key": keyword(), "release_id": keyword(),
		"authority_digest": keyword(), "visible": map[string]any{"type": "boolean"}, "visibility_policy": keyword(),
		// 查询使用完整父文档text，章节信息供定位；object不引入nested每文档对象数的额外低上限。
		"title": text(), "text": text(), "chapters": map[string]any{"properties": map[string]any{
			"unit_id": keyword(), "title": text(), "text": text(),
		}},
	}
}

func (c *Client) verifyMapping(ctx context.Context, physical string) (bool, error) {
	status, data, err := c.request(ctx, "GET", "/"+physical+"/_mapping", nil, 1<<20)
	if err != nil {
		return false, err
	}
	if status == 404 {
		return false, nil
	}
	if status != 200 {
		return false, errors.New("SEARCH_ES_MAPPING_UNAVAILABLE")
	}
	var response map[string]struct {
		Mappings struct {
			Dynamic string `json:"dynamic"`
			Meta    struct {
				Application string `json:"application"`
				Owner       string `json:"owner"`
				Schema      string `json:"schema"`
			} `json:"_meta"`
			Properties map[string]any `json:"properties"`
		} `json:"mappings"`
	}
	if json.Unmarshal(data, &response) != nil || len(response) != 1 {
		return false, errors.New("SEARCH_INDEX_SCHEMA_MISMATCH")
	}
	actual, exists := response[physical]
	if !exists || actual.Mappings.Dynamic != "strict" || actual.Mappings.Meta.Application != "collaboration-report" ||
		actual.Mappings.Meta.Owner != c.owner || actual.Mappings.Meta.Schema != schemaVersion || !reflect.DeepEqual(actual.Mappings.Properties, properties()) {
		return false, errors.New("SEARCH_INDEX_SCHEMA_MISMATCH")
	}
	return true, nil
}

// EnsureIndex不接管同名未知索引；超时后只能重读原名称/owner/schema，不能删除重建。
func (c *Client) EnsureIndex(ctx context.Context, physical string) error {
	if !physicalName.MatchString(physical) {
		return errors.New("SEARCH_PHYSICAL_INDEX_INVALID")
	}
	exists, err := c.verifyMapping(ctx, physical)
	if err != nil || exists {
		return err
	}
	status, _, err := c.request(ctx, "PUT", "/"+physical, map[string]any{
		"settings": map[string]any{"number_of_shards": 1, "number_of_replicas": 0},
		"mappings": map[string]any{"dynamic": "strict", "_meta": map[string]any{
			"application": "collaboration-report", "owner": c.owner, "schema": schemaVersion,
		}, "properties": properties()},
	}, 1<<20)
	if err != nil {
		return err
	}
	if status != 200 && status != 400 {
		return errors.New("SEARCH_INDEX_CREATE_UNKNOWN")
	}
	exists, err = c.verifyMapping(ctx, physical)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("SEARCH_INDEX_CREATE_UNKNOWN")
	}
	return nil
}

func (c *Client) DeleteOwnedIndex(ctx context.Context, physical string) error {
	if !physicalName.MatchString(physical) {
		return errors.New("SEARCH_PHYSICAL_INDEX_INVALID")
	}
	exists, err := c.verifyMapping(ctx, physical)
	if err != nil || !exists {
		return err
	}
	status, data, err := c.request(ctx, "GET", "/"+physical+"/_alias", nil, 1<<20)
	var aliases map[string]struct {
		Aliases map[string]any `json:"aliases"`
	}
	if err != nil || status != 200 || json.Unmarshal(data, &aliases) != nil || len(aliases) != 1 || len(aliases[physical].Aliases) != 0 {
		return errors.New("SEARCH_INDEX_STILL_ALIASED")
	}
	status, _, err = c.request(ctx, "DELETE", "/"+physical, nil, 1<<20)
	if err != nil {
		return err
	}
	if status != 200 {
		return errors.New("SEARCH_INDEX_DELETE_UNKNOWN")
	}
	return nil
}

// validation.go：保留原实现，按业务合并文件。

// RefreshCount用于写屏障后的完整目标核验；部分分片成功不能作为完整统计。
func (c *Client) RefreshCount(ctx context.Context, physical string) (uint64, error) {
	if !physicalName.MatchString(physical) {
		return 0, errors.New("SEARCH_PHYSICAL_INDEX_INVALID")
	}
	var count uint64
	for _, method := range []string{"POST", "GET"} {
		path := "/" + physical + "/_refresh"
		if method == "GET" {
			path = "/" + physical + "/_count"
		}
		status, raw, err := c.request(ctx, method, path, nil, 1<<20)
		if err != nil {
			return 0, err
		}
		var reply struct {
			Count  *uint64                                  `json:"count"`
			Shards struct{ Total, Successful, Failed *int } `json:"_shards"`
		}
		if status != 200 || json.Unmarshal(raw, &reply) != nil || reply.Shards.Total == nil || reply.Shards.Successful == nil || reply.Shards.Failed == nil || *reply.Shards.Total <= 0 || *reply.Shards.Failed != 0 || *reply.Shards.Successful != *reply.Shards.Total {
			return 0, errors.New("SEARCH_REBUILD_SHARDS_INCOMPLETE")
		}
		if method == "GET" {
			if reply.Count == nil {
				return 0, errors.New("SEARCH_REBUILD_COUNT_INVALID")
			}
			count = *reply.Count
		}
	}
	return count, nil
}
````

UTF-8/LF全文SHA-256：`a4aedeb75fc55a42c3b34c129cb0768f816da8f4c6bee73581d0078089685ffa`。[参考实现](../../../../services/search/internal/client/elastic/mapping_client.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
