export const parsePostTag = (content: string) => {
  const tags: string[] = [];
  const users: string[] = [];
  // 话题: 井号后紧跟非空白字符(#话题 空白结尾 / #话题# 闭合式)
  // 井号后有空格(# 标题)或多个井号(## 标题)不匹配 —— 那是Markdown标题
  const tagExp = /(#|＃)([^#@\s])+?(\s+?|#|＃|$)/g; // 这⾥中⽂#和英⽂#都会识别
  const atExp = /@([a-zA-Z0-9])+?\s+?/g; // 这⾥中⽂#和英⽂#都会识别
  content = content
    .replace(/<[^>]*?>/gi, '')
    .replace(/(.*?)<\/[^>]*?>/gi, '')
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
    if (latestChar == '#' || latestChar == '#' || latestChar == '@') {
      content = content.substring(0, maxSize - 1);
    }
  }
  const tagExp = /(#|＃)([^#@\s])+?(\s+?|#|＃|$)/g; // 这⾥中⽂#和英⽂#都会识别
  const atExp = /@([a-zA-Z0-9])+?\s+?/g; // 这⾥中⽂#和英⽂#都会识别
  content = content
    .replace(/<[^>]*?>/gi, '')
    .replace(/(.*?)<\/[^>]*?>/gi, '')
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
