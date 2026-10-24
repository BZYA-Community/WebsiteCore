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
  /:key="project\.post\.id"/,
  'project cards must use the post id as their stable key',
);
assert.match(
  projectsView,
  /PostItemTypeEnum\.TITLE/,
  'project publishing must keep the title separate',
);
assert.match(
  projectsView,
  /PostItemTypeEnum\.TEXT/,
  'project publishing must keep the introduction separate',
);
assert.match(
  projectsView,
  /PostItemTypeEnum\.LINKURL/,
  'project publishing must include its external link',
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
  /to\.name === 'projects'[^\n]+projectsEnabled/,
  'the Projects route must be disabled by the profile flag',
);
assert.match(
  sidebar,
  /option\.key !== 'projects'[^\n]+projectsEnabled/,
  'the Projects navigation item must be disabled by the profile flag',
);

console.log(
  'Projects checks passed: tags, feature flag, stable keys, structured content, route and navigation gating.',
);
