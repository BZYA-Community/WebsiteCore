import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';

const read = (relativePath) =>
  fs.readFileSync(new URL(`../${relativePath}`, import.meta.url), 'utf8');

const composableSource = read('src/composables/usePostContent.ts').replace(
  /^import [\s\S]*?;\r?$/gm,
  '',
);
const composableCode = ts.transpileModule(
  `${composableSource}\nmodule.exports = { hasPostTag, isProjectPost };`,
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
const { hasPostTag, isProjectPost } = context.module.exports;

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

const projectsView = read('src/views/Projects.vue');
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
  /getProjects\(\{/,
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
assert.doesNotMatch(
  projectsView,
  /\bany\b/,
  'the Projects view must not introduce explicit any types',
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

console.log(
  'Projects checks passed: independent API, tags, search, Markdown, feature flag, and route gating.',
);
