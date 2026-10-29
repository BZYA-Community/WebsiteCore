// Exercise the list's actual submission button and the editor after permission
// revocation, without a backing server. Match the existing component harnesses.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import * as vue from 'vue';
import ts from 'typescript';
import { parse, compileTemplate } from '@vue/compiler-sfc';
import { renderToString } from '@vue/server-renderer';

const user = vue.reactive({
  userInfo: { id: 8 }, userLogined: true, permissions: [],
  hasPermission(permission) { return this.permissions.includes(permission); },
});
const route = vue.reactive({ query: { tab: 'mine' }, fullPath: '/columns?tab=mine' });
let opened;
const context = {
  ...vue, module: { exports: {} },
  useI18n: () => ({ t: key => key, locale: vue.ref('en') }),
  useStoreUser: () => user,
  useStoreMain: () => ({ theme: 'light' }),
  useRoute: () => route,
  useRouter: () => ({ push: value => { opened = value; } }),
  onBeforeUnmount() {}, onMounted() {}, watch() {},
  localStorage: { getItem: () => null, length: 0 },
  formatPrettyTime: () => '', MD_MAX_LENGTH: 50000,
  ColumnDetail: {}, ColumnReturned: {}, ColumnDeleteButton: {},
  ColumnEditor: {}, CollectionEntries: {}, ColumnMediaInput: {},
};
function loadComponent(file, names, overrides = {}) {
  const { descriptor } = parse(fs.readFileSync(new URL(file, import.meta.url), 'utf8'));
  const source = descriptor.scriptSetup.content.replace(/^import[\s\S]*?;\s*$/gm, '');
  const code = ts.transpileModule(source + `\nmodule.exports = {${names}};`, {
    compilerOptions: { target: ts.ScriptTarget.ES2022 },
  }).outputText;
  const scope = { ...context, ...overrides, module: { exports: {} } };
  vm.runInNewContext(code, scope);
  return { state: scope.module.exports, template: descriptor.template.content };
}
const { state: list, template } = loadComponent('../src/views/Columns.vue',
  'items,total,view,tab,kind,drafts,keyword,official,returned,loading,error,user,t,canOpenSubmission,canEdit,edit,canOrganize,statusText,formatPrettyTime,changeTab,search,changeReturned,kinds,changeKind,load');
// Render the real template so removing its button guard is covered as well as
// the permission predicate. Stub layout and unrelated navigation components.
const compiled = compileTemplate({ source: template, filename: 'Columns.vue', id: 'access', compilerOptions: { mode: 'function' } });
assert.equal(compiled.errors.length, 0);
const render = new Function('Vue', compiled.code)(vue);
const wrapper = { setup: (_, { slots }) => () => vue.h('div', slots.default?.()) };
const button = { setup: (_, { slots }) => () => vue.h('button', slots.default?.()) };
const renderList = async () => {
  const app = vue.createSSRApp({ setup: () => list, render });
  for (const name of ['MainNav', 'NCard', 'NTabs', 'NTab', 'NSpace', 'NSelect', 'NInputGroup', 'NInput', 'NCheckbox', 'NSpin', 'NTag', 'RouterLink', 'ColumnEditor', 'ColumnReturned', 'CollectionEntries', 'ColumnDetail', 'NPopconfirm', 'NAlert', 'NEmpty', 'NImage', 'NPagination']) app.component(name, wrapper);
  app.component('NButton', button);
  app.component('ColumnDeleteButton', { render: () => null });
  return renderToString(app);
};
for (const kind of ['article', 'collection']) {
  for (const status of [0, 1, 2, 3]) {
    const item = { id: 11, kind, user_id: 8, title: 'Approved title', published_on: status === 1 ? 123 : 0,
      submission: { title: 'Latest submission', audit_status: status } };
    list.items.value = [item];
    assert.equal(list.canOpenSubmission(item), true);
    assert.equal(list.canEdit(item), false);
    assert.match(await renderList(), /<button[^>]*>columns.viewSubmission<\/button>/);
    list.edit(item);
    assert.equal(opened.query.view, 'edit');
    assert.equal(opened.query.id, item.id);
    const { state: editor } = loadComponent('../src/components/columns/column-editor.vue',
      'load,draft,readonly,canPublish,pending,error', {
        defineProps: () => ({ id: item.id, kind, draftId: 'new' }),
        getColumn: async (id, editing) => {
          assert.equal(id, item.id); assert.equal(editing, true);
          return { item: { ...item, submission: { ...item.submission, body: 'Preserved latest body' } } };
        },
      });
    await editor.load();
    assert.equal(editor.error.value, false);
    assert.equal(editor.draft.value.body, 'Preserved latest body');
    assert.equal(editor.readonly.value, true);
    assert.equal(editor.canPublish.value, false);
  }
}
const foreign = { id: 12, kind: 'article', user_id: 99 };
assert.equal(list.canOpenSubmission(foreign), false);
user.permissions = ['content.manage'];
assert.equal(list.canOpenSubmission(foreign), false);
assert.equal(list.canOpenSubmission({ ...foreign, kind: 'collection' }), true);
user.permissions = ['column.publish'];
const own = { id: 11, kind: 'article', user_id: 8 };
assert.equal(list.canEdit(own), true);
assert.equal(list.canOpenSubmission({ ...own, blocked: true }), true);
assert.equal(list.canEdit({ ...own, blocked: true }), false);
console.log('Column submission access: article/collection states, rendered owner button, read-only latest submission, and manager scope passed');
