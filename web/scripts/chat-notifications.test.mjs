import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import * as vue from 'vue';
import ts from 'typescript';
import moment from 'moment';
import { toSystemNotification } from '../src/utils/systemNotification.ts';

const follow = {
  id: 1,
  sender_id: 12,
  sender_name: '小明',
  sender_username: 'follower',
  brief: '关注了你',
  content: '快去看看 TA 的主页吧',
  type: 99,
  timestamp: 1791331200,
  seen: true,
};
const audit = {
  ...follow,
  id: 2,
  sender_id: 0,
  sender_name: '',
  sender_username: '',
  brief: '动态审核通过',
  content: '审核结果',
  post_id: 42,
};

assert.equal(toSystemNotification(follow).senderName, '小明');
assert.equal(toSystemNotification(follow).profilePath, '#/u?s=follower');
assert.equal(toSystemNotification(follow).postPath, undefined);
assert.equal(
  toSystemNotification(audit).postPath,
  '#/post?id=42',
  'type 99 audit links to its post',
);
assert.equal(
  toSystemNotification(audit).profilePath,
  undefined,
  'system sender has no profile',
);
for (const type of [1, 2, 3]) {
  assert.equal(
    toSystemNotification({ ...follow, type, post_id: 7 }).postPath,
    '#/post?id=7',
  );
}
assert.equal(
  toSystemNotification({ ...follow, sender_name: '' }).senderName,
  'follower',
);
assert.equal(
  toSystemNotification({ ...follow, sender_username: '' }).profilePath,
  undefined,
);
assert.equal(
  toSystemNotification({ ...audit, post_id: 0 }).postPath,
  undefined,
);
const unsafe = toSystemNotification({
  ...follow,
  sender_username: 'x&next=javascript:alert(1)',
  content: '<img src=x onerror=alert(1)>',
  sender_name: '<script>alert(1)</script>',
});
assert.equal(unsafe.profilePath, '#/u?s=x%26next%3Djavascript%3Aalert(1)');
assert.equal(
  unsafe.content,
  '<img src=x onerror=alert(1)>',
  'content remains plain text',
);

const source = fs
  .readFileSync(
    process.env.CHAT_SOURCE_PATH ||
      new URL('../src/composables/useChatRooms.ts', import.meta.url),
    'utf8',
  )
  .replace(/^import [\s\S]*?;\r?$/gm, '')
  .replace(/export /g, '');
const code = ts.transpileModule(`${source}\nmodule.exports = useChatRooms;`, {
  compilerOptions: { target: ts.ScriptTarget.ES2022 },
}).outputText;
const translate = (key, params) =>
  params?.url ? `${key}: ${params.url}` : key;
const context = {
  ...vue,
  moment,
  console,
  module: { exports: {} },
  toSystemNotification,
  location: { origin: 'https://example.test', pathname: '/' },
  i18n: { global: { t: translate } },
  useStoreUser: () => ({ userInfo: { id: 34 } }),
  useStoreProfile: () => ({ profile: {} }),
  Api: {
    v1: {
      user: {
        get: {
          chat: {
            history: async ({ user_id }) => ({
              messages:
                user_id === 0
                  ? [follow, audit]
                  : [{ ...follow, type: 4, content: 'private text' }],
              total_rows: user_id === 0 ? 2 : 1,
              can_send: user_id !== 0,
            }),
          },
        },
      },
    },
  },
};
const renderer = vue.createRenderer({
  createElement: () => ({}),
  createText: () => ({}),
  createComment: () => ({}),
  insert() {},
  remove() {},
  setText() {},
  setElementText() {},
  patchProp() {},
  parentNode: () => null,
  nextSibling: () => null,
});
let chat;
const app = renderer.createApp({
  setup() {
    vm.runInNewContext(code, context);
    chat = context.module.exports();
    return () => vue.h('div');
  },
});
app.mount({});
try {
  await chat.onFetchMessages({
    room: { roomId: '0' },
    options: { reset: true },
  });
  assert.equal(
    chat.messages.value[0].notification?.profilePath,
    '#/u?s=follower',
    'old follow notices have a real profile action',
  );
  assert.equal(chat.messages.value[1].notification?.postPath, '#/post?id=42');
  assert.equal(
    chat.messages.value[0].senderId,
    '0',
    'notifications are not outgoing chat bubbles',
  );
  assert.equal(
    chat.messages.value[0].content,
    follow.content,
    'do not append raw URLs',
  );
  assert.equal(chat.canSend.value, false);
  await chat.onFetchMessages({
    room: { roomId: '12' },
    options: { reset: true },
  });
  assert.equal(
    chat.messages.value[0].notification,
    undefined,
    'private messages keep their normal rendering',
  );
  assert.equal(chat.messages.value[0].content, 'private text');
  assert.equal(chat.canSend.value, true);
  console.log(
    'Chat notification checks passed: follow/audit/comment links, legacy senders, safe paths, and private-message isolation.',
  );
} finally {
  app.unmount();
}
