import { isAbsolute } from 'node:path';
import { isIP } from 'node:net';

const localFile = (value) =>
  typeof value === 'string' &&
  isAbsolute(value) &&
  !value.startsWith('\\\\') &&
  !value.startsWith('//');

// 共用SMTP段的公开字段规则。密码不能混进settings或服务配置；此函数不读凭据内容。
export function smtpSettings(value) {
  const fail = () => {
    throw Error('SMTP_SETTINGS_INVALID');
  };
  if (!value || typeof value !== 'object' || Array.isArray(value)) fail();
  if (
    Object.keys(value).some(
      (key) => !['address', 'from', 'security', 'credentialsFile', 'caFile'].includes(key),
    )
  )
    fail();
  const address =
    typeof value.address === 'string' &&
    /^(?:\[([0-9a-fA-F:]+)\]|([A-Za-z0-9.-]+)):(\d{1,5})$/.exec(value.address);
  if (
    !address ||
    Number(address[3]) < 1 ||
    Number(address[3]) > 65535 ||
    typeof value.from !== 'string' ||
    value.from.length > 254 ||
    !/^[^\s<>()[\]\\",;:@]+@[A-Za-z0-9.-]+$/.test(value.from)
  )
    fail();
  const host = address[1] ?? address[2];
  const security = value.security ?? 'development';
  if (security === 'development') {
    if (
      !((isIP(host) === 4 && host.split('.')[0] === '127') || host === '::1') ||
      value.credentialsFile ||
      value.caFile
    )
      fail();
  } else if (security === 'tls' || security === 'starttls') {
    if (!localFile(value.credentialsFile) || (value.caFile && !localFile(value.caFile))) fail();
  } else fail();
  return {
    address: value.address,
    from: value.from,
    security,
    ...(security === 'development' ? {} : { credentialsFile: value.credentialsFile }),
    ...(value.caFile ? { caFile: value.caFile } : {}),
  };
}

// 重跑基础/登录准备时优先保留已绑定的SMTP整段。不能把真实投递改回开发收件器，
// 也不能替用户修掉有错的旧配置而改变投递通道；旧段非法时显式失败等待修正。
export function retainedSMTP(existing, fallback) {
  if (existing !== undefined && existing !== null) {
    smtpSettings(existing);
    return existing;
  }
  return smtpSettings(fallback);
}

export function withSMTPSettings(configuration, settings) {
  if (configuration?.role !== 'account') throw Error('SMTP_SERVICE_BINDING_INVALID');
  // 只替换smtp字段，DSN、证书、端口、owner等部署身份均来自原文件。
  return { ...configuration, smtp: smtpSettings(settings) };
}
