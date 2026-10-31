#!/usr/bin/env node
// Requires a disposable loopback fixture from scripts/test_columns.py.
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
const { chromium, expect } = createRequire(import.meta.url)('playwright/test');

async function main() {
  const fixture = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
  if (!['localhost', '127.0.0.1', '::1'].includes(new URL(fixture.base).hostname)) throw new Error('Use a disposable loopback server');
  const output = process.argv[3] || '/tmp/websitecore-column-returns';
  fs.mkdirSync(output, { recursive: true });
  const browser = await chromium.launch({ executablePath: process.env.COLUMNS_BROWSER_EXECUTABLE || undefined, headless: true, args: ['--no-sandbox'] });
  let checks = 0;
  let restoreIdentity;
  const check = (condition, message) => { if (!condition) throw new Error(message); checks++; };
  try {
    const context = await browser.newContext({ viewport: { width: 1366, height: 1000 } });
    await context.addInitScript(() => { if (!localStorage.getItem('PAOPAO_LOCALE')) localStorage.setItem('PAOPAO_LOCALE', 'zh-CN'); });
    await context.grantPermissions(['clipboard-read', 'clipboard-write'], { origin: fixture.base });
    async function api(method, route, token, data, success = true) {
      const response = await context.request.fetch(fixture.base + route, { method, data, headers: token ? { Authorization: `Bearer ${token}` } : {} });
      const result = await response.json();
      check((result.code === 0) === success, `${method} ${route}: ${result.msg}`);
      return result.data;
    }
    const login = async name => (await api('POST', '/v1/auth/login', '', { username: name, password: fixture.password })).token;
    const operator = await login(fixture.operator), author = await login(fixture.names.a), other = await login(fixture.names.b);
    const originalGroups = (await api('GET', '/v1/user/info', author)).identity_groups
      .filter(group => group.key !== 'guest' && group.key !== 'member').map(group => group.id);
    const stamp = Date.now(), title = `返还内容示例 ${stamp}`, latestTitle = `最新提交 ${stamp}`;
    const body = '# 最新一次提交\n\n这里保留尚未公开的 **最新修改**。\n\n- 可以复制或下载 Markdown\n- 可以另建文章重新送审';
    async function review(item) {
      const queue = await api('GET', '/v1/admin/audit/columns?page_size=100', operator);
      const task = queue.items.find(row => row.submission?.id === item.submission_id).task;
      await api('POST', '/v1/admin/audit/column', operator, { task_id: task.id, target_id: task.target_id, kind: task.kind, revision: task.revision, action: 'approve' });
    }
    const original = (await api('POST', '/v1/column/submit', author, { kind: 'article', title, body: 'Previously public body' })).item;
    await review(original);
    await api('POST', '/v1/column/action', operator, { id: original.id, action: 'invite' });
    await api('POST', '/v1/column/comment', operator, { column_id: original.id, contents: [{ type: 2, content: 'Original discussion', sort: 1 }] });
    const entries = await api('GET', `/v1/column/entries?id=${fixture.collection}`, '');
    await api('POST', '/v1/column/entries', operator, { id: fixture.collection, articles: [...entries.items.map(item => item.id), original.id] });
    const upload = await (await context.request.post(fixture.base + '/v1/attachment', {
      headers: { Authorization: `Bearer ${author}` }, multipart: { type: 'attachment', file: { name: 'returned-notes.md', mimeType: 'text/plain', buffer: Buffer.from('Verified returned attachment') } },
    })).json();
    check(upload.code === 0, 'Attachment upload failed');
    const latest = (await api('POST', '/v1/column/submit', author, { id: original.id, expected_submission: original.submission_id,
      kind: 'article', title: latestTitle, body, attachments: [{ type: 7, content: upload.data.content, sort: 1 }] })).item;
    await api('POST', '/v1/column/action', operator, { id: original.id, action: 'delete' }, false);
    await api('POST', '/v1/column/action', operator, { id: original.id, action: 'delete', reason: '理'.repeat(256) }, false);

    const page = await context.newPage(), errors = [];
    page.on('pageerror', error => errors.push(error.message));
    async function identity(token, target) {
      await page.goto(fixture.base + '/v1/');
      await page.evaluate(value => value ? localStorage.setItem('PAOPAO_TOKEN', value) : localStorage.removeItem('PAOPAO_TOKEN'), token);
      await page.goto(fixture.base + target);
    }
    async function settled() {
      await page.waitForLoadState('networkidle');
      await expect(page.locator('.n-spin-body:visible')).toHaveCount(0);
      await expect(page.locator('.n-spin-content--spinning')).toHaveCount(0);
    }
    await identity(operator, `/#/columns?view=read&id=${original.id}`);
    await expect(page.locator('article > h1')).toHaveText(title);
    await page.getByRole('button', { name: '删除', exact: true }).click();
    const confirm = page.getByRole('button', { name: '删除并返还作者', exact: true });
    await expect(confirm).toBeDisabled();
    await page.getByPlaceholder('请填写删除原因（必填，最多 255 字符）').fill('请修改文章中的相关内容后重新送审。');
    await expect(confirm).toBeEnabled();
    await page.screenshot({ path: path.join(output, 'admin-delete-reason.png'), fullPage: true, animations: 'disabled' });
    const deletion = page.waitForResponse(response => response.url().endsWith('/v1/column/action') && response.request().method() === 'POST');
    await confirm.click();
    check((await (await deletion).json()).code === 0, 'Administrator deletion failed');
    await page.waitForURL(/#\/columns$/);
    await api('GET', `/v1/column?id=${original.id}`, '', undefined, false);
    await api('GET', `/v1/column/comments?id=${original.id}`, author, undefined, false);
    await api('GET', `/v1/column/submission?id=${original.id}`, author, undefined, false);
    await api('GET', `/v1/column/attachment?id=${original.id}&submission_id=${latest.submission_id}&index=0`, author, undefined, false);
    for (const token of ['', other, operator]) {
      await api('GET', `/v1/column/returned?id=${original.id}`, token, undefined, false);
      await api('GET', `/v1/column/returned/attachment?id=${original.id}&index=0`, token, undefined, false);
    }
    const returned = (await api('GET', `/v1/column/returned?id=${original.id}`, author)).item;
    check(returned.body === body && returned.title === latestTitle && returned.submission_id === latest.submission_id, 'Return did not use the latest submission');
    const own = await api('GET', '/v1/columns?kind=article&mine=true&returned=true', author);
    check(own.items.some(item => item.id === original.id && item.title === latestTitle), 'Return missing from owner list');
    check(!(await api('GET', '/v1/columns?kind=article&mine=true&returned=true', other)).items.some(item => item.id === original.id), 'Return leaked through another user list');
    check(!(await api('GET', `/v1/column/entries?id=${fixture.collection}`, '')).items.some(item => item.id === original.id), 'Deleted article remained in collection');
    await api('POST', '/v1/column/action', operator, { id: original.id, action: 'delete', reason: 'Repeat' }, false);
    const messages = (await api('GET', '/v1/user/chat/history?user_id=0&page_size=100', author)).messages;
    check(messages.filter(item => item.column_id === original.id && item.column_returned).length === 1, 'Return notice was missing or duplicated');
    check(messages.some(item => item.column_returned && item.content.includes('重新送审')), 'Deletion reason missing from notice');

    // Losing publication permission must not prevent private retrieval or export.
    const member = (await api('GET', '/v1/admin/identity/groups', operator)).groups.find(group => group.key === 'member');
    await api('POST', '/v1/admin/identity/groups', operator, { ...member, permissions: member.permissions.filter(permission => permission !== 'column.publish') });
    try {
      await identity(author, `/#/columns?view=returned&id=${original.id}`);
      await expect(page.locator('.returned-column > h1')).toHaveText(latestTitle);
      await expect(page.getByRole('button', { name: '基于此内容重新创作', exact: true })).toHaveCount(0);
      await page.getByRole('button', { name: '复制 Markdown', exact: true }).click();
      check(await page.evaluate(() => navigator.clipboard.readText()) === body, 'Copy did not export the latest Markdown');
      const exported = page.waitForEvent('download');
      await page.getByRole('button', { name: '下载 Markdown', exact: true }).click();
      const download = await exported;
      check(fs.readFileSync(await download.path(), 'utf8') === body, 'Downloaded Markdown differs from latest submission');
      const attachmentResponse = page.waitForResponse(response => response.url().includes('/v1/column/returned/attachment'));
      await page.getByRole('button', { name: '附件 1', exact: true }).click();
      const signed = (await (await attachmentResponse).json()).data.signed_url;
      check((await (await context.request.get(signed)).text()).includes('Verified returned attachment'), 'Returned attachment download failed');
      await api('POST', '/v1/column/submit', author, { recreated_from_id: original.id, kind: 'article', title: 'Denied', body }, false);
      await settled();
      await page.screenshot({ path: path.join(output, 'returned-without-publish-permission.png'), fullPage: true, animations: 'disabled' });
    } finally {
      await api('POST', '/v1/admin/identity/groups', operator, member);
    }
    const exempt = (await api('POST', '/v1/admin/identity/groups', operator, { key: `return_exempt_${stamp}`, name: 'Return test exemption', permissions: ['content.publish_unreviewed'] })).groups[0];
    await api('POST', '/v1/admin/user/identity', operator, { user_id: fixture.ids.a, group_ids: [exempt.id] });
    restoreIdentity = () => api('POST', '/v1/admin/user/identity', operator, { user_id: fixture.ids.a, group_ids: originalGroups });
    check((await api('GET', '/v1/user/info', author)).permissions.includes('content.publish_unreviewed'), 'Exemption fixture not configured');
    await identity(author, '/#/columns?tab=mine&returned=1');
    await page.locator('.column-row').filter({ hasText: latestTitle }).getByRole('button', { name: '取回内容', exact: true }).click();
    await expect(page.locator('.returned-column > h1')).toHaveText(latestTitle);
    await settled();
    await page.screenshot({ path: path.join(output, 'returned-latest-submission.png'), fullPage: true, animations: 'disabled' });
    for (const width of [375, 390, 821, 1366]) {
      await page.setViewportSize({ width, height: 900 });
      check(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `Returned view overflow at ${width}px`);
    }
    await page.setViewportSize({ width: 390, height: 844 });
    await page.screenshot({ path: path.join(output, 'returned-mobile.png'), fullPage: true, animations: 'disabled' });
    await page.getByRole('button', { name: '基于此内容重新创作', exact: true }).click();
    const titleInput = page.locator('.column-form .n-form-item').first().locator('input');
    await expect(titleInput).toHaveValue(latestTitle);
    const recreatedTitle = `重新创作 ${stamp}`;
    await titleInput.fill(recreatedTitle);
    await expect.poll(() => page.evaluate(id => Object.keys(localStorage).some(key => key.startsWith('columns:draft:') && JSON.parse(localStorage.getItem(key) || '{}').sourceId === id), original.id)).toBe(true);
    await page.getByRole('button', { name: '返回专栏', exact: true }).click();
    await page.getByRole('button', { name: recreatedTitle, exact: true }).click();
    check(new URL(page.url()).hash.includes(`source=${original.id}`), 'Reopening a local draft lost its return source');
    await page.reload();
    await expect(titleInput).toHaveValue(recreatedTitle);
    const submitted = page.waitForResponse(response => response.url().endsWith('/v1/column/submit') && response.request().method() === 'POST');
    await page.getByRole('button', { name: '提交发布', exact: true }).click();
    const recreated = (await (await submitted).json()).data.item;
    check(recreated.id !== original.id && !recreated.published_on && recreated.submission.audit_status === 0 && !recreated.official && !recreated.locked, 'Recreation restored old state or bypassed review');
    check(recreated.submission.body === body && recreated.submission.attachments.length === 1, 'Recreation lost Markdown or attachments');
    await page.waitForURL(/tab=mine/);
    await page.locator('.column-row').filter({ hasText: recreatedTitle }).getByRole('button', { name: '编辑', exact: true }).click();
    await page.getByRole('button', { name: '撤回审核', exact: true }).click();
    await expect(titleInput).toBeEnabled();
    const resubmission = page.waitForResponse(response => response.url().endsWith('/v1/column/submit') && response.request().method() === 'POST');
    await page.getByRole('button', { name: '提交发布', exact: true }).click();
    const resubmitted = (await (await resubmission).json()).data.item;
    check(resubmitted.submission.audit_status === 0, 'Withdraw/resubmit bypassed mandatory review');
    await api('GET', `/v1/column?id=${recreated.id}`, '', undefined, false);
    await review(resubmitted);
    const published = (await api('GET', `/v1/column?id=${recreated.id}`, '')).item;
    check(published.body === body, 'Approved recreation did not publish');
    check((await api('GET', `/v1/column/comments?id=${recreated.id}`, '')).total === 0, 'Recreation inherited old comments');
    await api('GET', `/v1/column?id=${original.id}`, '', undefined, false);
    check((await api('GET', `/v1/column/returned?id=${original.id}`, author)).item.body === body, 'Recreation altered returned snapshot');
    check(errors.length === 0, 'Browser runtime errors: ' + errors.join('; '));
    console.log(`Column return checks passed: ${checks}; latest submission, administrator deletion UI, owner-only retrieval, exports, attachments, revocation, notices and mandatory review.`);
  } finally {
    try {
      if (restoreIdentity) await restoreIdentity();
    } finally {
      await browser.close();
    }
  }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
