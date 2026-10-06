import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import * as vue from 'vue';
import ts from 'typescript';
const source = fs
  .readFileSync(
    new URL('../src/components/course/course-lessons.vue', import.meta.url),
    'utf8',
  )
  .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1]
  .replace(/^import .*;\r?$/gm, '');
const code = ts.transpileModule(
  source +
    '\nmodule.exports = {lessons,loading,canManage,canView,draft,primaryVideo,materials,editLesson,editor,busy,error,saveLesson,uploadFiles,setPrimaryVideo};',
  { compilerOptions: { target: ts.ScriptTarget.ES2022 } },
).outputText;
const props = vue.reactive({
  course: { id: 1, teacher_id: 8 },
  managing: false,
});
const user = vue.reactive({
  permissions: [],
  userInfo: { id: 8, contact_verified: false },
  userLogined: false,
  hasPermission(permission) {
    return this.permissions.includes(permission);
  },
});
const video = (id) => ({
  id,
  attachment_id: id,
  name: `${id}.mp4`,
  kind: 'resource',
  file_size: 100,
  mime_type: 'video/mp4',
});
const pdf = (id) => ({
  id,
  attachment_id: id,
  name: `${id}.pdf`,
  kind: 'attachment',
  file_size: 100,
  mime_type: 'application/pdf',
});
const lesson = {
  id: 10,
  course_id: 1,
  title: 'First',
  intro: 'Introduction',
  sort: 5,
  attachments: [pdf(1), video(2), video(3)],
};
const calls = { list: [], uploads: [], saves: [] };
let failSave = false,
  failUpload = false,
  pauseUpload;
const cleanup = [];
const context = {
  ...vue,
  AbortController,
  module: { exports: {} },
  window: { $message: { success() {} } },
  onBeforeUnmount: (fn) => cleanup.push(fn),
  defineProps: () => props,
  withDefaults: (value) => value,
  defineEmits: () => () => {},
  useI18n: () => ({ t: (key) => key }),
  useStoreUser: () => user,
  useStoreMain: () => ({ triggerAuthKey() {}, triggerAuth() {} }),
  useStoreProfile: () => ({
    profile: {
      uploadLimits: {
        course_attachment_max_bytes: 50 * 1024 ** 2,
        course_resource_max_bytes: 2 * 1024 ** 3,
      },
    },
  }),
  getCourseLessons: async (id) => {
    calls.list.push(id);
    return { lessons: [lesson] };
  },
  uploadCourseFile: async (file, kind, signal) => {
    calls.uploads.push({ file, kind, signal });
    if (pauseUpload) await pauseUpload;
    if (failUpload) throw Error('Upload failed');
    return { ...video(40 + calls.uploads.length), name: file.name };
  },
  updateCourseLesson: async (data) => {
    calls.saves.push(data);
    if (failSave) throw Error('Save failed');
  },
  createCourseLesson: async (data) => {
    calls.saves.push(data);
    if (failSave) throw Error('Save failed');
  },
  deleteCourseLesson: async () => {},
};
const tick = async () => {
  for (let i = 0; i < 5; i++) {
    await Promise.resolve();
    await vue.nextTick();
  }
};
const event = (file) => ({ target: { value: 'selected', files: [file] } });
(async () => {
  const scope = vue.effectScope();
  scope.run(() => vm.runInNewContext(code, context));
  const page = context.module.exports;
  await tick();
  assert.equal(
    calls.list.length,
    0,
    'guests must not load restricted lesson resources',
  );
  user.permissions = ['course.view', 'course.manage_own', 'course.upload'];
  user.userLogined = true;
  await tick();
  assert.equal(calls.list.length, 1);
  assert.equal(
    page.canManage.value,
    false,
    'student mode must hide edit controls',
  );
  props.managing = true;
  await tick();
  assert.equal(page.canManage.value, true);
  props.course.teacher_id = 9;
  assert.equal(page.canManage.value, false);
  props.course.teacher_id = 8;
  page.editLesson(lesson);
  assert.equal(page.editor.value, true);
  assert.equal(page.primaryVideo.value.attachment_id, 2);
  assert.equal(page.materials.value.length, 2);
  page.draft.title = 'Updated';
  assert.equal(lesson.title, 'First', 'editing must not mutate loaded content');
  await page.uploadFiles(
    event({ name: 'replacement.mp4', type: 'video/mp4', size: 100 }),
    'video',
  );
  assert.equal(calls.uploads[0].kind, 'resource');
  assert.deepEqual(
    Array.from(page.draft.attachments, (file) => file.attachment_id),
    [1, 41, 3],
    'replacement must keep PDF and second video',
  );
  page.setPrimaryVideo(page.draft.attachments[2]);
  assert.deepEqual(
    Array.from(page.draft.attachments, (file) => file.attachment_id),
    [3, 1, 41],
    'changing primary must retain both videos',
  );
  await page.uploadFiles(
    event({ name: 'fake.txt', type: 'text/plain', size: 100 }),
    'video',
  );
  assert.equal(
    calls.uploads.length,
    1,
    'non-video must not be uploaded as a primary video',
  );
  await page.uploadFiles(
    event({
      name: 'big.zip',
      type: 'application/zip',
      size: 50 * 1024 ** 2 + 1,
    }),
    'attachment',
  );
  assert.equal(
    calls.uploads.length,
    1,
    'attachment limit enforced before request',
  );
  failSave = true;
  await page.saveLesson();
  assert.equal(page.editor.value, true);
  assert.equal(page.draft.title, 'Updated');
  assert.equal(
    page.draft.attachments.length,
    3,
    'failed saves retain all draft materials',
  );
  failUpload = true;
  await page.uploadFiles(
    event({ name: 'failed.mp4', type: 'video/mp4', size: 100 }),
    'video',
  );
  assert.equal(
    page.primaryVideo.value.attachment_id,
    3,
    'failed replacement must retain current video',
  );
  failUpload = false;
  let finishUpload;
  pauseUpload = new Promise((resolve) => {
    finishUpload = resolve;
  });
  const pending = page.uploadFiles(
    event({ name: 'cancelled.mp4', type: 'video/mp4', size: 100 }),
    'video',
  );
  await tick();
  assert.equal(page.busy.value, true);
  const savedCount = calls.saves.length;
  await page.saveLesson();
  assert.equal(
    calls.saves.length,
    savedCount,
    'saving during upload is blocked',
  );
  props.managing = false;
  await tick();
  assert.equal(calls.uploads.at(-1).signal.aborted, true);
  finishUpload();
  await pending;
  assert.equal(
    page.primaryVideo.value.attachment_id,
    3,
    'cancelled upload must not replace current video',
  );
  pauseUpload = undefined;
  props.managing = true;
  await tick();
  page.editLesson(lesson);
  failSave = false;
  await page.saveLesson();
  assert.equal(page.editor.value, false);
  assert.equal(
    calls.saves.at(-1).attachments.length,
    3,
    'unchanged videos preserved on metadata save',
  );
  user.permissions = [];
  await tick();
  assert.equal(page.lessons.value.length, 0);
  const lastCount = calls.list.length;
  await tick();
  assert.equal(calls.list.length, lastCount);
  cleanup.forEach((fn) => fn());
  scope.stop();
  console.log(
    'Course editor checks passed: access gating, ownership, student mode, primary video replacement, multi-video preservation, MIME and size rejection, failed-save retention, cancellation, busy save guard.',
  );
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
