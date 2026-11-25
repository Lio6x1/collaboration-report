package rebuildmeta

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrepareAndSealBindExactOriginalFacts(t *testing.T) {
	p := Prepare{Version: 1, Operation: "operation-1", Owner: "owner", Alias: "reports", Source: "reports-g1", SourceGeneration: "1", SourceRevision: "1", Target: "reports-g2", TargetGeneration: "2", Cluster: "cluster", Topic: "events", TopicID: strings.Repeat("a", 32), Starts: [3]string{"0", "1", "9223372036854775807"}, Nonce: strings.Repeat("b", 64)}
	raw, err := p.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParsePrepare(raw)
	if err != nil || parsed != p {
		t.Fatal("PREPARE_FACTS_CHANGED")
	}
	for _, invalid := range [][]byte{
		bytes.Replace(raw, []byte(`"target_generation":"2"`), []byte(`"target_generation":"3"`), 1),
		bytes.Replace(raw, []byte(`"source_generation":"1"`), []byte(`"source_generation":"01"`), 1),
		append([]byte(`{"version":1,`), raw[1:]...),
		bytes.Replace(raw, []byte(`9223372036854775807`), []byte(`9223372036854775808`), 1),
	} {
		if _, err := ParsePrepare(invalid); err == nil {
			t.Fatal("INVALID_BINDING_ACCEPTED")
		}
	}
	s := Seal{Version: 1, PrepareRef: "rebuild-prepare-v1:" + strings.Repeat("c", 64), Count: "0", EntriesDigest: strings.Repeat("d", 64)}
	raw, err = s.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := ParseSeal(raw)
	if err != nil || sealed != s {
		t.Fatal("SEAL_FACTS_CHANGED")
	}
}
