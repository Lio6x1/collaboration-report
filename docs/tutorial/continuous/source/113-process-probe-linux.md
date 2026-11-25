# services/document/internal/client/process_probe_linux.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/113-process-probe-linux.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：适配跨服务或外部工具；保留期限、认证与错误语义。

主要声明：`probeProcess`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
//go:build linux

package client

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Linux发行版把应用放在同一个PID命名空间。另一个容器里看不到PID不能作为退出证据。
func (c *ProcessOwner) probeProcess(ctx context.Context, pid uint64) (processFacts, error) {
	if err := ctx.Err(); err != nil {
		return processFacts{}, err
	}
	// 先证明procfs可读；未挂载或无权限均是未知，绝不返回ABSENT。
	if _, err := os.ReadFile("/proc/self/cmdline"); err != nil {
		return processFacts{}, err
	}
	dir := filepath.Join("/proc", strconv.FormatUint(pid, 10))
	raw, err := os.ReadFile(filepath.Join(dir, "cmdline"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if _, statErr := os.Stat(dir); errors.Is(statErr, os.ErrNotExist) {
				return processFacts{State: "ABSENT"}, nil
			}
		}
		return processFacts{}, err
	}
	if len(raw) == 0 || raw[len(raw)-1] != 0 {
		// 僵尸/退出中的空命令行无法证明身份，等待真正回收后再重试。
		return processFacts{}, errors.New("PROCESS_ARGUMENTS_UNAVAILABLE")
	}
	return processFacts{State: "PRESENT", Args: strings.Split(string(raw[:len(raw)-1]), "\x00")}, nil
}
````

UTF-8/LF全文SHA-256：`ddd5c220ecbc038a281cc30b27ac06882341cddbb4dc4768035ffb4e0b6384a4`。[参考实现](../../../../services/document/internal/client/process_probe_linux.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
