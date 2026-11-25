# services/document/internal/client/process_owner.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/112-process-owner.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`ProcessOwner`、`processFacts`、`NewProcessOwner`、`Verify`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package client

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type ProcessOwner struct{ root string }

type processFacts struct {
	State   string `json:"state"`
	Command string `json:"commandLine"`
	Args    []string
}

func NewProcessOwner(root string) *ProcessOwner { return &ProcessOwner{root: root} }

// Verify只接受指定PID、绝对入口和run-owner的实际证据；查询超时不等于旧进程已退出。
func (c *ProcessOwner) Verify(ctx context.Context, pid uint64, role, owner string) (string, error) {
	if pid == 0 || pid > 2147483647 || owner == "" || (role != "document" && role != "edge") {
		return "", errors.New("OUTPUT_EXIT_PROOF_UNAVAILABLE")
	}
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	facts, err := c.probeProcess(probeCtx, pid)
	// 先读取再取消，避免主动 cancel 把真实的探测超时原因覆盖掉；失败仍不能当退出证明。
	probeError := probeCtx.Err()
	cancel()
	if err != nil || probeError != nil {
		if probeError == context.DeadlineExceeded {
			return "", errors.New("OUTPUT_EXIT_PROOF_DEADLINE")
		}
		return "", errors.New("OUTPUT_EXIT_PROOF_UNAVAILABLE")
	}
	entry := filepath.Join(c.root, ".runtime/bin/document.exe")
	if runtime.GOOS == "linux" {
		entry = filepath.Join(c.root, ".runtime/bin/document")
	}
	if role == "edge" {
		entry = filepath.Join(c.root, "scripts/edge.mjs")
	}
	matched := strings.Contains(strings.ToLower(facts.Command), strings.ToLower(entry)) && strings.Contains(facts.Command, "--run-owner "+owner)
	if runtime.GOOS == "linux" {
		// procfs给出独立argv，必须精确匹配入口与参数值，owner前缀不能代表原实例。
		entryFound, ownerFound := false, false
		for i, arg := range facts.Args {
			entryFound = entryFound || arg == entry
			if arg == "--run-owner" && i+1 < len(facts.Args) && facts.Args[i+1] == owner {
				ownerFound = true
			}
		}
		matched = entryFound && ownerFound
	}
	if facts.State == "PRESENT" && matched {
		return "", errors.New("OLD_OUTPUT_PROCESS_STILL_RUNNING")
	}
	if facts.State != "PRESENT" && facts.State != "ABSENT" {
		return "", errors.New("OUTPUT_EXIT_PROOF_UNAVAILABLE")
	}

	return facts.State, nil
}
````

UTF-8/LF全文SHA-256：`c8e334eb93ee93fe4369c2ba2c719d81dae2a01e31eba4e0cf2030ca9eebd5a4`。[参考实现](../../../../services/document/internal/client/process_owner.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
