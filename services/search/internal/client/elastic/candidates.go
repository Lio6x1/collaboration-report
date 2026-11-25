package elastic

import (
	pb "collaboration/contracts/internalv1"
	"collaboration/search/internal/domain/cursor"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const candidateResponseLimit = 24 << 20
const pitHandleLimit = 64 << 10

// CandidateBatch仅携带最小候选；PIT不进入HTTP游标。即使响应不完整，只要已读到
// 合法的新PIT，错误返回仍保留它作为关闭线索，不能由调用者继续旧句柄。
type CandidateBatch struct {
	PIT        string
	Candidates []*pb.SearchCandidate
	End        json.RawMessage
	More       bool
}

type pitShards struct {
	Total      *int `json:"total"`
	Successful *int `json:"successful"`
	Skipped    *int `json:"skipped"`
	Failed     *int `json:"failed"`
}

func (s *pitShards) complete() bool {
	// ES的successful包含skipped，Open/refresh可省略后者；不能把skipped再相加。
	// total/successful/failed仍必须同时存在，避免缺summary被零值误判成完整成功。
	return s != nil && s.Total != nil && s.Successful != nil && s.Failed != nil &&
		*s.Total > 0 && *s.Successful == *s.Total && *s.Failed == 0 &&
		(s.Skipped == nil || (*s.Skipped >= 0 && *s.Skipped <= *s.Successful))
}

func validPIT(pit string) bool {
	return pit != "" && len(pit) <= pitHandleLimit && utf8.ValidString(pit) && strings.IndexFunc(pit, func(r rune) bool { return r <= ' ' || r == 127 }) < 0
}

func pitRemaining(remaining time.Duration) (string, error) {
	// 向下取整，绝不为了凑成1ms而把原绝对到期向后推。业务层每次从原expires计算remaining。
	if remaining < time.Millisecond {
		return "", errors.New("SEARCH_CURSOR_EXPIRED")
	}
	return strconv.FormatInt(remaining.Milliseconds(), 10) + "ms", nil
}

func (c *Client) OpenPIT(ctx context.Context, physical string, remaining time.Duration) (string, error) {
	if !physicalName.MatchString(physical) {
		return "", errors.New("SEARCH_PHYSICAL_INDEX_INVALID")
	}
	keepAlive, err := pitRemaining(remaining)
	if err != nil {
		return "", err
	}
	requestCtx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	status, data, err := c.request(requestCtx, "POST", "/"+physical+"/_pit?keep_alive="+url.QueryEscape(keepAlive)+"&allow_partial_search_results=false", nil, candidateResponseLimit)
	if err != nil {
		return "", err
	}
	var response struct {
		ID     string     `json:"id"`
		Shards *pitShards `json:"_shards"`
	}
	if status != 200 || json.Unmarshal(data, &response) != nil || !validPIT(response.ID) {
		return "", errors.New("SEARCH_ES_PIT_OPEN_UNKNOWN")
	}
	if !response.Shards.complete() || requestCtx.Err() != nil {
		return response.ID, errors.New("SEARCH_ES_PIT_OPEN_INCOMPLETE")
	}
	return response.ID, nil
}

type candidateHit struct {
	Physical string `json:"_index"`
	ID       string `json:"_id"`
	Source   struct {
		Document string `json:"document_id"`
		Release  string `json:"release_id"`
		Sequence string `json:"sequence"`
	} `json:"_source"`
	Sort json.RawMessage `json:"sort"`
}

func candidateUnsigned(value string) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 || strconv.FormatUint(parsed, 10) != value {
		return 0, errors.New("SEARCH_ES_CANDIDATE_BINDING_INVALID")
	}
	return parsed, nil
}

func (hit candidateHit) candidate() (*pb.SearchCandidate, error) {
	if !physicalName.MatchString(hit.Physical) || hit.ID != hit.Source.Document || cursor.ValidateSort(hit.Sort) != nil {
		return nil, errors.New("SEARCH_ES_CANDIDATE_BINDING_INVALID")
	}
	var sort []json.RawMessage
	var sortDocument string
	if json.Unmarshal(hit.Sort, &sort) != nil || json.Unmarshal(sort[1], &sortDocument) != nil || sortDocument != hit.Source.Document {
		return nil, errors.New("SEARCH_ES_CANDIDATE_BINDING_INVALID")
	}
	document, err := candidateUnsigned(hit.Source.Document)
	if err != nil {
		return nil, err
	}
	release, err := candidateUnsigned(hit.Source.Release)
	if err != nil {
		return nil, err
	}
	sequence, err := candidateUnsigned(hit.Source.Sequence)
	if err != nil {
		return nil, err
	}
	return &pb.SearchCandidate{DocumentId: document, ReleaseId: release, AggregateSeq: sequence,
		PhysicalIndex: hit.Physical, SortJsonUtf8: append([]byte(nil), hit.Sort...)}, nil
}

func (c *Client) Candidates(ctx context.Context, pit, query string, after json.RawMessage, size int, remaining time.Duration) (CandidateBatch, error) {
	var result CandidateBatch
	if !validPIT(pit) || !utf8.ValidString(query) || strings.TrimSpace(query) == "" || utf8.RuneCountInString(query) > 500 ||
		size < 1 || size > 100 || (len(after) != 0 && cursor.ValidateSort(after) != nil) {
		return result, errors.New("SEARCH_ES_CANDIDATE_REQUEST_INVALID")
	}
	keepAlive, err := pitRemaining(remaining)
	if err != nil {
		return result, err
	}
	request := map[string]any{
		"size": size + 1, "track_total_hits": false,
		"_source": []string{"document_id", "release_id", "sequence"},
		"pit":     map[string]any{"id": pit, "keep_alive": keepAlive},
		"sort":    []any{map[string]string{"_score": "desc"}, map[string]string{"document_id": "asc"}, map[string]string{"_shard_doc": "asc"}},
		"query": map[string]any{"bool": map[string]any{
			"filter": []any{map[string]any{"term": map[string]any{"visible": true}}},
			// 中文标准分词会拆出单字；默认OR会把“设定”误匹配到只有“固定”的正文。
			// 要求同一字段中连续的词序，复用原索引，不以标题去重掩盖独立文档。
			"must": []any{map[string]any{"multi_match": map[string]any{
				"query": query, "type": "phrase", "slop": 0, "fields": []string{"title", "text", "chapters.title"},
			}}},
		}},
	}
	if len(after) != 0 {
		// RawMessage只保留位置的原JSON词法，不能经float64重新编码高位shard序号。
		request["search_after"] = after
	}
	requestCtx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	status, data, err := c.request(requestCtx, "POST", "/_search?allow_partial_search_results=false", request, candidateResponseLimit)
	if err != nil {
		return result, err
	}
	var response struct {
		PIT        string     `json:"pit_id"`
		TimedOut   *bool      `json:"timed_out"`
		Terminated bool       `json:"terminated_early"`
		Shards     *pitShards `json:"_shards"`
		Hits       *struct {
			Hits json.RawMessage `json:"hits"`
		} `json:"hits"`
	}
	if status != 200 || json.Unmarshal(data, &response) != nil || !validPIT(response.PIT) {
		return result, errors.New("SEARCH_ES_CANDIDATES_INCOMPLETE")
	}
	result.PIT = response.PIT
	if response.TimedOut == nil || *response.TimedOut || response.Terminated || !response.Shards.complete() ||
		response.Hits == nil || requestCtx.Err() != nil {
		return result, errors.New("SEARCH_ES_CANDIDATES_INCOMPLETE")
	}
	var hits []candidateHit
	if json.Unmarshal(response.Hits.Hits, &hits) != nil || hits == nil || len(hits) > size+1 {
		return result, errors.New("SEARCH_ES_CANDIDATES_INCOMPLETE")
	}
	// 先验证整个响应，包括第B+1项，之后才能交付批次；损坏probe不能伪造成has_more。
	candidates := make([]*pb.SearchCandidate, 0, len(hits))
	seen := make(map[uint64]bool, len(hits))
	physical := ""
	for _, hit := range hits {
		candidate, err := hit.candidate()
		if err != nil || seen[candidate.GetDocumentId()] || (physical != "" && physical != hit.Physical) {
			return result, errors.New("SEARCH_ES_CANDIDATE_BINDING_INVALID")
		}
		physical = hit.Physical
		seen[candidate.DocumentId] = true
		candidates = append(candidates, candidate)
	}
	if requestCtx.Err() != nil {
		return result, errors.New("SEARCH_ES_CANDIDATES_INCOMPLETE")
	}
	result.More = len(candidates) > size
	if result.More {
		candidates = candidates[:size]
	}
	result.Candidates = candidates
	result.End = append(json.RawMessage(nil), after...)
	if len(candidates) != 0 {
		result.End = append(json.RawMessage(nil), candidates[len(candidates)-1].SortJsonUtf8...)
	}
	return result, nil
}

func (c *Client) ClosePIT(ctx context.Context, pit string) error {
	if !validPIT(pit) {
		return errors.New("SEARCH_ES_PIT_INVALID")
	}
	status, data, err := c.request(ctx, "DELETE", "/_pit", map[string]any{"id": pit}, candidateResponseLimit)
	if err != nil {
		return err
	}
	var response struct {
		Succeeded bool `json:"succeeded"`
		Freed     *int `json:"num_freed"`
	}
	// num_freed=0也可能是原PIT已自然释放；仅ES明确succeeded才确认该句柄已关闭。
	if status != 200 || json.Unmarshal(data, &response) != nil || !response.Succeeded || response.Freed == nil || *response.Freed < 0 || ctx.Err() != nil {
		return errors.New("SEARCH_ES_PIT_CLOSE_UNKNOWN")
	}
	return nil
}
