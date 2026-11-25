# services/document/internal/storage/local/guard_unix.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/127-guard-unix.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：按明确授权操作资源；不将内部存储路径当作公开下载链接。

主要声明：`lockFile`、`unlockFile`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package local

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func lockFile(file *os.File, exclusive bool) error {
	mode := unix.LOCK_SH
	if exclusive {
		mode = unix.LOCK_EX
	}
	err := unix.Flock(int(file.Fd()), mode|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return ErrBusy
	}
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

func unlockFile(file *os.File) {
	_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
}
````

UTF-8/LF全文SHA-256：`4b3e5e73b91dde23e03553e2bb330819941887fff7fcb6ade9dcc25440503b42`。[参考实现](../../../../services/document/internal/storage/local/guard_unix.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
