import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';

const read = (relativePath) =>
  fs.readFileSync(new URL(`../${relativePath}`, import.meta.url), 'utf8');

const helperSource = read('src/utils/external-link.ts').replace(
  /^import [\s\S]*?;\r?$/gm,
  '',
);
const helperCode = ts.transpileModule(
  `${helperSource}\nmodule.exports = { normalizeExternalUrl, normalizeTrustedDomain, isTrustedExternalUrl, buildExternalWarningRoute };`,
  {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
    },
  },
).outputText;
const module = { exports: {} };
const context = { module, exports: module.exports, URL };
vm.runInNewContext(helperCode, context);
const {
  normalizeExternalUrl,
  normalizeTrustedDomain,
  isTrustedExternalUrl,
  buildExternalWarningRoute,
} = context.module.exports;

const origin = 'https://community.example';
assert.equal(
  normalizeExternalUrl('https://docs.example/path?q=1', origin),
  'https://docs.example/path?q=1',
  'cross-origin HTTPS links must be accepted',
);
assert.equal(
  normalizeExternalUrl('http://docs.example/path', origin),
  'http://docs.example/path',
  'cross-origin HTTP links must be accepted',
);
for (const value of [
  '',
  '/relative',
  '#fragment',
  'https://community.example/account',
  'javascript:alert(1)',
  'data:text/html,unsafe',
  'mailto:user@example.com',
]) {
  assert.equal(
    normalizeExternalUrl(value, origin),
    '',
    `${value || 'empty URL'} must not be treated as a safe external destination`,
  );
}
assert.equal(
  isTrustedExternalUrl('http://github.com/openai', ['github.com']),
  false,
  'insecure HTTP links must still show the warning',
);
assert.equal(
  buildExternalWarningRoute('http://github.com/openai', origin, ['github.com'])?.name,
  'external-link',
);

const warningRoute = buildExternalWarningRoute(
  'https://docs.example/guide',
  origin,
);
assert.equal(warningRoute?.name, 'external-link');
assert.equal(warningRoute?.query?.url, 'https://docs.example/guide');
assert.equal(
  buildExternalWarningRoute('javascript:alert(1)', origin),
  null,
  'unsafe destinations must not produce a warning route',
);

assert.equal(normalizeTrustedDomain('*.GitHub.COM.'), 'github.com');
assert.equal(normalizeTrustedDomain('https://github.com'), '');
for (const value of [
  'https://github.com/openai',
  'https://www.github.com/openai',
  'https://gist.github.com/example',
  'https://github.com.:8443/example',
]) {
  assert.equal(isTrustedExternalUrl(value, ['github.com']), true, `${value} must be trusted`);
  assert.equal(
    buildExternalWarningRoute(value, origin, ['github.com']),
    null,
    `${value} must skip the warning route`,
  );
}
for (const value of [
  'https://evilgithub.com',
  'https://github.com.evil.example',
  'https://github.com@evil.example',
]) {
  assert.equal(isTrustedExternalUrl(value, ['github.com']), false, `${value} must not be trusted`);
  assert.equal(
    buildExternalWarningRoute(value, origin, ['github.com'])?.name,
    'external-link',
    `${value} must keep the warning route`,
  );
}

const router = read('src/router/index.ts');
assert.match(router, /path:\s*['"]\/external-link['"]/);
assert.match(router, /name:\s*['"]external-link['"]/);
assert.match(
  router,
  /external-link[\s\S]{0,300}return true/,
  'the public safety page must not depend on session/profile loading',
);

const app = read('src/App.vue');
assert.match(app, /['"]click['"][\s\S]{0,120}addEventListener\(event,\s*protectEventAnchor,\s*true\)/);
assert.match(app, /removeEventListener\(event,\s*protectEventAnchor,\s*true\)/);
assert.match(
  app,
  /resolveExternalWarningHref/,
  'global anchor interception must use the shared warning helper',
);
assert.match(app, /event\.composedPath\(\)/, 'Shadow DOM links must be discovered from the composed event path');
assert.match(app, /new MutationObserver\(/, 'ordinary anchors must expose the warning route as their real href');
assert.match(app, /anchor\.href\s*=\s*warningHref/, 'untrusted anchors must be rewritten to the local warning route');
assert.doesNotMatch(app, /event\.(?:preventDefault|stopImmediatePropagation)\(/, 'native target and modifier-key behavior must be preserved');

const profileStore = read('src/store/profile.ts');
assert.match(profileStore, /trustedExternalDomains:\s*\['github\.com'\]/);
assert.match(profileStore, /data\.trusted_external_domains/);

const adminSettings = read('src/views/AdminSettings.vue');
assert.match(adminSettings, /type === ['"]string_list['"]/);
assert.match(adminSettings, /n-dynamic-tags/);

const warningView = read('src/views/ExternalLink.vue');
assert.match(warningView, /externalLink\.warning/);
assert.match(warningView, /window\.location\.assign\(/);
assert.match(warningView, /router\.(?:back|push)\(/);

const zh = JSON.parse(read('src/locales/zh-CN/externalLink.json'));
const en = JSON.parse(read('src/locales/en/externalLink.json'));
assert.equal(
  zh.warning,
  '您即将离开社区，请注意您的账号和财产安全',
  'the requested Chinese warning must remain exact',
);
assert.equal(typeof en.warning, 'string');
assert.ok(en.warning.length > 0, 'the English warning must be present');

for (const relativePath of [
  'src/components/project-card.vue',
  'src/components/post-item.vue',
  'src/components/post-detail.vue',
  'src/utils/content.ts',
  'src/components/post-attachment.vue',
  'src/views/CourseDetail.vue',
]) {
  const source = read(relativePath);
  assert.match(
    source,
    /@\/utils\/external-link/,
    `${relativePath} must send programmatic external navigation through the shared helper`,
  );
}

for (const relativePath of ['src/components/compose.vue', 'src/views/ComposeMd.vue']) {
  const source = read(relativePath);
  assert.match(
    source,
    /if \(!openExternalLink\([\s\S]{0,220}window\.location\.assign\(url\)/,
    `${relativePath} must directly navigate when a programmatic destination is trusted`,
  );
}

assert.doesNotMatch(
  read('src/utils/content.ts'),
  /window\.location\.assign\(\s*`?https?:/,
  'the easter-egg redirect must not bypass the safety page',
);

console.log(
  'External-link checks passed: safe URL validation, public warning route, global interception, translations, and programmatic entry points.',
);
