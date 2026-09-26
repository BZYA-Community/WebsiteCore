/**
 * @file i18n 国际化入口
 *
 * 语言包组织方式: src/locales/<locale>/<namespace>.json
 * 文件名即 namespace (如 ./zh-CN/common.json -> messages['zh-CN'].common)。
 * 纯 JSON 文件可直接导入 Crowdin / Weblate / Tolgee 等翻译平台,
 * 由社区译者按模块认领维护其他语言。
 */
import { createI18n } from 'vue-i18n';
import { setMomentLocale } from '@/utils/formatTime';

export const LOCALE_KEY = 'PAOPAO_LOCALE';

export const SUPPORTED_LOCALES = ['zh-CN', 'en'] as const;
export type SupportedLocale = (typeof SUPPORTED_LOCALES)[number];

/** 语言选项自称固定展示, 不进语言包 */
export const localeOptions: { label: string; value: SupportedLocale }[] = [
  { label: '简体中文', value: 'zh-CN' },
  { label: 'English', value: 'en' },
];

// 按 namespace 聚合语言包 (由 @intlify/unplugin-vue-i18n 预编译)
const modules = import.meta.glob('./**/*.json', { eager: true }) as Record<
  string,
  { default: Record<string, unknown> }
>;

const messages: Record<string, Record<string, unknown>> = {};
for (const path of Object.keys(modules)) {
  const matched = path.match(/^\.\/([^/]+)\/([^/]+)\.json$/);
  if (!matched) continue;
  const [, locale, namespace] = matched;
  messages[locale] ??= {};
  messages[locale][namespace] = modules[path].default;
}

/** 语言检测: localStorage -> 浏览器语言 -> 默认 zh-CN */
function detectLocale(): SupportedLocale {
  const saved = localStorage.getItem(LOCALE_KEY);
  if (saved && (SUPPORTED_LOCALES as readonly string[]).includes(saved)) {
    return saved as SupportedLocale;
  }
  const navLang = (navigator.language || '').toLowerCase();
  if (navLang.startsWith('zh')) return 'zh-CN';
  return 'en';
}

const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: detectLocale(),
  fallbackLocale: 'zh-CN',
  messages,
  missingWarn: false,
  fallbackWarn: false,
});

function applyLocaleSideEffects(loc: string) {
  document.documentElement.lang = loc;
  setMomentLocale(loc);
  // document.title 由 router 按路由 titleKey 统一维护 (含语言切换时的刷新)
}

export function setLocale(loc: SupportedLocale) {
  i18n.global.locale.value = loc;
  localStorage.setItem(LOCALE_KEY, loc);
  applyLocaleSideEffects(loc);
}

export function getLocale(): SupportedLocale {
  return i18n.global.locale.value as SupportedLocale;
}

applyLocaleSideEffects(getLocale());

export default i18n;
