import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import * as vue from 'vue';
import ts from 'typescript';

const source = fs.readFileSync(new URL('../src/views/AdminUsers.vue', import.meta.url), 'utf8')
  .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1]
  .replace(/^import [\s\S]*?;\r?$/gm, '');
const code = ts.transpileModule(source + '\nmodule.exports = { registeredRange, keyword, users, loading, loadUsers, pagination, changePageSize, checkedRowKeys, checkedUsers, columns, openBatchMembership, batchOpen, batchUsers, batchGroups, saveBatchMembership };',
  { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText;
const userInfo = vue.ref({ id: 1, is_operator: false });
const rows = [
  { id: 1, username: 'self' }, { id: 2, username: 'operator', is_operator: true },
  { id: 3, username: 'first', identity_groups: [] }, { id: 4, username: 'second', identity_groups: [] },
  { id: 5, username: 'elevated', identity_groups: [{ key: 'publisher', permissions: ['content.publish_unreviewed'] }] },
];
const queries = [], writes = [], messages = [];
let failBatch = false, deferList = false;
const pendingLists = [];
const context = {
  ...vue, Date, console, module: { exports: {} },
  window: { $message: { success: (message) => messages.push(message) } },
  useI18n: () => ({ t: (key) => key }), useDialog: () => ({}),
  useStoreUser: () => ({ userInfo, hasPermission: (permission) => permission !== 'content.publish_unreviewed' }),
  storeToRefs: (store) => store,
  assignable: (group) => group.key !== 'guest' && group.key !== 'member',
  getIdentityGroups: async () => ({ groups: [] }),
  setUsersIdentity: async (ids, groups) => {
    writes.push({ ids: [...ids], groups: [...groups] });
    if (failBatch) throw new Error('Permission denied');
  },
  Api: { v1: { admin: { get: { user: { list: async (query) => {
    queries.push(query);
    if (deferList) return new Promise(resolve => pendingLists.push(resolve));
    return { list: rows, pager: { total_rows: 5 } };
  } } } } } },
};
const renderer = vue.createRenderer({
  createElement: () => ({}), createText: () => ({}), createComment: () => ({}),
  insert() {}, remove() {}, setText() {}, setElementText() {}, patchProp() {},
  parentNode: () => null, nextSibling: () => null,
});
let page;
const app = renderer.createApp({ setup() {
  vm.runInNewContext(code, context);
  page = context.module.exports;
  return () => vue.h('div');
} });
const tick = async () => { for (let i = 0; i < 5; i++) { await Promise.resolve(); await vue.nextTick(); } };
app.mount({});
try {
  await tick();
  const from = new Date(2026, 9, 1).getTime(), through = new Date(2026, 9, 7).getTime();
  page.registeredRange.value = [from, through];
  page.keyword.value = ' first ';
  await page.loadUsers();
  assert.equal(queries.at(-1).registered_from, Math.floor(from / 1000));
  assert.equal(queries.at(-1).registered_to, Math.floor(new Date(2026, 9, 7, 23, 59, 59).getTime() / 1000), 'include the entire final local calendar day');
  assert.equal(queries.at(-1).keyword, 'first');
  const selection = page.columns.value[0];
  assert.equal(selection.type, 'selection');
  assert.equal(selection.disabled(rows[0]), true, 'own identity is protected');
  assert.equal(selection.disabled(rows[1]), true, 'operators are protected');
  assert.equal(selection.disabled(rows[4]), true, 'elevated identities are protected');
  page.checkedRowKeys.value = [3, 4];
  page.openBatchMembership();
  page.batchGroups.value = [8];
  failBatch = true;
  await page.saveBatchMembership();
  assert.equal(page.batchOpen.value, true, 'failed batch remains available for retry');
  assert.equal(page.checkedUsers.value.length, 2);
  failBatch = false;
  await page.saveBatchMembership();
  assert.deepEqual(writes.at(-1), { ids: [3, 4], groups: [8] });
  assert.equal(page.batchOpen.value, false);
  assert.equal(page.checkedRowKeys.value.length, 0, 'successful refresh clears stale selections');
  assert.deepEqual(messages, ['common.operationSuccess']);
  page.registeredRange.value = null;
  page.changePageSize(100);
  await tick();
  assert.equal(queries.at(-1).page, 1);
  assert.equal(queries.at(-1).page_size, 100);
  assert.equal(queries.at(-1).registered_from, undefined);
  assert.equal(queries.at(-1).registered_to, undefined);
  deferList = true;
  const older = page.loadUsers(), newer = page.loadUsers();
  pendingLists[1]({ list: [rows[3]], pager: { total_rows: 1 } });
  await newer;
  pendingLists[0]({ list: rows, pager: { total_rows: 5 } });
  await older;
  assert.equal(page.users.value.length, 1, 'an older filter response cannot overwrite the current result');
  console.log('Admin users checks passed: inclusive date range, keywords, selection protections, bulk retry, pagination and stale responses.');
} finally {
  app.unmount();
}
