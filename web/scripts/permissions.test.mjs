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
let resolveUser;
const modules = {
  '@/api/auth': { userInfo: () => new Promise((resolve) => { resolveUser = resolve; }) },
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
console.log('Permission and session-race checks passed.');
