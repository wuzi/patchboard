import { expect, test } from '@playwright/test';
import { adminApi, adminLogin, createPost, uid } from './helpers';

test('admin re-tags a post from the post view', async ({ page, request }) => {
  const title = `Retag ${uid()}`;
  const p = await createPost(request, title);
  await adminLogin(page);
  await page.goto(`/p/${p.id}-${p.slug}`);
  const d = page.getByRole('dialog', { name: title });
  await d.getByLabel('Tag', { exact: true }).selectOption({ label: 'Bug' });
  await expect(d.locator('.chips').getByText('Bug', { exact: true })).toBeVisible();
  await d.getByLabel('Tag', { exact: true }).selectOption({ label: 'Sem tag' });
  await expect(d.locator('.chips').getByText('Bug', { exact: true })).toHaveCount(0);
});

test('admin bans a comment author from the post view', async ({ page, request, playwright }) => {
  const title = `Troll ${uid()}`;
  const p = await createPost(request, title);
  const troll = await playwright.request.newContext({ baseURL: 'http://localhost:8099', extraHTTPHeaders: { 'X-PB-CSRF': '1' } });
  expect((await troll.put('/api/me/name', { data: { name: 'Troll' } })).status()).toBe(200);
  expect((await troll.post(`/api/posts/${p.id}/comments`, { data: { body: 'spam spam' } })).status()).toBe(201);
  const admin = await adminApi(playwright);
  try {
    await adminLogin(page);
    await page.goto(`/p/${p.id}-${p.slug}`);
    const d = page.getByRole('dialog', { name: title });
    await d.getByRole('button', { name: 'Banir autor' }).first().click();
    const form = d.getByTestId('ban-form');
    await form.getByLabel('Também bloquear a rede (IP)').uncheck(); // every e2e client shares 127.0.0.1
    await form.getByRole('button', { name: 'Banir' }).click();
    await expect(page.getByText('Salvo')).toBeVisible();
    expect((await troll.post(`/api/posts/${p.id}/comments`, { data: { body: 'more spam' } })).status()).toBe(403);
  } finally {
    const bans = await (await admin.get('/api/admin/bans')).json();
    for (const b of bans) await admin.delete(`/api/admin/bans/${b.id}`);
  }
});
