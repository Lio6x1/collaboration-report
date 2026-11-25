// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package elastic

import (
	"context"
	"encoding/json"
	"errors"
)

// alias.go：保留原实现，按业务合并文件。

// ResolveAlias拒绝多目标、额外filter/routing和未知owner/schema。
// 后台消费仍直接写解析得到的物理索引，不能让一次alias变化改写在途请求的目标。
func (c *Client) ResolveAlias(ctx context.Context, alias string) (string, bool, error) {
	if !physicalName.MatchString(alias) {
		return "", false, errors.New("SEARCH_ALIAS_INVALID")
	}
	status, data, err := c.request(ctx, "GET", "/_alias/"+alias, nil, 1<<20)
	if err != nil {
		return "", false, err
	}
	if status == 404 {
		return "", false, nil
	}
	var bindings map[string]struct {
		Aliases map[string]map[string]json.RawMessage `json:"aliases"`
	}
	if status != 200 || json.Unmarshal(data, &bindings) != nil || len(bindings) != 1 {
		return "", false, errors.New("SEARCH_ALIAS_UNBOUND")
	}
	for physical, binding := range bindings {
		metadata, exists := binding.Aliases[alias]
		var writable bool
		if !exists || len(binding.Aliases) != 1 || len(metadata) != 1 || json.Unmarshal(metadata["is_write_index"], &writable) != nil || !writable || !physicalName.MatchString(physical) {
			return "", false, errors.New("SEARCH_ALIAS_UNBOUND")
		}
		owned, err := c.verifyMapping(ctx, physical)
		if err != nil {
			return "", false, err
		}
		if !owned {
			return "", false, errors.New("SEARCH_ALIAS_UNBOUND")
		}
		return physical, true, nil
	}
	return "", false, errors.New("SEARCH_ALIAS_UNBOUND")
}

// EnsureInitialAlias只创建配置中明确指定的初代绑定。重启时可核验同一绑定，
// 不能因SQL缺行就覆盖已指向其他索引的alias，也不能冒称ES和SQL原子提交。
func (c *Client) EnsureInitialAlias(ctx context.Context, alias, physical string) error {
	if !physicalName.MatchString(alias) || !physicalName.MatchString(physical) || alias == physical {
		return errors.New("SEARCH_ALIAS_INVALID")
	}
	actual, exists, err := c.ResolveAlias(ctx, alias)
	if err != nil {
		return err
	}
	if exists {
		if actual != physical {
			return errors.New("SEARCH_INITIAL_ALIAS_CONFLICT")
		}
		return nil
	}
	if err := c.EnsureIndex(ctx, physical); err != nil {
		return err
	}
	status, data, err := c.request(ctx, "POST", "/_aliases?master_timeout=1s&timeout=1s", map[string]any{
		"actions": []any{map[string]any{"add": map[string]any{"index": physical, "alias": alias, "is_write_index": true}}},
	}, 1<<20)
	if err != nil {
		return err
	}
	var response struct {
		Acknowledged bool `json:"acknowledged"`
		Errors       bool `json:"errors"`
	}
	if status != 200 || json.Unmarshal(data, &response) != nil || !response.Acknowledged || response.Errors {
		return errors.New("SEARCH_ALIAS_CREATE_UNKNOWN")
	}
	actual, exists, err = c.ResolveAlias(ctx, alias)
	if err != nil {
		return err
	}
	if !exists || actual != physical {
		return errors.New("SEARCH_ALIAS_CREATE_UNKNOWN")
	}
	return nil
}

// alias_switch.go：保留原实现，按业务合并文件。

// AliasSwitchState仅描述ES事实；AtTarget不代表SQL代次和原job已经提交。
type AliasSwitchState string

const (
	AliasAtSource AliasSwitchState = "SOURCE"
	AliasAtTarget AliasSwitchState = "TARGET"
)

// InspectAliasSwitch供持久SWITCHING恢复使用。调用方须核验原job的封存验证事实，
// 再按SOURCE重试原切换或按TARGET完成原SQL CAS，不能另选目标或解除屏障。
func (c *Client) InspectAliasSwitch(ctx context.Context, alias, source, target string) (AliasSwitchState, error) {
	if !physicalName.MatchString(alias) || !physicalName.MatchString(source) || !physicalName.MatchString(target) ||
		alias == source || alias == target || source == target {
		return "", errors.New("SEARCH_ALIAS_INVALID")
	}
	actual, exists, err := c.ResolveAlias(ctx, alias)
	if err != nil {
		return "", err
	}
	if !exists || (actual != source && actual != target) {
		return "", errors.New("SEARCH_ALIAS_SWITCH_CONFLICT")
	}
	// 即使当前alias仍在source，也必须确认目标是已经构建的本owner索引；
	// 恢复检查不创建缺失索引，避免把空索引当作已验证目标。
	owned, err := c.verifyMapping(ctx, target)
	if err != nil {
		return "", err
	}
	if !owned {
		return "", errors.New("SEARCH_ALIAS_SWITCH_CONFLICT")
	}
	if actual == target {
		return AliasAtTarget, nil
	}
	return AliasAtSource, nil
}

// SwitchAlias只执行一次精确切换。调用方必须已持久化SWITCHING屏障及完整验证。
// ES与SQL不构成分布式事务：请求/回执未知直接返回错误，留给原job重读恢复。
func (c *Client) SwitchAlias(ctx context.Context, alias, source, target string) error {
	state, err := c.InspectAliasSwitch(ctx, alias, source, target)
	if err != nil {
		return err
	}
	if state != AliasAtSource {
		return errors.New("SEARCH_ALIAS_SWITCH_RECONCILE_REQUIRED")
	}
	status, data, err := c.request(ctx, "POST", "/_aliases?master_timeout=1s&timeout=1s", map[string]any{
		"actions": []any{
			map[string]any{"remove": map[string]any{"index": source, "alias": alias, "must_exist": true}},
			map[string]any{"add": map[string]any{"index": target, "alias": alias, "is_write_index": true}},
		},
	}, 1<<20)
	if err != nil {
		return err
	}
	if status != 200 || !validSwitchAcknowledgement(data, alias, source, target) {
		return errors.New("SEARCH_ALIAS_SWITCH_UNKNOWN")
	}
	actual, exists, err := c.ResolveAlias(ctx, alias)
	if err != nil {
		return err
	}
	if !exists || actual != target {
		return errors.New("SEARCH_ALIAS_SWITCH_UNKNOWN")
	}
	return nil
}

func validSwitchAcknowledgement(data []byte, alias, source, target string) bool {
	var reply struct {
		Acknowledged *bool           `json:"acknowledged"`
		Errors       bool            `json:"errors"`
		Error        json.RawMessage `json:"error"`
		Results      json.RawMessage `json:"action_results"`
	}
	if json.Unmarshal(data, &reply) != nil || reply.Acknowledged == nil || !*reply.Acknowledged || reply.Errors || len(reply.Error) != 0 {
		return false
	}
	// ES成功时允许省略action_results；一旦返回逐动作结果就必须完整对应两个原动作，
	// 不能只看HTTP 200或顶层errors=false而忽略部分失败或错误目标。
	if len(reply.Results) == 0 {
		return true
	}
	var results []struct {
		Action struct {
			Type    string   `json:"type"`
			Indices []string `json:"indices"`
			Aliases []string `json:"aliases"`
		} `json:"action"`
		Status int             `json:"status"`
		Error  json.RawMessage `json:"error"`
	}
	if json.Unmarshal(reply.Results, &results) != nil || len(results) != 2 {
		return false
	}
	for i, result := range results {
		kind, physical := "remove", source
		if i == 1 {
			kind, physical = "add", target
		}
		if result.Status != 200 || len(result.Error) != 0 || result.Action.Type != kind ||
			len(result.Action.Indices) != 1 || result.Action.Indices[0] != physical ||
			len(result.Action.Aliases) != 1 || result.Action.Aliases[0] != alias {
			return false
		}
	}
	return true
}
