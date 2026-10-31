#!/usr/bin/env node
// Uses a disposable loopback fixture created by scripts/test_columns.py.
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
const { chromium, expect } = createRequire(import.meta.url)('playwright/test');

async function main() {
  const fixture = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
  if (!['localhost', '127.0.0.1', '::1'].includes(new URL(fixture.base).hostname)) throw new Error('Use a disposable loopback server');
  const output = process.argv[3] || '/tmp/websitecore-columns-discovery';
  fs.mkdirSync(output, { recursive: true });
  const browser = await chromium.launch({ executablePath: process.env.COLUMNS_BROWSER_EXECUTABLE || undefined, headless: true, args: ['--no-sandbox'] });
  let checks = 0;
  try {
    const context = await browser.newContext({ viewport: { width: 1366, height: 1000 } });
    await context.addInitScript(() => { if (!localStorage.getItem('PAOPAO_LOCALE')) localStorage.setItem('PAOPAO_LOCALE', 'zh-CN'); });
    const api = async (method, route, token, data) => {
      const response = await context.request.fetch(fixture.base + route, { method, data, headers: token ? { Authorization: `Bearer ${token}` } : {} });
      const body = await response.json();
      if (body.code !== 0) throw new Error(`${route}: ${body.msg}`);
      return body.data;
    };
    const login = async name => (await api('POST', '/v1/auth/login', '', { username: name, password: fixture.password })).token;
    const operator = await login(fixture.operator), author = await login(fixture.names.a);
    const operatorId = (await api('GET', '/v1/user/info', operator)).id;
    const publicCount = async (kind, authorId) => (await api('GET', `/v1/columns?kind=${kind}&author_id=${authorId}&page_size=1`, '')).total;
    const operatorArticleCount = await publicCount('article', operatorId);
    const operatorCollectionCount = await publicCount('collection', operatorId);
    const authorArticleCount = await publicCount('article', fixture.ids.a);
    const authorCollectionCount = await publicCount('collection', fixture.ids.a);
    const stamp = Date.now();
    const intro = `IntroductionSearch${stamp}`, bodyKeyword = `BodyOnly${stamp}`;
    const submit = async (token, kind, title, summary = '', item) => (await api('POST', '/v1/column/submit', token, {
      id: item?.id || 0, expected_submission: item?.submission_id || 0,
      kind, title, summary, body: kind === 'article' ? bodyKeyword : '',
    })).item;
    const articles = [], collections = [];
    for (let i = 1; i <= 21; i++) articles.push(await submit(operator, 'article', `公开专栏 ${String(i).padStart(2, '0')} · ${stamp}`));
    for (const action of ['takedown', 'delete']) {
      const item = await submit(operator, 'article', `${action} hidden article ${stamp}`);
      await api('POST', '/v1/column/action', operator, { id: item.id, action });
    }
    for (let i = 1; i <= 21; i++) {
      const item = await submit(operator, 'collection', `合集名称 ${String(i).padStart(2, '0')} · ${stamp}`, intro);
      await api('POST', '/v1/column/entries', operator, { id: item.id, articles: [articles[0].id] });
      collections.push(item);
    }
    await submit(operator, 'collection', `Empty collection ${stamp}`, intro);
    const ownerCollectionTitle = `作者公开合集 ${stamp}`;
    const ownerCollection = await submit(author, 'collection', ownerCollectionTitle, `OriginalIntroduction${stamp}`);
    await api('POST', '/v1/column/entries', author, { id: ownerCollection.id, articles: [fixture.article] });
    const queue = await api('GET', '/v1/admin/audit/columns?page_size=100', operator);
    const task = queue.items.find(item => item.submission?.id === ownerCollection.submission_id).task;
    await api('POST', '/v1/admin/audit/column', operator, { task_id: task.id, target_id: task.target_id, kind: task.kind, revision: task.revision, action: 'approve' });
    const pendingName = `PendingMetadata${stamp}`;
    await submit(author, 'collection', pendingName, pendingName, ownerCollection);
    await submit(author, 'article', `PendingArticle${stamp}`);

    const page = await context.newPage(), errors = [];
    page.on('pageerror', error => errors.push(error.message));
    const profileTab = label => page.locator('.profile-tabs-wrap .n-tabs-tab').filter({ hasText: new RegExp(`^${label}$`) });
    const rows = page.locator('.author-column-row');
    async function settled() {
      await page.waitForLoadState('networkidle');
      await expect(page.locator('.n-spin-body:visible')).toHaveCount(0);
      await expect(page.locator('.n-spin-content--spinning')).toHaveCount(0);
    }
    async function identity(token, target) {
      await page.goto(fixture.base + '/v1/');
      await page.evaluate(value => value ? localStorage.setItem('PAOPAO_TOKEN', value) : localStorage.removeItem('PAOPAO_TOKEN'), token);
      await page.goto(fixture.base + target);
    }
    await page.goto(`${fixture.base}/#/u?s=${fixture.operator}`);
    await profileTab('专栏').click();
    await expect(rows).toHaveCount(20);
    await settled();
    await page.screenshot({ path: path.join(output, 'profile-public-articles.png'), fullPage: false });
    await page.locator('.author-columns .n-pagination-item').filter({ hasText: /^2$/ }).click();
    await expect(rows).toHaveCount(Math.min(20, operatorArticleCount + 1));
    checks++;
    await profileTab('合集').click();
    await expect(rows).toHaveCount(20);
    await settled();
    await page.screenshot({ path: path.join(output, 'profile-public-collections.png'), fullPage: false });
    await page.locator('.author-columns .n-pagination-item').filter({ hasText: /^2$/ }).click();
    await expect(rows).toHaveCount(Math.min(20, operatorCollectionCount + 1));
    checks++;
    await rows.getByRole('link').first().click();
    await expect(page.locator('article > h1')).toHaveText(collections[0].title);
    checks++;

    await page.goto(`${fixture.base}/#/u?s=${fixture.names.a}`);
    await profileTab('专栏').click();
    await expect(rows).toHaveCount(Math.min(20, authorArticleCount));
    await expect(rows.filter({ hasText: `PendingArticle${stamp}` })).toHaveCount(0);
    checks++;
    await profileTab('合集').click();
    await expect(rows).toHaveCount(Math.min(20, authorCollectionCount + 1));
    await expect(rows.filter({ hasText: ownerCollectionTitle })).toHaveCount(1);
    await expect(rows.filter({ hasText: pendingName })).toHaveCount(0);
    checks++;

    await identity(author, '/#/profile');
    await profileTab('专栏').click();
    await expect(rows).toHaveCount(Math.min(20, authorArticleCount));
    await expect(rows.filter({ hasText: `PendingArticle${stamp}` })).toHaveCount(0);
    await profileTab('合集').click();
    await expect(rows).toHaveCount(Math.min(20, authorCollectionCount + 1));
    await expect(rows.filter({ hasText: ownerCollectionTitle })).toHaveCount(1);
    checks++;

    // An old article response must not overwrite the newly selected collections.
    let release, captured;
    const gate = new Promise(resolve => { release = resolve; });
    const started = new Promise(resolve => { captured = resolve; });
    let held = false;
    await page.route('**/v1/columns?**', async route => {
      const query = new URL(route.request().url()).searchParams;
      if (!held && query.get('kind') === 'article' && query.get('author_id') === String(fixture.ids.a)) {
        held = true;
        const response = await route.fetch();
        captured();
        await gate;
        await route.fulfill({ response });
      } else await route.continue();
    });
    await profileTab('专栏').click();
    await started;
    await profileTab('合集').click();
    await expect(rows.filter({ hasText: ownerCollectionTitle })).toHaveCount(1);
    const oldResponse = page.waitForResponse(response => {
      const query = new URL(response.url()).searchParams;
      return response.url().includes('/v1/columns?') && query.get('kind') === 'article';
    });
    release();
    await oldResponse;
    await settled();
    await expect(rows.filter({ hasText: ownerCollectionTitle })).toHaveCount(1);
    await page.unroute('**/v1/columns?**');
    checks++;

    await page.route('**/v1/columns?**', route => route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 50000, msg: 'Temporary test failure' }) }));
    await profileTab('专栏').click();
    await expect(page.locator('.author-columns .n-alert')).toBeVisible();
    await page.unroute('**/v1/columns?**');
    await page.locator('.author-columns').getByRole('button', { name: '重试', exact: true }).click();
    await expect(rows).toHaveCount(Math.min(20, authorArticleCount));
    await expect(page.locator('.author-columns .n-alert')).toHaveCount(0);
    await profileTab('合集').click();
    await expect(rows.filter({ hasText: ownerCollectionTitle })).toHaveCount(1);
    await expect(page.getByText('Temporary test failure', { exact: true })).toHaveCount(0);
    checks++;

    for (const width of [375, 390, 821, 1366, 1920]) {
      await page.setViewportSize({ width, height: 900 });
      await settled();
      if (await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1)) throw new Error(`Profile overflow at ${width}px`);
      checks++;
    }
    await page.setViewportSize({ width: 390, height: 844 });
    await page.screenshot({ path: path.join(output, 'profile-collections-mobile.png'), fullPage: true });
    await identity('', '/#/columns?tab=collections');
    const search = page.getByPlaceholder('搜索合集名称或简介'), results = page.locator('.column-row');
    async function searchFor(keyword) {
      await search.fill(keyword);
      await page.getByRole('button', { name: '搜索合集', exact: true }).click();
      await settled();
    }
    await searchFor(intro);
    await expect(results).toHaveCount(20);
    await page.locator('.n-pagination-item').filter({ hasText: /^2$/ }).click();
    await expect(results).toHaveCount(1);
    checks++;
    await searchFor(collections[0].title);
    await expect(results).toHaveCount(1);
    await expect(results).toContainText(collections[0].title);
    checks++;
    await searchFor(bodyKeyword);
    await expect(results).toHaveCount(0);
    checks++;
    await searchFor(pendingName);
    await expect(results).toHaveCount(0);
    checks++;
    await searchFor(`OriginalIntroduction${stamp}`);
    await expect(results).toHaveCount(1);
    await expect(results).toContainText(ownerCollectionTitle);
    checks++;
    await page.screenshot({ path: path.join(output, 'collection-search-mobile.png'), fullPage: true });
    await page.setViewportSize({ width: 1366, height: 1000 });
    await searchFor(collections[0].title);
    await page.screenshot({ path: path.join(output, 'collection-search.png'), fullPage: true });
    if (errors.length) throw new Error(errors.join('; '));
    console.log(`Column discovery checks passed: ${checks}; guest and own profiles, public-only content, pagination, stale responses, collection name/introduction search and responsive layouts.`);
  } finally {
    await browser.close();
  }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
