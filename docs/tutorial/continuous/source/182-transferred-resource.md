# services/document/internal/repository/document/transferred_resource.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/182-transferred-resource.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`LockedDocumentResource`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"database/sql"
)

// 可写正文依赖先持 Space/quota，再锁不可变资源；后续 canonical 版本 INSERT 的
// 同事务计费不会从资源反向新取 quota。资格必须精确到当前 Doc/Space 的已接受转让。
func LockedDocumentResource(ctx context.Context, tx *sql.Tx, resource, document, space uint64) (string, error) {
	if err := LockDocumentCapacity(ctx, tx, document); err != nil {
		return "", err
	}
	_, state, err := LockedResourceSpace(ctx, tx, resource)
	if err != nil {
		return state, err
	}
	allowed, err := resources.ResourceAllowedForDocument(ctx, tx, resource, document, space)
	if err != nil {
		return state, err
	}
	if !allowed {
		return state, domain.Violation(domain.StateConflict, "RESOURCE_SCOPE_CHANGED")
	}
	return state, nil
}
````

UTF-8/LF全文SHA-256：`830c430ccd23325fc2398fd45bbd30fca865de8af6a685e0c2529c7866e5f543`。[参考实现](../../../../services/document/internal/repository/document/transferred_resource.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
