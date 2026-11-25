# services/document/internal/client/process_probe_windows.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/115-process-probe-windows.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`probeProcess`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
//go:build windows

package client

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strconv"
)

// Windows继续通过既有只读PowerShell查询，保持进程启动标记和超时语义。
func (c *ProcessOwner) probeProcess(ctx context.Context, pid uint64) (processFacts, error) {
	probe := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-File", filepath.Join(c.root, "scripts/process-owner.ps1"), "-ProcessId", strconv.FormatUint(pid, 10))
	hideProbeWindow(probe)
	raw, err := probe.Output()
	if err != nil {
		return processFacts{}, err
	}
	var facts processFacts
	if json.Unmarshal(raw, &facts) != nil {
		return processFacts{}, errors.New("PROCESS_FACTS_INVALID")
	}
	return facts, nil
}
````

UTF-8/LF全文SHA-256：`ec31f0172fbb1f20a64a585c033abc248e4a781fe4dcd4fb3d451e36782b04e6`。[参考实现](../../../../services/document/internal/client/process_probe_windows.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
