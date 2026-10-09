const postEasterEggVideos = new Map([
  ['Never Gonna Give You Up', 'BV1dyXqBaEE5'],
  ['毕业旅行', 'BV1Kj411g7Lu'],
  ['神州平板', 'BV1jN411o7zS'],
  ['最后一课', 'BV1Ut411v74a'],
  ['因你而在的故事', 'BV1fY4y1F7GL'],
  ['往昔的涟漪', 'BV14G1kB5Evp'],
  ['希儿希儿希', 'BV1cE411Z74g'],
  ['女王降临', 'BV1aW411P7UJ'],
  ['天使重构', 'BV1v4411A7kc'],
  ['阿波卡利斯如是说', 'BV1bY411b7k9'],
  ['芽衣姐姐', 'BV1UT4y1J7xN'],
  ['格拉默的余烬', 'BV1us421u7sp'],
  ['此刻，在同一片星空下', 'BV16s421u7NP'],
  ['使一颗心免于哀伤', 'BV15x4y1i7m9'],
  ['希望有羽毛和翅膀', 'BV1Tf421m7iW'],
  ['玄黄', 'BV1Th4y1S7KF'],
  ['飞光', 'BV16g4y157Zc'],
  ['听！狂欢在那神佑的山巅', 'BV1twgSzhEo8'],
  ['永劫轮舞', 'BV1aC411h7e6'],
  ['不眠之夜', 'BV1me411n7eu'],
  ['独角戏', 'BV1mW421A7nM'],
  ['你好，世界', 'BV14G1kB5Evp'],
  ['折枝落梦', 'BV17D4y1t74j'],
  ['罪人的终幕', 'BV1vu4y1b7Y9'],
  ['轻涟', 'BV1Jg4y197YW'],
  ['惟余旧忆', 'BV1mW4y1C7eD'],
  ['神女劈观', 'BV1kS4y1T7kK'],
  ['花车颠呀颠', 'BV1AG4y1h7Ap'],
  ['未行之路', 'BV1bD421G7hy'],
  ['烬中歌', 'BV1xZ421Y7a1'],
  ['雪霁逢椿', 'BV1ZF411g7EZ'],
]);

export const redirectPostEasterEgg = (content: string) => {
  const videoId = postEasterEggVideos.get(content);
  if (videoId) {
    window.location.assign(`https://www.bilibili.com/video/${videoId}/`);
  }
};

// HTML实体转义: v-html 渲染前保证文本不会被解析为标签
const escapeHtml = (text: string): string =>
  text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

// 剥离HTML标签: 用浏览器DOM解析器解析后取纯文本再转义,
// 避免正则过滤可被构造残缺标签绕过的问题(且防止实体解码后的内容重新形成标签)
const stripHtml = (content: string): string => {
  const doc = new DOMParser().parseFromString(content, 'text/html');
  return escapeHtml(doc.body.textContent ?? '');
};

export const parsePostTag = (content: string) => {
  const tags: string[] = [];
  const users: string[] = [];
  // 话题: 井号后紧跟非空白字符(#话题 空白结尾 / #话题# 闭合式)
  // 井号后有空格(# 标题)或多个井号(## 标题)不匹配 —— 那是Markdown标题
  const tagExp = /(#|＃)([^#@\s])+?(\s+?|#|＃|$)/g; // 这⾥中⽂#和英⽂#都会识别
  const atExp = /@([a-zA-Z0-9])+?\s+?/g; // 这⾥中⽂#和英⽂#都会识别
  content = stripHtml(content)
    .replace(tagExp, (item) => {
      const raw = item.trim();
      let tag = raw.substring(1);
      // 闭合式话题去掉结尾井号
      if (tag.endsWith('#') || tag.endsWith('＃')) {
        tag = tag.slice(0, -1);
      }
      tags.push(tag);
      return (
        '<a class="hash-link" data-detail="tag:' +
        encodeURIComponent(tag) +
        '">' +
        raw +
        '</a> '
      );
    })
    .replace(atExp, (item) => {
      users.push(item.substr(1).trim());
      return (
        '<a class="hash-link" data-detail="user:' +
        encodeURIComponent(item.substr(1).trim()) +
        '">' +
        item.trim() +
        '</a> '
      );
    });
  return { content, tags, users };
};

export const preparePost = (
  content: string,
  foldHint: string,
  unfoldHint: string,
  maxSize: number,
  isFold: boolean = true,
) => {
  const isEllipsis = content.length > maxSize;
  if (isFold && isEllipsis) {
    content = content.substring(0, maxSize);
    const latestChar = content.charAt(maxSize - 1);
    if (latestChar == '#' || latestChar == '＃' || latestChar == '@') {
      content = content.substring(0, maxSize - 1);
    }
  }
  const tagExp = /(#|＃)([^#@\s])+?(\s+?|#|＃|$)/g; // 这⾥中⽂#和英⽂#都会识别
  const atExp = /@([a-zA-Z0-9])+?\s+?/g; // 这⾥中⽂#和英⽂#都会识别
  content = stripHtml(content)
    .replace(tagExp, (item) => {
      const raw = item.trim();
      let tag = raw.substring(1);
      // 闭合式话题去掉结尾井号
      if (tag.endsWith('#') || tag.endsWith('＃')) {
        tag = tag.slice(0, -1);
      }
      return (
        '<a class="hash-link" data-detail="tag:' +
        encodeURIComponent(tag) +
        '">' +
        raw +
        '</a> '
      );
    })
    .replace(atExp, (item) => {
      return (
        '<a class="hash-link" data-detail="user:' +
        encodeURIComponent(item.substring(1).trim()) +
        '">' +
        item.trim() +
        '</a> '
      );
    });
  if (isEllipsis) {
    content =
      content.trimEnd() +
      (isFold ? '...&nbsp;' : '&nbsp;') +
      '<a class="hash-link" data-detail="post">' +
      (isFold ? foldHint : unfoldHint) +
      '</a> ';
  }
  return content;
};
