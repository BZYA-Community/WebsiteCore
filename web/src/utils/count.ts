import i18n from '@/locales';

/**
 * 数字美化: 1500 -> 1.5千 (zh-CN) / 1.5K (en)
 * 在模板渲染中调用时读取 locale, 语言切换后自动生效
 */
export const prettyQuoteNum = (num: number) => {
  const { t, locale } = i18n.global;
  if (locale.value === 'en') {
    return new Intl.NumberFormat('en', {
      notation: 'compact',
      maximumFractionDigits: 1,
    }).format(num);
  }
  if (num >= 1000) {
    return t('common.thousand', { n: (num / 1000).toFixed(1) });
  } else if (num >= 10000) {
    return t('common.tenThousand', { n: (num / 10000).toFixed(1) });
  }
  return num;
};
