# packages/platform/schema_readiness.go

阶段01：[功能讲义](../01-foundation.md) · [分步编写](../write/028-schema-readiness.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`matchesSchemaMigrations`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package platform

import "encoding/hex"

// 期望集合来自受校验的发布配置；数据库不能自行声明一个新摘要就让服务认可。
// 001仍独立比较，附加迁移既不能缺失，也不能跳过版本或忽略未知记录。
func matchesSchemaMigrations(baseline string, additional, actual map[int]string) bool {
	if len(actual) != 1+len(additional) {
		return false
	}
	for version := 1; version <= 1+len(additional); version++ {
		expected := baseline
		if version > 1 {
			expected = additional[version]
		}
		decoded, err := hex.DecodeString(expected)
		if err != nil || len(decoded) != 32 || actual[version] != expected {
			return false
		}
	}
	return true
}
````

UTF-8/LF全文SHA-256：`cdda880353e658f756305a9f64d53bb5613abb6523d3b1c1e9ba03cda0a3aaf9`。[参考实现](../../../../packages/platform/schema_readiness.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
