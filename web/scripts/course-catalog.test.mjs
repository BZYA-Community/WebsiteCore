import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import * as vue from 'vue';
import ts from 'typescript';

const source = fs.readFileSync(new URL('../src/views/Courses.vue', import.meta.url), 'utf8')
  .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1]
  .replace(/^import .*;\r?$/gm, '');
const code = ts.transpileModule(source + '\nmodule.exports = {groupsLoading, loading, courses, removeCourse, selectedGroup, selectedPath, editGroup, groupDraft, editCourse, courseDraft};',
  { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText;
const course = { id: 1, group_id: 2, teacher_id: 8, title: 'Delete me' };
let savedCourses = [course, { ...course, id: 2, title: 'Keep me' }];
let failDelete = false;
const requestsForDeleted = [], successMessages = [];
const route = vue.reactive({ query: { category: '2', manage: '1' } });
const context = {
  ...vue,
  module: { exports: {} },
  window: { $message: { success: (message) => successMessages.push(message) } },
  useI18n: () => ({ t: (key) => key }),
  useRoute: () => route,
  useRouter: () => ({}),
  useStoreUser: () => ({ hasPermission: () => true, hasAnyPermission: () => true, userInfo: { id: 8 } }),
  useStoreMain: () => ({}),
  getCourseGroups: async () => ({ groups: [{ id: 2, parent_id: 0, name: 'Category', course_count: savedCourses.length }] }),
  getCourseList: async () => ({ list: [...savedCourses], pager: { total_rows: savedCourses.length } }),
  deleteCourse: async ({ id }) => {
    if (failDelete) throw new Error('Delete failed');
    savedCourses = savedCourses.filter((item) => item.id !== id);
  },
};
// Run Vue's actual mount/unmount scheduling with inert host nodes, without a browser.
const renderer = vue.createRenderer({
  createElement: () => ({}), createText: () => ({}), createComment: () => ({}),
  insert() {}, remove() {}, setText() {}, setElementText() {}, patchProp() {},
  parentNode: () => null, nextSibling: () => null,
});
const Lessons = {
  props: ['course'],
  setup(props) {
    if (!savedCourses.some((item) => item.id === props.course.id)) requestsForDeleted.push(props.course.id);
    return () => vue.h('div');
  },
};
let page;
const app = renderer.createApp({
  setup() {
    vm.runInNewContext(code, context);
    page = context.module.exports;
    return () => vue.h('main', page.groupsLoading.value || page.loading.value ? [] :
      page.courses.value.map((item) => vue.h(Lessons, { key: item.id, course: item })));
  },
});
const tick = async () => { for (let i = 0; i < 5; i++) { await Promise.resolve(); await vue.nextTick(); } };
app.mount({});
try {
  await tick();
  assert.equal(page.courses.value.length, 2);
  failDelete = true;
  assert.equal(await page.removeCourse(course), false);
  assert.equal(page.courses.value.length, 2, 'failed deletion preserves the course');
  failDelete = false;
  await page.removeCourse(course);
  await tick();
  assert.deepEqual(requestsForDeleted, [], 'refresh must not remount the deleted course and request its lessons');
  assert.deepEqual(Array.from(page.courses.value, (item) => item.id), [2]);
  assert.deepEqual(successMessages, ['course.deleteSuccess']);
  route.query.category = '999';
  await tick();
  assert.equal(page.selectedGroup.value, undefined);
  assert.equal(page.selectedPath.value.length, 0);
  page.editGroup();
  assert.equal(page.groupDraft.parent_id, 0, 'a missing category must not become the new major category parent');
  page.editCourse();
  assert.equal(page.courseDraft.group_id, null, 'a missing category requires an explicit valid selection');
  route.query.category = '2';
  await tick();
  assert.equal(page.selectedGroup.value.id, 2, 'query changes reactively select the valid category');
  page.editGroup();
  assert.equal(page.groupDraft.parent_id, 2);
  page.editCourse();
  assert.equal(page.courseDraft.group_id, 2);
  console.log('Course catalog checks passed: deletion retention, no stale lesson reload, missing-category defaults, reactive category selection.');
} finally {
  app.unmount();
}
