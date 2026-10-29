#!/usr/bin/env node
// Requires Playwright and a disposable loopback fixture from scripts/test_columns.py.
// COLUMNS_BROWSER_EXECUTABLE can select an installed Chrome instead of bundled Chromium.
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
const loadPackage = createRequire(import.meta.url);
const { chromium, expect } = loadPackage('playwright/test');

async function main() {
  const fixturePath = process.argv[2];
  if (!fixturePath) throw new Error('Provide the synthetic fixture JSON from scripts/test_columns.py');
  const fixture = JSON.parse(fs.readFileSync(fixturePath, 'utf8'));
  if (!['localhost', '127.0.0.1', '::1'].includes(new URL(fixture.base).hostname)) throw new Error('A disposable loopback server is required');
  const output = process.argv[3] || '/tmp/websitecore-columns-ui';
  fs.mkdirSync(output, { recursive: true });
  const browser = await chromium.launch({ executablePath: process.env.COLUMNS_BROWSER_EXECUTABLE || undefined, headless: true, args: ['--no-sandbox'] });
  let checks = 0;
  const check = (condition, message) => { if (!condition) throw new Error(message); checks++; };
  try {
    const context = await browser.newContext({ viewport: { width: 1366, height: 1000 } });
    await context.addInitScript(() => { if (!localStorage.getItem('PAOPAO_LOCALE')) localStorage.setItem('PAOPAO_LOCALE', 'zh-CN'); });
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    async function api(method, route, token, data) {
      const response = await context.request.fetch(fixture.base + route, { method, data, headers: token ? { Authorization: `Bearer ${token}` } : {} });
      const body = await response.json();
      check(body.code === 0, `${method} ${route}: ${body.msg}`);
      return body.data;
    }
    async function login(name) { return (await api('POST', '/v1/auth/login', '', { username: name, password: fixture.password })).token; }
    async function identity(token, target) {
      // Set session storage on a same-origin JSON document before the SPA boots.
      await page.goto(fixture.base + '/v1/');
      await page.evaluate(value => localStorage.setItem('PAOPAO_TOKEN', value), token);
      await page.goto(fixture.base + target);
    }
    async function settled() {
      await expect(page.locator('.columns-page .n-spin-body:visible')).toHaveCount(0);
      await expect(page.locator('.columns-page .n-spin-content--spinning')).toHaveCount(0);
    }
    const tab = label => page.locator('.n-tabs-tab').filter({ hasText: label });
    await page.goto(fixture.base + '/#/columns');
    await tab(/^文章$/).waitFor();
    await settled();
    check(await page.getByRole('button', { name: '写专栏', exact: true }).count() === 0, 'Guest sees publishing control');
    await page.screenshot({ path: path.join(output, 'columns-guest.png'), fullPage: true });
    await page.goto(`${fixture.base}/#/columns?view=read&id=${fixture.article}`);
    await page.locator('article > h1').waitFor();
    await settled();
    await page.screenshot({ path: path.join(output, 'column-detail.png'), fullPage: true });

    const author = await login(fixture.names.a), operator = await login(fixture.operator);
    await identity(author, '/#/columns?tab=mine');
    await page.getByRole('button', { name: '写专栏', exact: true }).click();
    const fileInput = page.locator('input[accept=".md,text/markdown,text/plain"]');
    const title = `browser-article-${Date.now()}`;
    await fileInput.setInputFiles({ name: title + '.md', mimeType: 'text/markdown', buffer: Buffer.from('# Browser article\n\nImported **Markdown** body.') });
    const titleInput = () => page.locator('.column-form .n-form-item').first().locator('input');
    await expect(titleInput()).toHaveValue(title);
    await expect.poll(() => page.evaluate(() => Object.keys(localStorage).filter(key => key.startsWith('columns:draft:')).length)).toBeGreaterThan(0);
    const firstDraftUrl = page.url();
    await page.reload();
    await expect(titleInput()).toHaveValue(title);
    await settled();
    checks++;
    await fileInput.setInputFiles({ name: 'oversized.md', mimeType: 'text/markdown', buffer: Buffer.from('x'.repeat(50001)) });
    await expect(page.getByText('请导入不超过 50,000 字符的 .md 文件。', { exact: true })).toBeVisible();
    await expect(titleInput()).toHaveValue(title);
    checks++;
    await page.getByRole('button', { name: '返回专栏', exact: true }).click();
    await page.getByRole('button', { name: '写专栏', exact: true }).click();
    await fileInput.setInputFiles({ name: 'second-local-draft.md', mimeType: 'text/markdown', buffer: Buffer.from('Second unsent draft') });
    await expect.poll(() => page.evaluate(() => Object.keys(localStorage).filter(key => key.startsWith('columns:draft:')).length)).toBeGreaterThan(1);
    checks++;
    await page.goto(firstDraftUrl);
    await expect(titleInput()).toHaveValue(title);
    await page.route('**/v1/column/submit', route => route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ code: 50000, msg: 'Synthetic save failure' }) }));
    await page.getByRole('button', { name: '提交发布', exact: true }).click();
    await expect(page.getByRole('button', { name: '提交发布', exact: true })).toBeEnabled();
    await expect(titleInput()).toHaveValue(title);
    check(page.url() === firstDraftUrl, 'Failed submission navigated away from draft');
    await page.unroute('**/v1/column/submit');
    await settled();
    await page.screenshot({ path: path.join(output, 'column-editor.png'), fullPage: true });
    await page.getByRole('button', { name: '提交发布', exact: true }).click();
    await page.waitForURL(/tab=mine/);
    const row = () => page.locator('.column-row').filter({ hasText: title });
    await row().getByRole('button', { name: '编辑', exact: true }).click();
    await expect(titleInput()).toBeDisabled();
    await page.getByRole('button', { name: '撤回审核', exact: true }).click();
    await expect(titleInput()).toBeEnabled();
    await page.getByRole('button', { name: '提交发布', exact: true }).click();
    await page.waitForURL(/tab=mine/);
    const own = await api('GET', '/v1/columns?mine=true&page_size=100', author);
    const article = own.items.find(item => item.submission?.title === title);
    check(!!article, 'Browser submission missing from own content');
    check(!own.items.some(item => (item.submission?.title || item.title) === 'second-local-draft'), 'Unsubmitted draft was saved on the server');

    await identity(operator, '/#/columns?tab=mine');
    await settled();
    check(await page.getByRole('button', { name: 'second-local-draft', exact: true }).count() === 0, 'Local draft leaked to another account');
    await page.goto(fixture.base + '/#/admin/audit');
    await tab(/^专栏审核$/).click();
    const review = page.locator('.review-item').filter({ hasText: title });
    await review.getByRole('button', { name: '通过', exact: true }).click();
    await expect(review).toHaveCount(0);
    checks++;
    await identity(author, `/#/columns?view=read&id=${article.id}`);
    await page.locator('article > h1').waitFor();
    const comment = 'Browser discussion ' + Date.now();
    await page.locator('.compose-wrap textarea').fill(comment);
    await page.locator('.compose-wrap').getByRole('button', { name: '发布', exact: true }).click();
    await expect(page.locator('.comment-item').filter({ hasText: comment })).toHaveCount(1);
    checks++;
    const queue = await api('GET', '/v1/admin/audit/columns?page_size=100', operator);
    const commentReview = queue.items.find(item => item.comment?.content === comment);
    await api('POST', '/v1/admin/audit/column', operator, { task_id: commentReview.task.id, target_id: commentReview.task.target_id, kind: commentReview.task.kind, revision: commentReview.task.revision, action: 'approve' });
    await page.reload();
    const thread = page.locator('.comment-item').filter({ hasText: comment });
    await thread.locator('.reply-btn').first().click();
    await thread.locator('.reply-input-wrap input').fill('Browser reply');
    await thread.locator('.reply-input-wrap').getByRole('button', { name: '回复', exact: true }).click();
    await expect(thread.getByText('Browser reply', { exact: true })).toBeVisible();
    checks++;
    await page.getByRole('button', { name: '锁定评论', exact: true }).click();
    await expect(page.locator('.compose-wrap textarea')).toBeDisabled();
    await expect(page.locator('.reply-btn')).toHaveCount(0);
    checks++;
    await page.getByRole('button', { name: '解锁评论', exact: true }).click();

    for (const width of [375, 390, 821, 822, 830, 1000, 1200, 1366, 1600, 1920]) {
      await page.setViewportSize({ width, height: 900 });
      await settled();
      check(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), `Horizontal overflow at ${width}px`);
    }
    await page.setViewportSize({ width: 390, height: 844 });
    await page.screenshot({ path: path.join(output, 'column-mobile.png'), fullPage: true });
    await page.evaluate(() => { localStorage.setItem('PAOPAO_LOCALE', 'en'); localStorage.setItem('PAOPAO_THEME', 'dark'); });
    await page.reload();
    await page.locator('article > h1').waitFor();
    await settled();
    await expect(page.getByRole('button', { name: 'Copy link', exact: true })).toBeVisible();
    await page.screenshot({ path: path.join(output, 'column-mobile-en-dark.png'), fullPage: true });
    checks++;

    await identity(operator, `/#/columns?view=entries&id=${fixture.collection}&kind=collection`);
    await expect(page.locator('.ordered-entries li')).toHaveCount(2);
    const before = await api('GET', `/v1/column/entries?id=${fixture.collection}`, '');
    await page.locator('.ordered-entries li').first().getByRole('button', { name: 'Move down', exact: true }).click();
    const orderSaved = page.waitForResponse(response => response.url().endsWith('/v1/column/entries') && response.request().method() === 'POST');
    await page.getByRole('button', { name: 'Save articles and order', exact: true }).click();
    await orderSaved;
    const after = await api('GET', `/v1/column/entries?id=${fixture.collection}`, '');
    check(after.items[0].id === before.items[1].id, 'Collection order UI did not persist');
    await page.screenshot({ path: path.join(output, 'collection-order-mobile.png'), fullPage: true });
    check(errors.length === 0, 'Browser runtime errors: ' + errors.join('; '));
    console.log(`Column browser checks passed: ${checks}; Markdown import, local drafts, failed-save retention, review, comments, collections, ten widths and English/dark mode.`);
  } finally {
    await browser.close();
  }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
