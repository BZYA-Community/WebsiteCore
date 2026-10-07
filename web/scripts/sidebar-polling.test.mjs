import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import * as vue from 'vue';
import ts from 'typescript';

const source = fs.readFileSync(new URL('../src/components/sidebar.vue', import.meta.url), 'utf8')
  .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1]
  .replace(/^import [\s\S]*?;\r?$/gm, '');
const code = ts.transpileModule(source + '\nmodule.exports = { hasUnreadMsg };',
  { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText;
const userInfo = vue.ref({ id: 0 });
const allowed = vue.ref(true);
const profile = vue.ref({ defaultMsgLoopInterval: 5000 });
const unreadMsgCount = vue.ref(0);
const timers = new Map();
const requests = [];
let nextTimer = 0;
const context = {
  ...vue, console, module: { exports: {} }, window: {},
  useI18n: () => ({ t: (key) => key }),
  useRoute: () => ({}), useRouter: () => ({}), useMessage: () => ({}),
  useStoreMain: () => ({ unreadMsgCount, updateUnreadMsgCount: (count) => { unreadMsgCount.value = count; } }),
  useStoreUser: () => ({ userInfo, hasPermission: () => allowed.value }),
  useStoreProfile: () => ({ profile }), storeToRefs: (store) => store,
  setInterval: (callback, interval) => { const id = ++nextTimer; timers.set(id, { callback, interval }); return id; },
  clearInterval: (id) => timers.delete(id),
  Api: { v1: { user: { get: { msgcount: { unread: () => new Promise((resolve, reject) => requests.push({ resolve, reject })) } } } } },
};
const scope = vue.effectScope();
scope.run(() => vm.runInNewContext(code, context));
const tick = async () => { await Promise.resolve(); await vue.nextTick(); };
const poll = () => { for (const timer of timers.values()) timer.callback(); };

try {
  assert.equal(requests.length, 0, 'guests must not poll');
  userInfo.value = { id: 1 };
  assert.equal(requests.length, 1, 'login immediately loads unread messages');
  assert.equal(timers.size, 1);
  poll();
  assert.equal(requests.length, 1, 'slow requests must not overlap');
  requests[0].resolve({ count: 2 });
  await tick();
  assert.equal(context.module.exports.hasUnreadMsg.value, true);
  unreadMsgCount.value = 0;
  assert.equal(context.module.exports.hasUnreadMsg.value, false, 'marking messages read updates the badge');
  poll();
  userInfo.value = { id: 0 };
  assert.equal(timers.size, 0, 'logout stops polling');
  requests[1].resolve({ count: 9 });
  await tick();
  assert.equal(unreadMsgCount.value, 0, 'a logged-out request cannot update the next session');
  userInfo.value = { id: 2 };
  assert.equal(requests.length, 3, 'login again restarts polling');
  allowed.value = false;
  assert.equal(timers.size, 0, 'revoking profile permission stops polling');
  requests[2].resolve({ count: 8 });
  await tick();
  assert.equal(unreadMsgCount.value, 0);
  allowed.value = true;
  requests[3].reject(new Error('network unavailable'));
  await tick();
  profile.value.defaultMsgLoopInterval = 10000;
  assert.equal(timers.size, 1, 'interval changes replace the existing timer');
  assert.equal([...timers.values()][0].interval, 10000);
  scope.stop();
  assert.equal(timers.size, 0, 'unmount cleans up polling');
  requests[4].resolve({ count: 7 });
  await tick();
  assert.equal(unreadMsgCount.value, 0, 'an unmounted request cannot update the store');
  console.log('Sidebar polling checks passed: login, badge, request overlap, logout, permissions, interval changes and cleanup.');
} finally {
  scope.stop();
}
