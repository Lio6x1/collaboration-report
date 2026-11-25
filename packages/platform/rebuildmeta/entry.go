// Package rebuildmeta定义两服务交接的元数据行，不包含正文或访问凭据。
package rebuildmeta

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"strconv"
)

type Entry struct{ Document, Minimum, Release uint64 }

// Line使用规范十进制字符串保留全uint64；无正式指针用null，不用字符串"0"冒充引用。
func (e Entry) Line() ([]byte, error) {
	if e.Document == 0 {
		return nil, errors.New("REBUILD_ENTRY_INVALID")
	}
	release := "null"
	if e.Release != 0 {
		release = strconv.Quote(strconv.FormatUint(e.Release, 10))
	}
	return []byte(`{"document_id":"` + strconv.FormatUint(e.Document, 10) + `","minimum_seq":"` + strconv.FormatUint(e.Minimum, 10) + `","release_hint":` + release + "}\n"), nil
}

// Scan只保留一行，严格检查升序、规范原字节和完整LF尾部。回调可写入调用者自己的事务，
// 返回的摘要/总数只有整个流成功才有效，调用者不能在封存前提交部分导入。
func Scan(reader io.Reader, visit func(Entry) error) (uint64, [32]byte, error) {
	hash := sha256.New()
	input := bufio.NewReaderSize(io.TeeReader(reader, hash), 4096)
	var count, previous uint64
	fail := func() (uint64, [32]byte, error) { return 0, [32]byte{}, errors.New("REBUILD_ENTRIES_INVALID") }
	for {
		line, err := input.ReadSlice('\n')
		if err == io.EOF && len(line) == 0 {
			break
		}
		if err != nil {
			return fail()
		}
		var wire struct {
			Document string  `json:"document_id"`
			Minimum  string  `json:"minimum_seq"`
			Release  *string `json:"release_hint"`
		}
		if json.Unmarshal(line, &wire) != nil {
			return fail()
		}
		document, err := strconv.ParseUint(wire.Document, 10, 64)
		if err != nil || document <= previous {
			return fail()
		}
		minimum, err := strconv.ParseUint(wire.Minimum, 10, 64)
		if err != nil {
			return fail()
		}
		var release uint64
		if wire.Release != nil {
			release, err = strconv.ParseUint(*wire.Release, 10, 64)
			if err != nil || release == 0 {
				return fail()
			}
		}
		entry := Entry{document, minimum, release}
		canonical, err := entry.Line()
		// 原字节比较同时拒绝重复/未知字段、非规范数字、重排和缺字段。
		if err != nil || !bytes.Equal(canonical, line) || count == ^uint64(0) {
			return fail()
		}
		if visit != nil {
			if err := visit(entry); err != nil {
				return 0, [32]byte{}, err
			}
		}
		previous, count = document, count+1
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return count, digest, nil
}
