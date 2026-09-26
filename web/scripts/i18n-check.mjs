#!/usr/bin/env node
/**
 * i18n 语言包校验脚本（适配 src/locales/<locale>/<namespace>.json 结构）
 *
 * 检查项:
 *  1. missing  —— 代码中 t()/te()/titleKey 引用的 key 在语言包中不存在
 *  2. unused   —— 语言包中存在但代码未引用的 key（errors.codes.* 动态引用除外）
 *  3. parity   —— zh-CN 与 en 的 key 结构必须完全一致
 *
 * 用法: node scripts/i18n-check.mjs [--fix-unused 仅打印, 不自动删]
 */
import fs from 'node:fs';
import path from 'node:path';

const SRC = path.resolve('src');
const LOCALES_DIR = path.join(SRC, 'locales');
const SOURCE_LOCALE = 'zh-CN';
const TARGET_LOCALES = ['en'];

// ---------- 收集语言包 ----------
function leaves(obj, prefix = '') {
  return Object.entries(obj).flatMap(([k, v]) =>
    v && typeof v === 'object' ? leaves(v, `${prefix}${k}.`) : [`${prefix}${k}`],
  );
}
function loadLocale(loc) {
  const dir = path.join(LOCALES_DIR, loc);
  const map = new Map(); // full key -> value
  for (const f of fs.readdirSync(dir)) {
    if (!f.endsWith('.json')) continue;
    const ns = path.basename(f, '.json');
    const json = JSON.parse(fs.readFileSync(path.join(dir, f), 'utf8'));
    for (const leaf of leaves(json)) map.set(`${ns}.${leaf}`, leaf);
  }
  return map;
}
const sourceKeys = loadLocale(SOURCE_LOCALE);
const targetKeys = Object.fromEntries(TARGET_LOCALES.map((l) => [l, loadLocale(l)]));

// ---------- 收集代码引用 ----------
function* walk(dir) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) {
      if (p === LOCALES_DIR) continue; // 跳过语言包本身
      yield* walk(p);
    } else if (/\.(vue|ts)$/.test(e.name) && !e.name.endsWith('.d.ts')) {
      yield p;
    }
  }
}
const usedKeys = new Set();
// t('key' / te('key' / $t('key' / i18n.global.t('key'
const callRe = /\bte?\(\s*['"]([a-zA-Z0-9_.]+)['"]/g;
// 路由 meta titleKey: 'nav.xxx' 等动态 t(keyVar) 的字符串来源
const titleKeyRe = /titleKey:\s*['"]([a-zA-Z0-9_.]+)['"]/g;
for (const file of walk(SRC)) {
  const text = fs.readFileSync(file, 'utf8');
  for (const m of text.matchAll(callRe)) usedKeys.add(m[1]);
  for (const m of text.matchAll(titleKeyRe)) usedKeys.add(m[1]);
}

// ---------- 检查 ----------
let fail = false;

// 1. missing
const missing = [...usedKeys].filter((k) => !sourceKeys.has(k)).sort();
if (missing.length) {
  fail = true;
  console.error(`\n❌ [missing] 代码引用但 ${SOURCE_LOCALE} 语言包缺失 (${missing.length}):`);
  for (const k of missing) console.error('   -', k);
}

// 2. parity
for (const loc of TARGET_LOCALES) {
  const tk = targetKeys[loc];
  const onlySource = [...sourceKeys.keys()].filter((k) => !tk.has(k));
  const onlyTarget = [...tk.keys()].filter((k) => !sourceKeys.has(k));
  if (onlySource.length || onlyTarget.length) {
    fail = true;
    console.error(`\n❌ [parity] ${SOURCE_LOCALE} vs ${loc} 结构不一致:`);
    if (onlySource.length) console.error(`   ${loc} 缺少 (${onlySource.length}):`, onlySource.join(', '));
    if (onlyTarget.length) console.error(`   ${loc} 多出 (${onlyTarget.length}):`, onlyTarget.join(', '));
  }
}

// 3. unused（errors.* 走 translateErrMsg 动态拼接/默认参数，豁免）
const unused = [...sourceKeys.keys()]
  .filter((k) => !usedKeys.has(k) && !k.startsWith('errors.'))
  .sort();
if (unused.length) {
  console.warn(`\n⚠️  [unused] 语言包存在但代码未引用 (${unused.length}):`);
  for (const k of unused) console.warn('   -', k);
}

if (!fail) {
  console.log(
    `✅ i18n check OK: ${sourceKeys.size} keys (${SOURCE_LOCALE}), 引用 ${usedKeys.size} 个 key, missing=0, parity=OK` +
      (unused.length ? `, unused=${unused.length}(仅警告)` : ''),
  );
}
process.exit(fail ? 1 : 0);
