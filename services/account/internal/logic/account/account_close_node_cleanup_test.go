package account

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// 复用集中注销场景的原冻结计划，避免为Node清理重复建立整份跨服务身份图。
func verifyActualNodeAccountCleanup(t *testing.T, ctx context.Context, operation string, user uint64, closing, surviving, private closeDocumentFixture) {
	t.Helper()
	if os.Getenv("COLLAB_TEST_NODE_CLEANUP") != "1" {
		return
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(os.Getenv("COLLAB_TEST_ACCOUNT_CONFIG")), "../.."))
	text := func(id uint64) string { return strconv.FormatUint(id, 10) }
	input, err := json.Marshal(map[string]string{
		"operation": operation, "user": text(user), "document": text(closing.id), "unit": text(closing.unit),
		"surviving": text(surviving.id), "survivingUnit": text(surviving.unit),
		"resource": text(closing.resource), "privateResource": text(closing.privateResource), "manifest": text(closing.manifest),
		"version": text(closing.version), "privateVersion": text(closing.privateVersion), "privateManifest": text(closing.privateManifest),
		"structure": text(closing.structure), "style": text(closing.style), "ancestorStructure": text(closing.ancestorStructure),
		"ancestorStyle": text(closing.ancestorStyle), "ancestorManifest": text(closing.ancestorManifest),
		"privateDocument": text(private.id), "erasedVersion": text(private.privateVersion),
	})
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, filepath.Join(root, ".runtime/toolchains/node-v24.19.0-win-x64/node.exe"), "--no-maglev", filepath.Join(root, "tests/lifecycle/account-document-cleanup.mjs"), string(input))
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("ACTUAL_NODE_CLEANUP_FAILED: %v\n%s", err, output)
	}
	t.Log(string(output))
}
