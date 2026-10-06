import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import * as vue from 'vue';
import ts from 'typescript';
const source = fs
  .readFileSync(new URL('../src/views/CourseDetail.vue', import.meta.url), 'utf8')
  .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1]
  .replace(/^import .*;\r?$/gm, '');
const code = ts.transpileModule(
  source +
    '\nmodule.exports = {course, lessons, lesson, videoUrl, videoError, videoLoading, selectedVideo, activeTab, comments, commentsLoaded, loadComments, loadVideo, loading};',
  { compilerOptions: { target: ts.ScriptTarget.ES2022 } },
).outputText;
const route = vue.reactive({ query: { id: '1' } });
const deferred = () => {
  let resolve, reject;
  const promise = new Promise((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};
const videoCalls = [],
  questionCalls = [],
  replacements = [],
  cleanup = [];
const video = (id) => ({
  id,
  attachment_id: id,
  name: `${id}.mp4`,
  kind: 'resource',
  file_size: 100,
  mime_type: 'video/mp4',
});
const outline = (id) => [
  {
    id: id * 10,
    course_id: id,
    title: 'First',
    intro: 'Intro',
    sort: 0,
    attachments: [video(id * 100)],
  },
  {
    id: id * 10 + 1,
    course_id: id,
    title: 'Second',
    intro: 'Other intro',
    sort: 1,
    attachments: [video(id * 100 + 1)],
  },
];
const context = {
  ...vue,
  URL,
  module: { exports: {} },
  window: { location: { origin: 'http://localhost' }, open: () => null },
  onBeforeUnmount: (fn) => cleanup.push(fn),
  useI18n: () => ({ t: (key) => key }),
  useRoute: () => route,
  useRouter: () => ({
    replace: async (loc) => {
      replacements.push(loc);
      route.query = loc.query;
    },
    push: (loc) => {
      route.query = loc.query;
    },
  }),
  useStoreUser: () => ({ hasPermission: () => false, userInfo: { id: 1 } }),
  getCourse: async ({ id }) => ({
    course: { id, title: 'Course', group_id: 1, teacher_id: 1 },
  }),
  getCourseLessons: async (id) => ({ lessons: outline(id) }),
  getCourseAttachment: (id) => {
    const call = deferred();
    videoCalls.push({ id, ...call });
    return call.promise;
  },
  getCourseComments: (args) => {
    const call = deferred();
    questionCalls.push({ args, ...call });
    return call.promise;
  },
  playCourse: async () => ({ play_count: 1 }),
};
const tick = async () => {
  for (let i = 0; i < 5; i++) {
    await Promise.resolve();
    await vue.nextTick();
  }
};
(async () => {
  const scope = vue.effectScope();
  scope.run(() => vm.runInNewContext(code, context));
  const page = context.module.exports;
  await tick();
  assert.equal(replacements.length, 1);
  assert.equal(route.query.lesson, '10');
  assert.equal(page.lesson.value.id, 10);
  assert.equal(videoCalls[0].id, 100);
  route.query = { id: '1', lesson: '11' };
  await tick();
  assert.equal(videoCalls[1].id, 101);
  assert.equal(page.videoUrl.value, '');
  videoCalls[1].resolve({ signed_url: '/video/second' });
  await tick();
  assert.equal(page.videoUrl.value, 'http://localhost/video/second');
  videoCalls[0].resolve({ signed_url: '/video/stale' });
  await tick();
  assert.equal(page.videoUrl.value, 'http://localhost/video/second');
  page.activeTab.value = 'questions';
  await tick();
  assert.equal(questionCalls[0].args.id, 1);
  route.query = { id: '2', lesson: '20' };
  await tick();
  assert.equal(page.course.value.id, 2);
  assert.equal(page.videoUrl.value, '');
  assert.equal(page.comments.value.length, 0);
  questionCalls[0].resolve({ list: [{ id: 99 }], pager: { total_rows: 1 } });
  await tick();
  assert.equal(page.comments.value.length, 0);
  page.activeTab.value = 'questions';
  await tick();
  assert.equal(questionCalls[1].args.id, 2);
  questionCalls[1].resolve({ list: [{ id: 200 }], pager: { total_rows: 1 } });
  await tick();
  assert.equal(page.comments.value[0].id, 200);
  const lastVideo = videoCalls.at(-1);
  lastVideo.reject(new Error('expired'));
  await tick();
  assert.equal(page.videoError.value, true);
  assert.equal(page.videoLoading.value, false);
  page.loadVideo();
  await tick();
  assert.equal(page.videoError.value, false);
  videoCalls.at(-1).resolve({ signed_url: 'javascript:alert(1)' });
  await tick();
  assert.equal(page.videoError.value, true);
  assert.equal(page.videoUrl.value, '');
  route.query = { id: '2', lesson: '999' };
  await tick();
  assert.equal(page.lesson.value, null);
  assert.equal(page.videoUrl.value, '');
  cleanup.forEach((fn) => fn());
  scope.stop();
  console.log(
    'Course watch checks passed: canonical links, lesson switching, stale video suppression, comment isolation, retry, unsafe URL rejection, missing lesson.',
  );
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
