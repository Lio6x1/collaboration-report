package rebuildmeta

import (
	"bytes"
	"crypto/sha256"
	"testing"
)

func TestCanonicalSnapshotEntries(t *testing.T) {
	// 零水位/无发布与全uint64必须无损，封存摘要覆盖完整规范行字节。
	entries := []Entry{{Document: 1}, {Document: ^uint64(0), Minimum: ^uint64(0), Release: ^uint64(0)}}
	var content []byte
	for _, entry := range entries {
		line, err := entry.Line()
		if err != nil {
			t.Fatal(err)
		}
		content = append(content, line...)
	}
	var decoded []Entry
	count, digest, err := Scan(bytes.NewReader(content), func(entry Entry) error { decoded = append(decoded, entry); return nil })
	if err != nil || count != 2 || digest != sha256.Sum256(content) || len(decoded) != 2 || decoded[0] != entries[0] || decoded[1] != entries[1] {
		t.Fatal("CANONICAL_ENTRIES_NOT_PRESERVED")
	}
	for _, invalid := range [][]byte{
		content[:len(content)-1],
		append(bytes.Clone(content), content...),
		[]byte("{\"document_id\":\"1\",\"document_id\":\"1\",\"minimum_seq\":\"0\",\"release_hint\":null}\n"),
		[]byte("{\"document_id\":\"01\",\"minimum_seq\":\"0\",\"release_hint\":null}\n"),
	} {
		if count, digest, err := Scan(bytes.NewReader(invalid), nil); err == nil || count != 0 || digest != [32]byte{} {
			t.Fatal("INVALID_STREAM_BECAME_SEALED")
		}
	}
}
