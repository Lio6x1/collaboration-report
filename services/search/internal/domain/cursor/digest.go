package cursor

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash"
	"regexp"
	"strconv"
)

const candidateDigestPurpose = "search-candidates-batch-digest-v1\x00"

var candidatePhysical = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,159}$`)

func digestField(digest hash.Hash, value []byte) {
	var length [2]byte
	binary.BigEndian.PutUint16(length[:], uint16(len(value)))
	digest.Write(length[:])
	digest.Write(value)
}

func validateDigestCandidates(before, after json.RawMessage, candidates []*pb.SearchCandidate, more bool) error {
	invalid := errors.New("SEARCH_CURSOR_DIGEST_INVALID")
	if len(candidates) > 100 || validateBatch(Batch{Chain: [32]byte{1}, Before: before, After: after,
		Count: uint8(len(candidates)), More: more, Digest: [32]byte{1}}) != nil {
		return invalid
	}
	seen := make(map[uint64]bool, len(candidates))
	physical := ""
	for _, candidate := range candidates {
		if candidate == nil || candidate.DocumentId == 0 || candidate.ReleaseId == 0 || candidate.AggregateSeq == 0 ||
			!candidatePhysical.MatchString(candidate.PhysicalIndex) || ValidateSort(candidate.SortJsonUtf8) != nil || seen[candidate.DocumentId] ||
			(physical != "" && physical != candidate.PhysicalIndex) {
			return invalid
		}
		var tuple []json.RawMessage
		var document string
		if json.Unmarshal(candidate.SortJsonUtf8, &tuple) != nil || json.Unmarshal(tuple[1], &document) != nil ||
			document != strconv.FormatUint(candidate.DocumentId, 10) {
			return invalid
		}
		physical = candidate.PhysicalIndex
		seen[candidate.DocumentId] = true
	}
	if len(candidates) != 0 && !bytes.Equal(after, candidates[len(candidates)-1].SortJsonUtf8) {
		return invalid
	}
	return nil
}

// DigestCandidates绑定完整有序候选与批边界，不依赖ES客户端，也不引入另一份RPC DTO。
// PIT/nonce/时间会在重放时改变，不能参与摘要；用途标签与长度字段防止跨用途或拼接歧义。
func DigestCandidates(before, after json.RawMessage, candidates []*pb.SearchCandidate, more bool) ([32]byte, error) {
	var result [32]byte
	if err := validateDigestCandidates(before, after, candidates, more); err != nil {
		return result, err
	}
	digest := sha256.New()
	digest.Write([]byte(candidateDigestPurpose))
	digestField(digest, before)
	digestField(digest, after)
	moreByte := byte(0)
	if more {
		moreByte = 1
	}
	digest.Write([]byte{byte(len(candidates)), moreByte})
	for _, candidate := range candidates {
		digestField(digest, []byte(candidate.PhysicalIndex))
		var identifiers [24]byte
		binary.BigEndian.PutUint64(identifiers[0:8], candidate.DocumentId)
		binary.BigEndian.PutUint64(identifiers[8:16], candidate.ReleaseId)
		binary.BigEndian.PutUint64(identifiers[16:24], candidate.AggregateSeq)
		digest.Write(identifiers[:])
		digestField(digest, candidate.SortJsonUtf8)
	}
	copy(result[:], digest.Sum(nil))
	return result, nil
}
