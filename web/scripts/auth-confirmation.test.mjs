import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import { createRequire } from 'node:module';
import { parse } from '@vue/compiler-sfc';
import ts from 'typescript';
import * as vue from 'vue';

const require = createRequire(import.meta.url);
const Schema = require('async-validator').default;
const source = fs.readFileSync(new URL('../src/components/auth.vue', import.meta.url), 'utf8');
const script = parse(source).descriptor.scriptSetup.content.replace(/^import .*;\r?$/gm, '');
const code = ts.transpileModule(`${script}\nmodule.exports = { registerForm, registerRule, registerRef, handleRegister };`, {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
}).outputText;
const submitted = [];
const store = {
  triggerAuth() {}, updateUserinfo() {},
};
const module = { exports: {} };
vm.runInNewContext(code, {
  ...vue, module, useI18n: () => ({ t: (key) => key }),
  useStoreMain: () => store, useStoreUser: () => store,
  useStoreProfile: () => ({ profile: {} }), storeToRefs: (value) => value,
  Api: { v1: { auth: { post: {
    register: (value) => { submitted.push({ ...value }); return Promise.resolve({}); },
    login: () => Promise.resolve({ token: 'synthetic-token' }),
  } } } },
  TOKEN_KEY: 'test', localStorage: { setItem() {} },
  userInfo: () => Promise.resolve({ id: 1 }), window: { $message: { success() {} } },
});
const { registerForm, registerRule, registerRef, handleRegister } = module.exports;
// Naive UI submits all rules through async-validator, independently of input triggers.
registerRef.value = {
  validate(callback) {
    new Schema(registerRule.value).validate(registerForm).then(() => callback(null), ({ errors }) => callback(errors));
  },
};
const event = { preventDefault() {}, stopPropagation() {} };
for (const confirmation of ['', 'L', 'LocalTest4', 'Different42']) {
  Object.assign(registerForm, { username: 'normaluser', password: 'LocalTest42', repassword: confirmation });
  handleRegister(event);
  await new Promise(setImmediate);
  assert.equal(submitted.length, 0, `confirmation ${JSON.stringify(confirmation)} must prevent submission`);
}
Object.assign(registerForm, { username: 'normaluser', password: 'LocalTest42', repassword: 'LocalTest42' });
// Editing the original password must also be checked at submit, even before another confirmation input event.
registerForm.password = 'UpdatedTest42';
handleRegister(event);
await new Promise(setImmediate);
assert.equal(submitted.length, 0);
registerForm.repassword = 'UpdatedTest42';
handleRegister(event);
await new Promise(setImmediate);
assert.equal(submitted.length, 1);
assert.equal(submitted[0].password, 'UpdatedTest42');
assert.equal(registerForm.repassword, '', 'successful registration still clears the form');
console.log('Registration confirmation and submit validation passed.');
