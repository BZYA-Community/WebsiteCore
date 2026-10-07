import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import ts from 'typescript';
import { hasPermission, hasAnyPermission, isAssignableIdentityGroup } from '../src/utils/permissions.ts';

assert.equal(hasPermission({}, 'user.manage'), false);
assert.equal(hasPermission({ is_admin: true, roles: ['admin'] }, 'user.manage'), false);
assert.equal(hasPermission({ permissions: ['post.view'] }, 'post.create'), false);
assert.equal(hasPermission({ permissions: ['post.view', 'post.create'] }, 'post.create'), true);
assert.equal(hasAnyPermission({ permissions: ['identity.manage'] }, ['user.manage', 'identity.manage']), true);
assert.equal(hasAnyPermission({ permissions: ['post.view'] }, ['user.manage', 'identity.manage']), false);
assert.equal(hasPermission({ is_operator: true }, 'identity.manage'), true);
assert.deepEqual(['guest', 'member', 'admin', 'instructor'].map((key) => ({ key })).filter(isAssignableIdentityGroup).map(({ key }) => key), ['admin', 'instructor']);

// Exercise the actual Pinia store with controllable API promises; no browser or server is needed.
const require = createRequire(import.meta.url);
const { createPinia, setActivePinia } = require('pinia');
const storage = new Map();
const localStorage = { getItem: (key) => storage.get(key) ?? null, removeItem: (key) => storage.delete(key) };
let resolveUser, rejectUser;
const modules = {
  '@/api/auth': { userInfo: () => new Promise((resolve, reject) => { resolveUser = resolve; rejectUser = reject; }) },
  '@/api/identity': { getIdentity: async () => ({ permissions: ['post.view'] }) },
  '@/utils/permissions': { hasPermission, hasAnyPermission },
};
const source = readFileSync(new URL('../src/store/user.ts', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
const module = { exports: {} };
new Function('require', 'module', 'exports', 'localStorage', compiled)(
  (name) => modules[name] ?? require(name), module, module.exports, localStorage,
);
const { useStoreUser, TOKEN_KEY } = module.exports;

setActivePinia(createPinia());
const signedOut = useStoreUser();
storage.set(TOKEN_KEY, 'old-session');
const pendingLogout = signedOut.loadSession();
signedOut.userLogout();
await signedOut.loadSession();
resolveUser({ id: 1, permissions: ['user.manage'] });
await pendingLogout;
assert.equal(signedOut.userLogined, false);
assert.equal(signedOut.hasPermission('post.view'), true);
assert.equal(signedOut.hasPermission('user.manage'), false);

setActivePinia(createPinia());
const switched = useStoreUser();
storage.set(TOKEN_KEY, 'earlier-session');
const pendingSwitch = switched.loadSession();
storage.set(TOKEN_KEY, 'new-session');
switched.updateUserinfo({ id: 2, permissions: ['post.create'] });
resolveUser({ id: 1, permissions: ['user.manage'] });
await pendingSwitch;
assert.equal(switched.userInfo.id, 2);
assert.equal(switched.hasPermission('post.create'), true);
assert.equal(switched.hasPermission('user.manage'), false);

setActivePinia(createPinia());
const expired = useStoreUser();
storage.set(TOKEN_KEY, 'disabled-or-expired-session');
const pendingExpired = Promise.all([expired.loadSession(), expired.loadSession()]);
// The shared HTTP 401 interceptor invalidates the token before rejecting user/info.
expired.userLogout();
rejectUser({ code: 20006 });
await pendingExpired;
assert.equal(storage.has(TOKEN_KEY), false);
assert.equal(expired.userLogined, false);
assert.equal(expired.hasPermission('post.view'), true, 'invalidated bootstrap must restore guest permissions');

setActivePinia(createPinia());
const rejectedOld = useStoreUser();
storage.set(TOKEN_KEY, 'rejected-old-session');
const pendingRejected = rejectedOld.loadSession();
storage.set(TOKEN_KEY, 'current-session');
rejectedOld.updateUserinfo({ id: 3, permissions: ['post.create'] });
rejectUser({ code: 10006 });
await pendingRejected;
assert.equal(rejectedOld.userInfo.id, 3);
assert.equal(storage.get(TOKEN_KEY), 'current-session');

for (const failure of [new Error('Network unavailable'), { code: 20007 }]) {
  setActivePinia(createPinia());
  const unavailable = useStoreUser();
  storage.set(TOKEN_KEY, 'valid-session');
  const pendingFailure = unavailable.loadSession();
  rejectUser(failure);
  await assert.rejects(pendingFailure, (error) => error === failure);
  assert.equal(storage.get(TOKEN_KEY), 'valid-session', 'ordinary failures must preserve the session');
}

// Two concurrent expired requests must not invalidate the guest recovery started by the first 401.
const requestSource = readFileSync(new URL('../src/utils/request.ts', import.meta.url), 'utf8').replace('import.meta.env.VITE_HOST', "''");
const requestCompiled = ts.transpileModule(requestSource, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
const requestModule = { exports: {} };
const requestModules = {
  '@/store/user': { useStoreUser, TOKEN_KEY },
  '@/store/main': { useStoreMain: () => ({ triggerAuth() {} }) },
  '@/locales/errorCodes': { translateErrMsg: () => '' },
};
new Function('require', 'module', 'exports', 'localStorage', 'window', requestCompiled)(
  (name) => requestModules[name] ?? require(name), requestModule, requestModule.exports, localStorage, {},
);
const service = requestModule.exports.default;
const requests = new Map();
service.defaults.adapter = (config) => new Promise((_resolve, reject) => {
  requests.set(config.url, () => reject({ config, response: { status: 401, data: { code: 20006 } } }));
});
let resolveGuest;
modules['@/api/auth'].userInfo = () => service.get('/v1/user/info');
modules['@/api/identity'].getIdentity = () => new Promise((resolve) => { resolveGuest = resolve; });
setActivePinia(createPinia());
const concurrent = useStoreUser();
storage.set(TOKEN_KEY, 'expired-session');
const pendingConcurrent = Promise.all([concurrent.loadSession(), concurrent.loadSession()]);
const pendingTopics = assert.rejects(service.get('/v1/topics'), (error) => error.code === 20006);
await new Promise(setImmediate);
requests.get('/v1/user/info')();
await new Promise(setImmediate);
assert.equal(storage.has(TOKEN_KEY), false);
assert.equal(typeof resolveGuest, 'function', 'first 401 must start guest recovery');
requests.get('/v1/topics')();
await pendingTopics;
resolveGuest({ permissions: ['post.view'] });
await pendingConcurrent;
assert.equal(concurrent.userLogined, false);
assert.equal(concurrent.hasPermission('post.view'), true, 'late 401 must preserve guest recovery');
console.log('Permission and session-race checks passed, including concurrent 401 guest recovery and current-session error preservation.');
