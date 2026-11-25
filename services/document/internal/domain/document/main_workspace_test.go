package document

import (
	"math"
	"testing"
)

func mainWorkspaceSample() MainWorkspaceFact {
	return MainWorkspaceFact{Manifest: 5, Structure: 7, Style: 11, MetadataControl: 4,
		Product: "MAIN_LOCK", Parent: true, Fixed: true, Succeeded: true, Installed: true, OriginalMatched: true,
		RecordedManifest: 5, RecordedStructure: 7, RecordedStyle: 11, Control: 2}
}

// 元数据control可以领先物理MAIN；来源必须仍是当前清单的原实际完成安装，不能猜成当前control。
func TestMainWorkspaceUsesOriginalCompletedManifest(t *testing.T) {
	for _, product := range []string{"MAIN_LOCK", "STYLE_CHANGE", "CHAPTER_CREATE", "STRUCTURE_CHANGE", "MERGE"} {
		fact := mainWorkspaceSample()
		fact.Product = product
		if epoch, err := ResolveMainWorkspace(fact); err != nil || epoch != 2 {
			t.Fatal("original physical MAIN epoch was replaced by metadata revision")
		}
	}
	fact := mainWorkspaceSample()
	fact.Control, fact.MetadataControl = math.MaxUint64, math.MaxUint64
	if epoch, err := ResolveMainWorkspace(fact); err != nil || epoch != math.MaxUint64 {
		t.Fatal("physical epoch lost uint64 precision")
	}
}

func TestMainWorkspaceInitialEpochIsExplicit(t *testing.T) {
	fact := MainWorkspaceFact{Manifest: 5, Structure: 7, Style: 11, MetadataControl: 9, Product: "INITIAL"}
	if epoch, err := ResolveMainWorkspace(fact); err != nil || epoch != 1 {
		t.Fatal("INITIAL must retain its actual initial epoch")
	}
	for _, corrupt := range []func(*MainWorkspaceFact){
		func(f *MainWorkspaceFact) { f.Parent = true },
		func(f *MainWorkspaceFact) { f.Fixed = true },
		func(f *MainWorkspaceFact) { f.Control = 2 },
	} {
		wrong := fact
		corrupt(&wrong)
		if _, err := ResolveMainWorkspace(wrong); err == nil {
			t.Fatal("initial epoch cannot cover a conflicting installed fact")
		}
	}
}

func TestMainWorkspaceRejectsIncompleteOrDifferentOriginalFacts(t *testing.T) {
	for _, corrupt := range []func(*MainWorkspaceFact){
		func(f *MainWorkspaceFact) { f.Manifest = 0 },
		func(f *MainWorkspaceFact) { f.MetadataControl = 0 },
		func(f *MainWorkspaceFact) { f.Product = "RELEASE" },
		func(f *MainWorkspaceFact) { f.Product = "UNKNOWN" },
		func(f *MainWorkspaceFact) { f.Parent = false },
		func(f *MainWorkspaceFact) { f.Fixed = false },
		func(f *MainWorkspaceFact) { f.Succeeded = false },
		func(f *MainWorkspaceFact) { f.Installed = false },
		func(f *MainWorkspaceFact) { f.OriginalMatched = false },
		func(f *MainWorkspaceFact) { f.RecordedManifest++ },
		func(f *MainWorkspaceFact) { f.RecordedStructure++ },
		func(f *MainWorkspaceFact) { f.RecordedStyle++ },
		func(f *MainWorkspaceFact) { f.Control = 0 },
		func(f *MainWorkspaceFact) { f.Control = f.MetadataControl + 1 },
	} {
		wrong := mainWorkspaceSample()
		corrupt(&wrong)
		if _, err := ResolveMainWorkspace(wrong); err == nil {
			t.Fatal("unknown physical MAIN must not fall back to metadata or epoch one")
		}
	}
}
