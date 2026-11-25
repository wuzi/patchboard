import { expect, test } from '@playwright/test';
import { adminApi, createPost, uid } from './helpers';

test('roadmap groups posts by status and follows the language', async ({ page, request, playwright }) => {
  const admin = await adminApi(playwright);
  const u = uid();
  const p = await createPost(request, `Transferência de tiers ${u}`);
  const r = await admin.post(`/api/admin/posts/${p.id}/status`, { data: { statusId: 3 } });
  expect(r.status()).toBe(200);
  await page.goto(`/roadmap?q=${u}`);
  const col = page.getByRole('region', { name: 'Planejado' });
  await expect(col.getByText(`Transferência de tiers ${u}`)).toBeVisible();
  await expect(col.getByTestId('column-count')).toHaveText('1');
  await expect(page.getByRole('region', { name: 'Sugestões' }).getByTestId('column-count')).toHaveText('0');
  await expect(page.getByRole('region', { name: 'Sugestões' }).getByText('Nada aqui ainda')).toBeVisible();
  await page.evaluate(() => localStorage.setItem('pb_lang', 'en'));
  await page.reload();
  await expect(page.getByRole('region', { name: 'Planned' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Roadmap' })).toHaveAttribute('aria-current', 'page');
});

test('the default column offers "add a post"', async ({ page }) => {
  await page.goto('/roadmap');
  await page.getByRole('region', { name: 'Sugestões' }).getByRole('button', { name: 'Adicionar post' }).click();
  await expect(page.getByRole('dialog', { name: 'Novo post' })).toBeVisible();
});
