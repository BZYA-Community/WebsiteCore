/**
 * @file 后端错误码 -> i18n 文案映射
 *
 * 后端 (internal/model/web/xerror.go, pkg/xerror/common.go) 不做 i18n,
 * 前端按错误码查 errors.codes.E<code>; 未映射的 code 回退后端原始 msg。
 */
import i18n from './index';

export function translateErrMsg(
  code?: number,
  msg?: string,
  fallbackKey = 'errors.requestFailed',
): string {
  const { t, te } = i18n.global;
  if (code != null && +code !== 0) {
    const key = `errors.codes.E${code}`;
    if (te(key as never)) return t(key as never);
  }
  if (msg) return msg;
  return t(fallbackKey as never);
}
