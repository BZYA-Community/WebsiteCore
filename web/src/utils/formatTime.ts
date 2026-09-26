/**
 * @file 格式化日期
 */

import moment from 'moment';
import 'moment/dist/locale/zh-cn';

/** moment 语言切换, 由 @/locales/index.ts 在初始化与 setLocale 时统一调用 */
export function setMomentLocale(locale: string) {
  moment.locale(locale === 'en' ? 'en' : 'zh-cn');
}

// 默认 zh-cn
setMomentLocale('zh-CN');

export const formatTime = (time: number) => {
  return moment.unix(time).utc(true).format('YYYY-MM-DD HH:mm');
};

export const formatHumanTime = (time: number) => {
  return moment().from(moment.unix(time));
};

export const formatRelativeTime = (time: number) => {
  return moment.unix(time).fromNow();
};

export const formatPrettyTime = (time: number) => {
  const mt = moment.unix(time);
  const now = moment();
  if (mt.year() != now.year()) {
    return mt.utc(true).format('YYYY-MM-DD HH:mm');
  } else if (moment().diff(mt, 'month') > 3) {
    return mt.utc(true).format('MM-DD HH:mm');
  }
  return mt.fromNow();
};

export const formatPrettyDate = (time: number) => {
  const mt = moment.unix(time);
  const now = moment();
  if (mt.year() != now.year()) {
    return mt.utc(true).format('YYYY-MM-DD');
  } else if (moment().diff(mt, 'month') > 3) {
    return mt.utc(true).format('MM-DD');
  }
  return mt.fromNow();
};

export const formatDate = (time: number) => {
  // 'LL' 随 moment.locale 输出本地化年月 (zh-cn: 2026年9月 / en: September 2026)
  return moment.unix(time).utc(true).format('LL');
};
