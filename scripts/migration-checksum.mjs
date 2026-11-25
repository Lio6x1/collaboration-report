import { createHash } from 'node:crypto';

export function validateMigration(bytes, manifest) {
  // 任何DDL之前核对原始字节；数据库自己的安装声明不能授权另一份SQL。
  const actual = createHash('sha256').update(bytes).digest('hex');
  if (!/^[a-f0-9]{64}$/.test(manifest.sha256 ?? '') || actual !== manifest.sha256)
    throw Error('MIGRATION_CHECKSUM_MISMATCH');
  return bytes;
}
