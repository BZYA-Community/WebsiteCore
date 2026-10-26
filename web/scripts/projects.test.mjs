import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import * as vue from 'vue';
import ts from 'typescript';
import { compileStyle, parse } from '@vue/compiler-sfc';

const read = (relativePath) =>
  fs.readFileSync(new URL(`../${relativePath}`, import.meta.url), 'utf8');

const composableSource = read('src/composables/usePostContent.ts').replace(
  /^import [\s\S]*?;\r?$/gm,
  '',
);
const composableCode = ts.transpileModule(
  `${composableSource}\nmodule.exports = { hasPostTag, isProjectPost, usePostContent };`,
  {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  },
).outputText;
const module = { exports: {} };
const context = { module, exports: module.exports, computed: (value) => value };
vm.runInNewContext(composableCode, context);
const { hasPostTag, isProjectPost, usePostContent } = context.module.exports;

assert.equal(
  hasPostTag({ tags: 'news, 项目' }, '项目'),
  true,
  'comma-separated project tags must be detected',
);
assert.equal(
  hasPostTag({ tags: { 项目: 1 } }, '项目'),
  true,
  'tag maps must be detected',
);
assert.equal(
  isProjectPost({ tags: '项目' }, true),
  true,
  'enabled projects must use project presentation',
);
assert.equal(
  isProjectPost({ tags: '项目' }, false),
  false,
  'disabled projects must use ordinary post presentation',
);

const parsedLinks = usePostContent({
  contents: [
    { id: 1, type: 6, content: 'project:42' },
    { id: 2, type: 6, content: 'https://example.com/project' },
    { id: 3, type: 6, content: 'project:not-an-id' },
  ],
}).get().links;
assert.equal(
  parsedLinks.map(({ content }) => content).join(','),
  'https://example.com/project,project:not-an-id',
  'internal project markers must stay hidden while ordinary links remain visible',
);

const projectsView = read('src/views/Projects.vue');
const projectDetailView = read('src/views/ProjectDetail.vue');
const projectCard = read('src/components/project-card.vue');
const collectionView = read('src/views/Collection.vue');
const projectApi = read('src/api/project.ts');
const adminAuditView = read('src/views/AdminAudit.vue');
const postDetailView = read('src/components/post-detail.vue');
const postItemView = read('src/components/post-item.vue');
const projectDownMigration = read('../scripts/migration/postgres/0034_project_module.down.sql');
const projectCardSfc = parse(projectCard).descriptor;
const projectCardStyle = projectCardSfc.styles[0];
const projectCardCss = compileStyle({
  source: projectCardStyle.content,
  filename: 'src/components/project-card.vue',
  id: 'data-v-project-card-test',
  scoped: projectCardStyle.scoped,
  preprocessLang: projectCardStyle.lang,
}).code;
assert.match(
  projectCardCss,
  /\.dark \.project-card\s*\{/,
  'shared project cards must receive a dark background in every host view',
);
assert.match(
  projectsView,
  /:key="project\.id"/,
  'project cards must use the independent project id as their stable key',
);
assert.match(
  projectsView,
  /createProject\(/,
  'project publishing must use the independent project API',
);
assert.match(
  projectsView,
  /tags:\s*draft\.tags/,
  'project publishing must submit project-specific tags',
);
assert.match(
  projectsView,
  /getProjects\(/,
  'project listing must use the independent project API',
);
assert.match(
  projectsView,
  /PROJECT_TITLE_MAX_LENGTH\s*=\s*30/,
  'project names must be limited to 30 characters',
);
assert.match(
  projectsView,
  /keyword:\s*keyword\.value/,
  'project listing must support name search',
);
assert.match(
  projectsView,
  /tag:\s*tagFilter\.value/,
  'project listing must support project tag search',
);
assert.match(
  projectsView,
  /MdEditor/,
  'project introductions must use the Markdown editor',
);
assert.match(
  projectsView,
  /<project-card/,
  'the project directory must use the shared project card',
);
assert.doesNotMatch(
  projectsView,
  /\bany\b/,
  'the Projects view must not introduce explicit any types',
);
assert.match(
  projectsView,
  /const revision = \+\+projectRequestRevision;[\s\S]{0,300}const params = \{[\s\S]{0,300}await getProjects\(params\);[\s\S]{0,100}if \(revision !== projectRequestRevision\) return;/,
  'project listing must snapshot request parameters and ignore stale responses',
);
assert.match(
  projectsView,
  /finally \{\s*if \(revision === projectRequestRevision\) loading\.value = false;/,
  'stale project requests must not clear the current loading state',
);
assert.match(
  projectsView,
  /resetDraft\(\);\s*showEditor\.value = true;/,
  'opening the project editor must start with a clean draft',
);

const projectEditorSource = parse(projectsView).descriptor.scriptSetup.content
  .replace(/^import [\s\S]*?;\r?$/gm, '')
  .replace(/import\.meta\.env\.VITE_HOST/g, "'http://localhost'");
const projectEditorCode = ts.transpileModule(
  `${projectEditorSource}\nmodule.exports = { draft, showEditor, fileQueue, uploadedImages, startPublishing, updateUpload, finishUpload, submitProject };`,
  { compilerOptions: { target: ts.ScriptTarget.ES2022 } },
).outputText;
const projectCreateCalls = [];
const projectMessages = [];
const projectEditorModule = { exports: {} };
const projectEditorContext = {
  ...vue,
  module: projectEditorModule,
  TOKEN_KEY: 'token',
  localStorage: { getItem: () => '' },
  window: {
    scrollTo() {},
    $message: {
      error() {},
      success() {},
      warning: (message) => projectMessages.push(message),
    },
  },
  useI18n: () => ({ t: (key) => key }),
  useStoreMain: () => ({ theme: 'light', triggerAuth() {}, triggerAuthKey() {} }),
  useStoreUser: () => ({ userLogined: true, userInfo: { id: 1 }, hasPermission: () => true }),
  useMediaUpload: () => ({ beforeUpload() {} }),
  mdTheme: () => 'light',
  getProjects: async () => ({ list: [], pager: { total_rows: 0 } }),
  createProject: async (data) => projectCreateCalls.push(data),
  onMounted() {},
};
vm.runInNewContext(projectEditorCode, projectEditorContext);
const projectEditor = projectEditorModule.exports;
projectEditor.startPublishing();
Object.assign(projectEditor.draft, {
  title: 'Old project',
  intro: 'Old intro',
  url: 'https://example.com/old',
  tags: ['old'],
});
const oldFile = { id: 'old-image', name: 'old.png', status: 'finished' };
const oldUploadEvent = {
  target: { response: JSON.stringify({ code: 0, data: { content: 'https://example.com/old.png' } }) },
};
projectEditor.updateUpload([oldFile]);
projectEditor.finishUpload({ file: oldFile, event: oldUploadEvent });
assert.equal(projectEditor.uploadedImages.value.length, 1);

projectEditor.showEditor.value = false;
projectEditor.startPublishing();
assert.deepEqual(
  { ...projectEditor.draft, tags: [...projectEditor.draft.tags] },
  { title: '', intro: '', url: '', tags: [] },
  'reopening the project editor must discard the cancelled draft',
);
assert.equal(projectEditor.fileQueue.value.length, 0);
assert.equal(projectEditor.uploadedImages.value.length, 0);
projectEditor.finishUpload({ file: oldFile, event: oldUploadEvent });
assert.equal(projectEditor.uploadedImages.value.length, 0, 'a cancelled upload must not reappear after reopening');

Object.assign(projectEditor.draft, {
  title: 'New project',
  intro: 'New intro',
  url: 'https://example.com/new',
});
await projectEditor.submitProject();
assert.equal(projectCreateCalls.length, 0, 'hidden images from a cancelled draft must never be submitted');
assert.equal(projectMessages.at(-1), 'project.imagesRequired');

assert.match(
  projectDetailView,
  /v-else-if="loadFailed"[^>]*status="error"[\s\S]{0,160}@click="loadProject"/,
  'project detail request failures must offer a retry action',
);

const projectDetailSource = parse(projectDetailView).descriptor.scriptSetup.content
  .replace(/^import [\s\S]*?;\r?$/gm, '');
const projectDetailCode = ts.transpileModule(
  `${projectDetailSource}\nmodule.exports = { project, loading, following, loadFailed, notFound, loadProject, toggleFollow };`,
  { compilerOptions: { target: ts.ScriptTarget.ES2022 } },
).outputText;
const deferred = () => {
  let resolve;
  let reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
};
const detailRoute = vue.reactive({ query: { id: '1' } });
const detailCalls = [];
const followCalls = [];
const projectFixture = (id, followed = false, followCount = id) => ({
  id,
  post_id: id * 10,
  creator: { id: 1, username: 'owner', nickname: 'Owner', avatar: '' },
  title: `Project ${id}`,
  intro: 'Intro',
  url: 'https://example.com',
  images: [],
  tags: ['custom'],
  members: [],
  followed,
  follow_count: followCount,
});
const projectDetailModule = { exports: {} };
const projectDetailContext = {
  ...vue,
  module: projectDetailModule,
  window: { $message: { warning() {}, info() {} } },
  useI18n: () => ({ t: (key) => key }),
  useRoute: () => detailRoute,
  useRouter: () => ({ push() {} }),
  useStoreMain: () => ({ theme: 'light', triggerAuth() {}, triggerAuthKey() {} }),
  useStoreUser: () => ({ userLogined: true, hasPermission: () => true }),
  mdTheme: () => 'light',
  prepareMdRender: (value) => value,
  getProject: (id) => {
    const call = deferred();
    detailCalls.push({ id, ...call });
    return call.promise;
  },
  followProject: (id) => {
    const call = deferred();
    followCalls.push({ id, ...call });
    return call.promise;
  },
  unfollowProject: (id) => {
    const call = deferred();
    followCalls.push({ id, ...call });
    return call.promise;
  },
};
const tick = async () => {
  for (let i = 0; i < 5; i++) {
    await Promise.resolve();
    await vue.nextTick();
  }
};
const detailScope = vue.effectScope();
detailScope.run(() => vm.runInNewContext(projectDetailCode, projectDetailContext));
const projectDetailPage = projectDetailModule.exports;
await tick();
assert.equal(detailCalls[0].id, 1);
detailRoute.query = { id: '2' };
await tick();
assert.equal(detailCalls[1].id, 2);
detailCalls[1].resolve({ project: projectFixture(2) });
await tick();
assert.equal(projectDetailPage.project.value.id, 2);
detailCalls[0].resolve({ project: projectFixture(1) });
await tick();
assert.equal(projectDetailPage.project.value.id, 2, 'an older detail response must not overwrite the current project');

const pendingFollow = projectDetailPage.toggleFollow();
await tick();
assert.equal(followCalls[0].id, 2);
detailRoute.query = { id: '3' };
await tick();
detailCalls[2].resolve({ project: projectFixture(3, false, 3) });
await tick();
followCalls[0].resolve({ status: true, follow_count: 99 });
await pendingFollow;
assert.equal(projectDetailPage.project.value.id, 3);
assert.equal(projectDetailPage.project.value.followed, false, 'a completed follow must not mutate a project opened later');
assert.equal(projectDetailPage.project.value.follow_count, 3);

detailRoute.query = { id: '4' };
await tick();
detailCalls[3].reject({ code: 30021 });
await tick();
assert.equal(projectDetailPage.notFound.value, true, 'project-not-found errors must render the 404 state');
assert.equal(projectDetailPage.loadFailed.value, false);
detailRoute.query = { id: '5' };
await tick();
detailCalls[4].reject({ code: 30022 });
await tick();
assert.equal(projectDetailPage.notFound.value, false);
assert.equal(projectDetailPage.loadFailed.value, true, 'other request errors must remain retryable');
const retry = projectDetailPage.loadProject();
assert.equal(projectDetailPage.loadFailed.value, false);
detailCalls[5].resolve({ project: projectFixture(5) });
await retry;
assert.equal(projectDetailPage.project.value.id, 5, 'retry must recover the project detail');
assert.equal(projectDetailPage.loading.value, false);
detailScope.stop();

for (const loader of ['loadPosts', 'loadComments', 'loadNicknames', 'loadAvatars', 'loadLogs']) {
  const body = adminAuditView.match(
    new RegExp(`const ${loader} = async \\(\\) => \\{([\\s\\S]*?)\\n\\};`),
  )?.[1];
  assert.ok(body, `${loader} must exist`);
  assert.match(body, /const revision = \+\+listRequestRevision;/, `${loader} must supersede older audit requests`);
  assert.match(body, /const params = \{/, `${loader} must snapshot its request parameters`);
  assert.match(body, /await[\s\S]*if \(revision !== listRequestRevision\) return;/, `${loader} must ignore stale responses`);
  assert.match(body, /finally \{[\s\S]*if \(revision === listRequestRevision\) loading\.value = false;/, `${loader} must not clear loading for a newer request`);
}

for (const field of [
  /selectedProject\.title/,
  /v-for="tag in selectedProject\.tags"/,
  /v-for="\(image, index\) in selectedProject\.images"/,
  /prepareMdRender\(selectedProject\.intro\)/,
  /selectedProject\.url/,
]) {
  assert.match(adminAuditView, field, 'the review dialog must render the complete project and its custom tags');
}

assert.match(
  projectApi,
  /followed\?:\s*boolean/,
  'the project API must support querying only projects followed by the viewer',
);
assert.match(
  collectionView,
  /name="posts"[\s\S]{0,220}name="projects"/,
  'favorites must expose separate saved-post and followed-project tabs',
);
assert.match(
  collectionView,
  /getProjects\(\{[\s\S]{0,160}followed:\s*true/,
  'the followed-project tab must request only projects followed by the viewer',
);
assert.match(
  collectionView,
  /v-if="projectsEnabled"/,
  'the followed-project tab must honor the projects feature flag',
);
assert.match(
  collectionView,
  /<project-card[^>]*v-for="project in projectList"[^>]*:key="project\.id"/,
  'favorites must render followed projects through the shared card with stable ids',
);
assert.match(
  collectionView,
  /postLoadFailed\.value\s*=\s*true/,
  'favorites must distinguish a failed initial post request from an empty collection',
);
assert.match(
  collectionView,
  /requestedPage\s*>\s*lastPage[\s\S]{0,120}projectPage\.value\s*=\s*lastPage[\s\S]{0,120}loadProjects\(\)/,
  'followed projects must recover when the current page becomes out of range',
);
assert.match(
  projectCard,
  /router\.push\(\{\s*name:\s*['"]project['"]/,
  'shared project cards must open the project detail route',
);
assert.match(
  projectCard,
  /@\/utils\/external-link/,
  'shared project cards must preserve outbound-link warnings',
);
assert.match(
  postDetailView,
  /v-if="projectID"[^>]*@click\.stop="goProject"/,
  'the view-project button must not bubble into the post detail navigation',
);
for (const [name, source] of [
  ['post detail', postDetailView],
  ['post item', postItemView],
]) {
  assert.match(
    source,
    /for \(const content of post\.value\.contents \|\| \[\]\)[\s\S]{0,140}\+content\.type === 6[\s\S]{0,140}\/\^project:\(\\d\+\)\$\//,
    `${name} must resolve its project id from the hidden raw marker`,
  );
}

const markerCleanupIndex = projectDownMigration.search(/DELETE FROM p_post_content/i);
const projectDropIndex = projectDownMigration.search(/DROP TABLE IF EXISTS p_project;/i);
assert.ok(
  markerCleanupIndex >= 0 && markerCleanupIndex < projectDropIndex,
  'project markers must be removed before the project table is dropped',
);
assert.match(
  projectDownMigration,
  /USING p_project AS project[\s\S]*content\.post_id = project\.post_id[\s\S]*content\.type = 6[\s\S]*content\.content = 'project:' \|\| project\.id/i,
  'rollback cleanup must target only the marker belonging to each project post',
);

for (const relativePath of [
  'src/components/post-item.vue',
  'src/components/post-detail.vue',
  'src/views/Post.vue',
]) {
  const source = read(relativePath);
  assert.match(
    source,
    /isProjectPost\([^\n]+projectsEnabled\)/,
    `${relativePath} must honor projects.enabled`,
  );
}

const router = read('src/router/index.ts');
const sidebar = read('src/components/sidebar.vue');
assert.match(
  router,
  /\['projects', 'project'\][^\n]+projectsEnabled/,
  'the project list and detail routes must be disabled by the profile flag',
);
assert.match(
  sidebar,
  /option\.key !== 'projects'[^\n]+projectsEnabled/,
  'the Projects navigation item must be disabled by the profile flag',
);
assert.match(
  sidebar,
  /route\.name === 'project'[^\n]+return 'projects'/,
  'the project detail route must keep the Projects navigation item selected',
);
const i18nTypes = read('src/types/vue-i18n.d.ts');
assert.match(
  i18nTypes,
  /import type externalLink[\s\S]*externalLink: typeof externalLink/,
  'the external-link locale namespace must participate in typed messages',
);

console.log(
  'Projects checks passed: independent API, tags, search, Markdown, feature flag, and route gating.',
);
