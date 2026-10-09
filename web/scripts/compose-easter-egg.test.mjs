import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';
import { redirectPostEasterEgg } from '../src/utils/content.ts';

const cases = [
  ['Never Gonna Give You Up', 'https://www.bilibili.com/video/BV1dyXqBaEE5/'],
  ['毕业旅行', 'https://www.bilibili.com/video/BV1Kj411g7Lu/'],
  ['神州平板', 'https://www.bilibili.com/video/BV1jN411o7zS/'],
  ['最后一课', 'https://www.bilibili.com/video/BV1Ut411v74a/'],
  ['因你而在的故事', 'https://www.bilibili.com/video/BV1fY4y1F7GL/'],
  ['往昔的涟漪', 'https://www.bilibili.com/video/BV14G1kB5Evp/'],
  ['希儿希儿希', 'https://www.bilibili.com/video/BV1cE411Z74g/'],
  ['女王降临', 'https://www.bilibili.com/video/BV1aW411P7UJ/'],
  ['天使重构', 'https://www.bilibili.com/video/BV1v4411A7kc/'],
  ['阿波卡利斯如是说', 'https://www.bilibili.com/video/BV1bY411b7k9/'],
  ['芽衣姐姐', 'https://www.bilibili.com/video/BV1UT4y1J7xN/'],
  ['格拉默的余烬', 'https://www.bilibili.com/video/BV1us421u7sp/'],
  ['此刻，在同一片星空下', 'https://www.bilibili.com/video/BV16s421u7NP/'],
  ['使一颗心免于哀伤', 'https://www.bilibili.com/video/BV15x4y1i7m9/'],
  ['希望有羽毛和翅膀', 'https://www.bilibili.com/video/BV1Tf421m7iW/'],
  ['玄黄', 'https://www.bilibili.com/video/BV1Th4y1S7KF/'],
  ['飞光', 'https://www.bilibili.com/video/BV16g4y157Zc/'],
  ['听！狂欢在那神佑的山巅', 'https://www.bilibili.com/video/BV1twgSzhEo8/'],
  ['永劫轮舞', 'https://www.bilibili.com/video/BV1aC411h7e6/'],
  ['不眠之夜', 'https://www.bilibili.com/video/BV1me411n7eu/'],
  ['独角戏', 'https://www.bilibili.com/video/BV1mW421A7nM/'],
  ['你好，世界', 'https://www.bilibili.com/video/BV14G1kB5Evp/'],
  ['折枝落梦', 'https://www.bilibili.com/video/BV17D4y1t74j/'],
  ['罪人的终幕', 'https://www.bilibili.com/video/BV1vu4y1b7Y9/'],
  ['轻涟', 'https://www.bilibili.com/video/BV1Jg4y197YW/'],
  ['惟余旧忆', 'https://www.bilibili.com/video/BV1mW4y1C7eD/'],
  ['神女劈观', 'https://www.bilibili.com/video/BV1kS4y1T7kK/'],
  ['花车颠呀颠', 'https://www.bilibili.com/video/BV1AG4y1h7Ap/'],
  ['未行之路', 'https://www.bilibili.com/video/BV1bD421G7hy/'],
  ['烬中歌', 'https://www.bilibili.com/video/BV1xZ421Y7a1/'],
  ['雪霁逢椿', 'https://www.bilibili.com/video/BV1ZF411g7EZ/'],
];
const limit = cases[0][0].length;
const redirects = [];
globalThis.window = { location: { assign: (url) => redirects.push(url) } };

for (const file of ['components/compose.vue', 'views/ComposeMd.vue']) {
  const source = fs.readFileSync(new URL(`../src/${file}`, import.meta.url), 'utf8')
    .match(/const changeContent = \(v: string\) => \{[\s\S]*?\r?\n\};/)[0];
  const code = ts.transpileModule(source + '\nchangeContent;',
    { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText;
  redirects.length = 0;
  const content = { value: '' };
  let draftsSaved = 0;
  const changeContent = vm.runInNewContext(code, {
    content, maxInputLength: { value: limit }, MD_MAX_LENGTH: limit, redirectPostEasterEgg,
    saveDraft: () => { draftsSaved++; },
  });

  const nonMatches = [
    '', '普通帖子', 'never gonna give you up', '__proto__', 'constructor', 'toString',
    '听!狂欢在那神佑的山巅', '你好,世界',
  ];
  for (const value of nonMatches) {
    changeContent(value);
    assert.equal(content.value, value.substring(0, limit), `${file}: preserve input limits`);
    assert.equal(redirects.length, 0, `${file}: only exact input may redirect`);
  }

  for (const [trigger, url] of cases) {
    redirects.length = 0;
    for (const value of [` ${trigger}`, `${trigger} `, `${trigger}\n`, `${trigger} extra`, trigger.slice(0, -1)]) {
      changeContent(value);
      assert.equal(content.value, value.substring(0, limit), `${file}: preserve input limits`);
      assert.equal(redirects.length, 0, `${file}: only exact input may redirect`);
    }
    changeContent(trigger);
    assert.equal(content.value, trigger);
    assert.deepEqual(redirects, [url], `${file}: ${trigger} redirects to its specified video`);
  }
  if (file === 'views/ComposeMd.vue') {
    assert.equal(draftsSaved, nonMatches.length + cases.length * 6, 'Markdown input still saves drafts');
  }
}
console.log(`Compose easter egg checks passed: ${cases.length} mappings in both editors, near misses, input limits and Markdown drafts.`);
