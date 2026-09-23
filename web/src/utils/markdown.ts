import { config, XSSPlugin } from 'md-editor-v3';
import type { Themes } from 'md-editor-v3';
import 'md-editor-v3/lib/preview.css';

// markdown-it 渲染结果经 XSS 白名单过滤(script/事件属性等被移除)后才进入预览
config({
  markdownItConfig(md) {
    md.use(XSSPlugin, {});
  },
});

/** Markdown 长文内容字数上限(普通动态沿用 profile.defaultTweetMaxLength) */
export const MD_MAX_LENGTH = 50000;

/** 主页/列表页节选行数 */
export const MD_EXCERPT_LINES = 5;

/** 获取主题对应的编辑器主题 */
export const mdTheme = (theme: string | null): Themes =>
  theme === 'dark' ? 'dark' : 'light';

// 话题token: 井号后紧跟非空白字符(#话题# 闭合式 / #话题+空白 开放式)
// 井号后有空格(# 标题)或多个井号(## 标题)都不匹配 —— 那是Markdown标题
// 排除 []() 字符避免破坏Markdown链接语法
const mdTagExp = /([#＃])([^#＃@\s[\]()]+)([#＃])|([#＃])([^#＃@\s[\]()]+)(?=\s|$)/g;

/**
 * 将Markdown中的话题token转换为可点击的链接
 * 话题被转成 [#话题](#) 形式的Markdown原生链接,
 * 渲染后由容器点击委托识别(href==="#"且文本以#开头)并跳转话题搜索
 */
export const linkifyMdTopics = (md: string): string => {
  return md.replace(mdTagExp, (_raw, h1: string, tag1: string, close: string, h2: string, tag2: string) => {
    const display = h1 ? h1 + tag1 + close : h2 + tag2;
    return `[${display}](#)`;
  });
};

/** 处理Markdown渲染前的内容(当前仅话题链接化) */
export const prepareMdRender = (md: string): string => linkifyMdTopics(md);

/**
 * 截取Markdown前N行作为列表页节选
 * 返回节选文本及是否发生了截断
 */
export const mdExcerpt = (
  md: string,
  lines: number = MD_EXCERPT_LINES,
): { text: string; truncated: boolean } => {
  const allLines = md.split('\n');
  if (allLines.length <= lines) {
    return { text: md, truncated: false };
  }
  return { text: allLines.slice(0, lines).join('\n'), truncated: true };
};

/** 去除Markdown语法标记得到纯文本(用于审核卡片/通知等摘要场景) */
export const mdPlainText = (md: string): string => {
  return md
    .replace(/```[\s\S]*?(```|$)/g, ' ')
    .replace(/`([^`]*)`/g, '$1')
    .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/^\s{0,3}#{1,6}\s*/gm, '')
    .replace(/^\s{0,3}>\s?/gm, '')
    .replace(/^\s{0,3}([-*+]|\d+\.)\s+/gm, '')
    .replace(/[*_~#＃]+/g, '')
    .replace(/\s+/g, ' ')
    .trim();
};
