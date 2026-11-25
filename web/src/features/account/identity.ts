// 账号与组织共用编号的展示/输入规则；权限、缓存键和接口始终使用原始十进制ID。
// 至少八位而不是截取八位，避免未来长编号或uint64身份被截断成另一个目标。
export function publicNumber(id: string): string {
  return id.padStart(8, '0');
}

export function idFromPublicNumber(input: string): string | null {
  const value = input.trim();
  if (!/^[0-9]{1,20}$/.test(value)) return null;
  const id = value.replace(/^0+/, '');
  // 不经过Number：uint64可能超过JS安全整数；补零只在用户输入边界去除。
  return id && BigInt(id) <= 18446744073709551615n ? id : null;
}

// 保留账号领域的调用名称，组织直接复用通用规则，不复制一套补零与uint64校验。
export const accountNumber = publicNumber;
export const accountIDFromInput = idFromPublicNumber;

export function matchesAccount(
  member: { user_id: string; display_name: string },
  input: string,
): boolean {
  const query = input.trim().toLocaleLowerCase();
  return (
    member.display_name.toLocaleLowerCase().includes(query) ||
    member.user_id.includes(query) ||
    accountNumber(member.user_id).includes(query)
  );
}
