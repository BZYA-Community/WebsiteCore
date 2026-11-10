import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import * as vue from 'vue';
import ts from 'typescript';
import { parse } from '@vue/compiler-sfc';

const deferred = () => {
  let resolve;
  let reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
};
const tick = async () => {
  for (let i = 0; i < 5; i++) {
    await Promise.resolve();
    await vue.nextTick();
  }
};
const fixture = (id = 1) => ({
  id, creator_id: 1, title: `Project ${id}`, intro: 'Intro',
  url: 'https://example.com/', images: ['https://example.com/original.png'],
  tags: ['Web'], creator: { id: 1 }, members: [], followed: false, follow_count: 0,
});
const plain = (value) => JSON.parse(JSON.stringify(value));

function editorHarness(edit) {
  const filename = edit ? 'ProjectDetail.vue' : 'Projects.vue';
  const sfc = fs.readFileSync(new URL(`../src/views/${filename}`, import.meta.url), 'utf8');
  const source = parse(sfc).descriptor.scriptSetup.content
    .replace(/^import [\s\S]*?;\r?$/gm, '')
    .replace(/import\.meta\.env\.VITE_HOST/g, "'http://localhost'");
  const exports = `draft: ${edit ? 'editDraft' : 'draft'}, showEditor, submitting, fileQueue, uploadedImages,
    imageUrlInput, addImageUrl, updateTags, finishUpload, removeUpload, linkError,
    open: ${edit ? 'openEditor' : 'startPublishing'}, submit: ${edit ? 'submitEdit' : 'submitProject'},
    ${edit ? 'project, canEdit, following, toggleFollow,' : ''}`;
  const code = ts.transpileModule(`${source}\nmodule.exports = { ${exports} };`, {
    compilerOptions: { target: ts.ScriptTarget.ES2022 },
  }).outputText;
  const calls = [];
  const followCalls = [];
  const messages = [];
  const unmount = [];
  const route = vue.reactive({ query: { id: '1' } });
  const user = vue.reactive({ userLogined: true, userInfo: { id: 1 }, allowed: true });
  user.hasPermission = () => user.allowed;
  const module = { exports: {} };
  const scope = vue.effectScope();
  scope.run(() => vm.runInNewContext(code, {
    ...vue, module, URL, Date: class extends Date { static now() { return 1; } },
    TOKEN_KEY: 'token', localStorage: { getItem: () => '' },
    onMounted() {}, onBeforeUnmount: (callback) => unmount.push(callback),
    window: {
      scrollTo() {},
      $message: Object.fromEntries(['warning', 'error', 'success'].map((level) => [level, (text) => messages.push({ level, text })])),
    },
    useI18n: () => ({ t: (key) => key }),
    useStoreMain: () => ({ theme: 'light', triggerAuth() {}, triggerAuthKey() {} }),
    useStoreUser: () => user,
    useMediaUpload: () => ({ beforeUpload: () => true }),
    useRoute: () => route, useRouter: () => ({ push() {} }),
    mdTheme: () => 'light', prepareMdRender: (value) => value,
    getProject: async (id) => ({ project: fixture(id) }),
    getProjects: async () => ({ list: [], pager: { total_rows: 0 } }),
    followProject: () => { const call = deferred(); followCalls.push(call); return call.promise; },
    unfollowProject: () => { const call = deferred(); followCalls.push(call); return call.promise; },
    createProject: (data) => { const call = deferred(); calls.push({ data: plain(data), ...call }); return call.promise; },
    updateProject: (data) => { const call = deferred(); calls.push({ data: plain(data), ...call }); return call.promise; },
  }));
  return {
    ...module.exports, calls, followCalls, messages, route, user,
    dispose() { for (const callback of unmount) callback(); scope.stop(); },
  };
}
function addURL(editor, url) {
  editor.imageUrlInput.value = url;
  editor.addImageUrl();
}
function remove(editor, file) {
  assert.equal(editor.removeUpload({ file }), true);
  // Naive UI updates its controlled queue after the remove callback accepts it.
  editor.fileQueue.value = editor.fileQueue.value.filter((item) => item.id !== file.id);
}
function validDraft(editor) {
  Object.assign(editor.draft, { title: ' Updated ', intro: ' Intro ', url: ' https://example.com/new ', tags: [' Web ', 'Web', ''] });
}
const uploadEvent = (content) => ({ target: { response: JSON.stringify({ code: 0, data: { content } }) } });

for (const locale of ['en', 'zh-CN']) {
  const errors = JSON.parse(fs.readFileSync(new URL(`../src/locales/${locale}/errors.json`, import.meta.url), 'utf8'));
  assert.ok(errors.codes.E30026, `${locale}: project update failures must have a localized message`);
}

for (const edit of [false, true]) {
  const label = edit ? 'edit' : 'create';
  const e = editorHarness(edit);
  await tick();
  e.open();
  validDraft(e);
  const initialImages = edit ? 1 : 0;
  const original = e.fileQueue.value[0];
  addURL(e, 'javascript:alert(1)');
  assert.equal(e.fileQueue.value.length, initialImages, `${label}: unsafe image URLs must be rejected`);
  assert.equal(e.messages.at(-1).text, 'project.imageUrlInvalid');
  addURL(e, '  https://example.com/first.png  ');
  e.addImageUrl();
  assert.equal(e.fileQueue.value.length, initialImages + 1, `${label}: repeated Add clicks must not append a cleared input`);
  addURL(e, 'https://example.com/second.png');
  assert.equal(new Set(e.fileQueue.value.map((file) => file.id)).size, initialImages + 2, `${label}: two URL additions in the same millisecond need distinct IDs`);
  const removed = e.fileQueue.value[initialImages];
  remove(e, removed);
  if (edit) remove(e, original);
  assert.equal(e.fileQueue.value.length, 1);

  const eight = Array.from({ length: 8 }, (_, i) => `tag-${i}`);
  e.updateTags(eight);
  e.updateTags([...eight, 'ninth']);
  assert.deepEqual(plain(e.draft.tags), eight, `${label}: selecting a ninth tag must preserve the accepted eight`);
  assert.equal(e.messages.at(-1).text, 'project.tagsMax');
  e.draft.tags = [...eight, 'ninth'];
  await e.submit();
  assert.equal(e.calls.length, 0, `${label}: submission must also enforce the tag count`);
  validDraft(e);
  e.draft.url = 'javascript:alert(1)';
  await e.submit();
  assert.equal(e.calls.length, 0, `${label}: invalid project links must not submit`);
  assert.equal(e.linkError.value, 'project.linkInvalid');
  validDraft(e);

  const pendingFile = { id: 'upload', name: 'file.png', status: 'uploading' };
  e.fileQueue.value.push(pendingFile);
  await e.submit();
  assert.equal(e.calls.length, 0, `${label}: saving must wait for every active upload`);
  const event = uploadEvent('https://example.com/upload.png');
  e.finishUpload({ file: pendingFile, event });
  e.finishUpload({ file: pendingFile, event });
  e.fileQueue.value.find((file) => file.id === pendingFile.id).status = 'finished';
  assert.equal(e.uploadedImages.value.filter((image) => image.id === pendingFile.id).length, 1, `${label}: repeated upload callbacks must not duplicate image state`);
  // Queue order is the authoritative cover order for both URL and uploaded images.
  e.fileQueue.value = [pendingFile, e.fileQueue.value[0]];
  const saving = e.submit();
  await e.submit();
  assert.equal(e.calls.length, 1, `${label}: repeated submit clicks must create one request`);
  assert.deepEqual(e.calls[0].data.images, ['https://example.com/upload.png', 'https://example.com/second.png'], `${label}: removed images must stay removed and every visible image must occur exactly once`);
  assert.deepEqual(e.calls[0].data.tags, ['Web']);
  assert.equal(e.calls[0].data.title, 'Updated');
  const queued = e.fileQueue.value.length;
  addURL(e, 'https://example.com/while-saving.png');
  e.open();
  assert.equal(e.fileQueue.value.length, queued, `${label}: reopening or adding while saving must not reset the active draft`);
  e.calls[0].resolve(edit ? { ...fixture(), title: 'Updated', images: e.calls[0].data.images } : fixture());
  await saving;
  assert.equal(e.showEditor.value, false);
  assert.equal(e.submitting.value, false);
  assert.equal(e.fileQueue.value.length, 0);
  if (edit) assert.equal(e.project.value.title, 'Updated');
  e.dispose();

  const limit = editorHarness(edit);
  await tick();
  limit.open();
  for (let i = initialImages; i < 6; i++) addURL(limit, `https://example.com/${i}.png`);
  addURL(limit, 'https://example.com/seventh.png');
  assert.equal(limit.fileQueue.value.length, 6, `${label}: URL and existing/uploaded images share the six-image cap`);
  assert.equal(limit.messages.at(-1).text, 'project.imagesMax');
  limit.dispose();

  const closed = editorHarness(edit);
  await tick();
  closed.open();
  const late = { id: 'cancelled', status: 'uploading' };
  closed.fileQueue.value.push(late);
  addURL(closed, 'https://example.com/cancelled.png');
  closed.draft.title = 'Discard me';
  closed.showEditor.value = false;
  assert.equal(closed.fileQueue.value.length, 0, `${label}: dismissal must clear image state immediately`);
  closed.open();
  closed.finishUpload({ file: late, event: uploadEvent('https://example.com/late.png') });
  assert.equal(closed.uploadedImages.value.some((image) => image.id === late.id), false, `${label}: callbacks from dismissed uploads must not leak into a reopened editor`);
  assert.equal(closed.draft.title, edit ? 'Project 1' : '');
  closed.dispose();

  const failed = editorHarness(edit);
  await tick();
  failed.open();
  validDraft(failed);
  if (!edit) addURL(failed, 'https://example.com/image.png');
  const rejected = failed.submit();
  failed.calls[0].reject(new Error('offline'));
  await rejected; // UI event handlers must consume expected API failures after the interceptor reports them.
  assert.equal(failed.messages.some((message) => message.level === 'success'), false);
  assert.equal(failed.submitting.value, false, `${label}: failed requests must unlock retry`);
  assert.equal(failed.showEditor.value, true);
  assert.equal(failed.draft.title, ' Updated ', `${label}: failed requests must preserve the draft`);
  const retry = failed.submit();
  assert.equal(failed.calls.length, 2);
  failed.dispose();
  failed.calls[1].resolve(fixture());
  await retry;
  assert.equal(failed.messages.some((message) => message.level === 'success'), false, `${label}: an unmounted editor must ignore an old successful response`);
}

const followRace = editorHarness(true);
await tick();
const pendingFollow = followRace.toggleFollow();
followRace.open();
assert.equal(followRace.showEditor.value, false, 'editing must wait for an in-flight follow so a delayed save cannot restore stale follow state');
followRace.followCalls[0].resolve({ status: true, follow_count: 1 });
await pendingFollow;
followRace.open();
assert.equal(followRace.showEditor.value, true);
await followRace.toggleFollow();
assert.equal(followRace.followCalls.length, 1, 'follow state must not change underneath an open edit dialog');
followRace.dispose();

const navigation = editorHarness(true);
await tick();
navigation.open();
validDraft(navigation);
const oldSave = navigation.submit();
navigation.route.query = { id: '2' };
await tick();
assert.equal(navigation.showEditor.value, false, 'navigation must dismiss the editor for the previous project');
assert.equal(navigation.project.value.id, 2);
navigation.open();
validDraft(navigation);
const newSave = navigation.submit();
navigation.calls[0].resolve({ ...fixture(1), title: 'Stale' });
await oldSave;
assert.equal(navigation.project.value.id, 2, 'an older edit response must not replace the newly opened project');
assert.equal(navigation.submitting.value, true, 'an older edit response must not unlock a newer save');
assert.equal(navigation.showEditor.value, true);
navigation.calls[1].resolve({ ...fixture(2), title: 'Saved 2' });
await newSave;
assert.equal(navigation.project.value.title, 'Saved 2');
navigation.route.query = { id: '1' };
await tick();
assert.equal(navigation.project.value.id, 1, 'back/forward project navigation must load the current route rather than a stale edit result');
navigation.open();
navigation.showEditor.value = false;
navigation.user.userInfo.id = 99;
navigation.open();
assert.equal(navigation.showEditor.value, false, 'nonowners must not open the edit dialog');
navigation.user.userInfo.id = 1;
navigation.user.allowed = false;
navigation.open();
assert.equal(navigation.showEditor.value, false, 'edit availability must honor the API permission requirements');
navigation.dispose();

console.log('Project editor checks passed: URL add/remove/order/limits, eight-tag limit, upload and repeated-submit guards, cancel/reopen, retry, and stale navigation/unmount responses.');
