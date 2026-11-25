import { expect, test } from '@playwright/test';
import { adminApi, createPost, uid } from './helpers';

test('name persists and the recovery link restores identity in another browser', async ({ page, browser }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'Perfil' }).click();
  await page.getByLabel('Seu nome', { exact: true }).fill('WuzeraProMAX');
  await page.getByRole('button', { name: 'Salvar' }).click();
  await expect(page.getByTestId('recovery-link')).toHaveValue(/\/r\//);
  await page.reload();
  await page.getByRole('button', { name: 'Perfil' }).click();
  await expect(page.getByLabel('Seu nome', { exact: true })).toHaveValue('WuzeraProMAX');
  const link = await page.getByTestId('recovery-link').inputValue();

  const other = await (await browser.newContext({ locale: 'pt-BR' })).newPage();
  await other.goto(link);
  await expect(other.getByText('Identidade restaurada')).toBeVisible();
  await expect(other).toHaveURL(/\/$/);
  await other.getByRole('button', { name: 'Perfil' }).click();
  await expect(other.getByLabel('Seu nome', { exact: true })).toHaveValue('WuzeraProMAX');
});

test('language switcher changes the interface and is remembered', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'Perfil' }).click();
  await page.getByRole('button', { name: 'EN' }).click();
  await expect(page.getByRole('link', { name: 'Updates' })).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('lang', 'en');
  await page.reload();
  await expect(page.getByRole('link', { name: 'Updates' })).toBeVisible();
});

test('updates list shows linked post status', async ({ page, request, playwright }) => {
  const u = uid();
  const p = await createPost(request, `Dark mode ${u}`);
  const admin = await adminApi(playwright);
  expect((await admin.post(`/api/admin/posts/${p.id}/status`, { data: { statusId: 3 } })).status()).toBe(200);
  const r = await admin.post('/api/admin/updates', { data: { title: `Dark mode is planned ${u}`, body: 'Thanks for the **votes**.', postIds: [p.id] } });
  expect(r.status()).toBe(201);
  await page.goto('/updates');
  const card = page.getByRole('article').filter({ hasText: `Dark mode is planned ${u}` });
  await expect(card.locator('strong', { hasText: 'votes' })).toBeVisible();
  await expect(card.getByRole('link', { name: new RegExp(`Planejado.*Dark mode ${u}`) })).toBeVisible();
});
