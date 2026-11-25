# services/document/internal/storage/local/guard_windows.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/129-guard-windows.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：按明确授权操作资源；不将内部存储路径当作公开下载链接。

主要声明：`lockFile`、`unlockFile`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package local

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func lockFile(file *os.File, exclusive bool) error {
	flags := uint32(windows.LOCKFILE_FAIL_IMMEDIATELY)
	if exclusive {
		flags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}
	err := windows.LockFileEx(windows.Handle(file.Fd()), flags, 0, 1, 0, &windows.Overlapped{})
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return ErrBusy
	}
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

func unlockFile(file *os.File) {
	_ = windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &windows.Overlapped{})
}
````

UTF-8/LF全文SHA-256：`3802f337fa9dc23026a9dd8b35f3ed7765b5965294fe01d1007d0c515f97dbc2`。[参考实现](../../../../services/document/internal/storage/local/guard_windows.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
