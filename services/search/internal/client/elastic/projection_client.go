// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package elastic

import (
	index "collaboration/search/internal/domain/index"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"unicode/utf8"
)

// application.go：保留原实现，按业务合并文件。

const applyScript = `
if (ctx.op == 'create' || !ctx._source.containsKey('sequence_key')) {
  ctx._source = params.document;
} else {
  int order = params.document.sequence_key.compareTo(ctx._source.sequence_key);
  if (order > 0) {
    ctx._source = params.document;
  } else if (order == 0 && params.document.authority_digest != ctx._source.authority_digest) {
    throw new IllegalArgumentException('SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT');
  } else {
    ctx.op = 'noop';
  }
}`

// Apply以确定Doc ID原子替换投影。成功或noop后重读实际ES水位，
// SQL不能把这次请求的旧水位冒充已经存在的更高水位。
func (c *Client) Apply(ctx context.Context, physical string, value index.Authority) (index.State, error) {
	var result index.State
	if !physicalName.MatchString(physical) {
		return result, errors.New("SEARCH_PHYSICAL_INDEX_INVALID")
	}
	projection, err := project(value)
	if err != nil {
		return result, err
	}
	status, data, err := c.request(ctx, "POST", "/"+physical+"/_update/"+strconv.FormatUint(value.Document, 10)+"?retry_on_conflict=3", map[string]any{
		"scripted_upsert": true, "upsert": map[string]any{}, "script": map[string]any{"lang": "painless", "source": applyScript,
			"params": map[string]any{"document": projection}},
	}, 1<<20)
	if err != nil {
		return result, err
	}
	if status != 200 && status != 201 {
		if sameSequenceConflict(data) {
			return result, errors.New("SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT")
		}
		return result, errors.New("SEARCH_ES_APPLY_FAILED")
	}
	stored, err := c.getProjection(ctx, physical, value.Document, true)
	if err != nil {
		return result, err
	}
	result, err = stored.state(value.Document)
	if err != nil {
		return result, err
	}
	if result.Sequence < value.Sequence || (result.Sequence == value.Sequence && result.Digest != value.Digest) {
		return result, errors.New("SEARCH_ES_APPLICATION_UNKNOWN")
	}
	return result, nil
}

func sameSequenceConflict(raw []byte) bool {
	var response struct {
		Error map[string]any `json:"error"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return false
	}
	for node, count := response.Error, 0; node != nil && count < 8; count++ {
		if node["type"] == "illegal_argument_exception" && node["reason"] == "SEARCH_SAME_SEQUENCE_AUTHORITY_CONFLICT" {
			return true
		}
		node, _ = node["caused_by"].(map[string]any)
	}
	return false
}

func (c *Client) getProjection(ctx context.Context, physical string, document uint64, metadataOnly bool) (projection, error) {
	var result projection
	if !physicalName.MatchString(physical) || document == 0 {
		return result, errors.New("SEARCH_INDEX_BINDING_INVALID")
	}
	path := "/" + physical + "/_doc/" + strconv.FormatUint(document, 10)
	maximum := int64(64 << 20)
	if metadataOnly {
		path += "?_source_includes=document_id,sequence,sequence_key,release_id,authority_digest,visible,visibility_policy"
		maximum = 1 << 20
	}
	status, data, err := c.request(ctx, "GET", path, nil, maximum)
	if err != nil {
		return result, err
	}
	var response struct {
		Found  bool       `json:"found"`
		Source projection `json:"_source"`
	}
	if status != 200 || json.Unmarshal(data, &response) != nil || !response.Found {
		return result, errors.New("SEARCH_ES_APPLICATION_UNKNOWN")
	}
	return response.Source, nil
}

func (c *Client) Get(ctx context.Context, physical string, document uint64) (index.Authority, error) {
	var result index.Authority
	stored, err := c.getProjection(ctx, physical, document, false)
	if err != nil {
		return result, err
	}
	state, err := stored.state(document)
	if err != nil {
		return result, err
	}
	result = index.Authority{Document: document, Sequence: state.Sequence, Release: state.Release, Digest: state.Digest,
		Visible: state.Visible, Policy: state.Policy, Title: stored.Title, Text: stored.Text, Chapters: []index.Chapter{}}
	for _, chapter := range stored.Chapters {
		unit, err := strconv.ParseUint(chapter.Unit, 10, 64)
		if err != nil || unit == 0 || strconv.FormatUint(unit, 10) != chapter.Unit {
			return result, errors.New("SEARCH_ES_CHAPTER_INVALID")
		}
		result.Chapters = append(result.Chapters, index.Chapter{Unit: unit, Title: chapter.Title, Text: chapter.Text})
	}
	if _, err = project(result); err != nil {
		return result, err
	}
	if !result.Visible && (result.Title != "" || result.Text != "" || len(result.Chapters) != 0) {
		return result, errors.New("SEARCH_ES_TOMBSTONE_INVALID")
	}
	return result, nil
}

// projection.go：保留原实现，按业务合并文件。

type wireChapter struct {
	Unit  string `json:"unit_id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}
type projection struct {
	Document  string        `json:"document_id"`
	Sequence  string        `json:"sequence"`
	Watermark string        `json:"sequence_key"`
	Release   *string       `json:"release_id"`
	Digest    string        `json:"authority_digest"`
	Visible   bool          `json:"visible"`
	Policy    string        `json:"visibility_policy"`
	Title     string        `json:"title"`
	Text      string        `json:"text"`
	Chapters  []wireChapter `json:"chapters"`
}

func project(value index.Authority) (projection, error) {
	var result projection
	if value.Document == 0 || !utf8.ValidString(value.Title) || !utf8.ValidString(value.Text) ||
		len(value.Title) > 1024 || len(value.Text) > 24<<20 || len(value.Chapters) > 100000 {
		return result, errors.New("SEARCH_INDEX_PROJECTION_INVALID")
	}
	if _, err := index.ShouldApply(index.State{}, value); err != nil {
		return result, err
	}
	result = projection{Document: strconv.FormatUint(value.Document, 10), Sequence: strconv.FormatUint(value.Sequence, 10),
		Watermark: index.Watermark(value.Sequence), Digest: hex.EncodeToString(value.Digest[:]), Visible: value.Visible, Policy: value.Policy,
		Chapters: []wireChapter{}}
	if value.Visible {
		if value.Sequence == 0 {
			return result, errors.New("SEARCH_INDEX_PROJECTION_INVALID")
		}
		release := strconv.FormatUint(value.Release, 10)
		result.Release, result.Title, result.Text = &release, value.Title, value.Text
		seen := make(map[uint64]bool)
		bytes := len(value.Text)
		for _, chapter := range value.Chapters {
			bytes += len(chapter.Title) + len(chapter.Text)
			if chapter.Unit == 0 || seen[chapter.Unit] || !utf8.ValidString(chapter.Title) || !utf8.ValidString(chapter.Text) || bytes > 48<<20 {
				return result, errors.New("SEARCH_INDEX_PROJECTION_LIMIT")
			}
			seen[chapter.Unit] = true
			result.Chapters = append(result.Chapters, wireChapter{Unit: strconv.FormatUint(chapter.Unit, 10), Title: chapter.Title, Text: chapter.Text})
		}
	}
	return result, nil
}

func (p projection) state(document uint64) (index.State, error) {
	var result index.State
	if p.Document != strconv.FormatUint(document, 10) {
		return result, errors.New("SEARCH_ES_DOCUMENT_BINDING_INVALID")
	}
	sequence, err := strconv.ParseUint(p.Sequence, 10, 64)
	if err != nil || strconv.FormatUint(sequence, 10) != p.Sequence || p.Watermark != index.Watermark(sequence) {
		return result, errors.New("SEARCH_ES_WATERMARK_INVALID")
	}
	digest, err := hex.DecodeString(p.Digest)
	if err != nil || len(digest) != 32 {
		return result, errors.New("SEARCH_ES_AUTHORITY_DIGEST_INVALID")
	}
	result.Sequence, result.Policy, result.Visible = sequence, p.Policy, p.Visible
	copy(result.Digest[:], digest)
	if p.Release != nil {
		result.Release, err = strconv.ParseUint(*p.Release, 10, 64)
		if err != nil || result.Release == 0 || strconv.FormatUint(result.Release, 10) != *p.Release {
			return result, errors.New("SEARCH_ES_RELEASE_INVALID")
		}
	}
	if _, err = index.ShouldApply(index.State{}, index.Authority{Sequence: result.Sequence, Release: result.Release, Visible: result.Visible, Policy: result.Policy, Digest: result.Digest}); err != nil {
		return result, err
	}
	return result, nil
}
