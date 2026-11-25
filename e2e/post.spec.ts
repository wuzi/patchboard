import { expect, test } from '@playwright/test';
import { createPost, uid } from './helpers';

test('create a post with a tag and an image, seeing similar posts first', async ({ page, request }) => {
  const u = uid();
  await createPost(request, `Aumentar tamanho do chat ${u}`);
  await page.goto('/');
  await page.getByRole('button', { name: 'Adicionar post' }).first().click();
  const d = page.getByRole('dialog');
  await d.getByLabel('Título').fill(`tamanho do chat ${u}`);
  await expect(d.getByRole('link', { name: new RegExp(`Aumentar tamanho do chat ${u}`) })).toBeVisible();
  await d.getByLabel('Título').fill(`Filtro de Level no market ${u}`);
  await d.getByLabel('Tag').selectOption({ label: 'Melhoria' });
  await d.locator('input[type=file]').setInputFiles('fixtures/shot.png');
  await expect(d.getByRole('img', { name: /anexo|attachment/i })).toBeVisible();
  await d.getByRole('button', { name: 'Publicar' }).click();
  await expect(page).toHaveURL(new RegExp(`/p/\\d+-filtro-de-level-no-market-${u}$`));
  const modal = page.getByRole('dialog');
  await expect(modal.getByRole('heading', { name: `Filtro de Level no market ${u}` })).toBeVisible();
  await expect(modal.getByRole('img', { name: 'Anexo 1' })).toBeVisible();
  await expect(modal.getByText('Melhoria')).toBeVisible();
});

test('validation errors are shown on the field', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'Adicionar post' }).first().click();
  const d = page.getByRole('dialog');
  await d.getByLabel('Título').fill('ab');
  await d.getByRole('button', { name: 'Publicar' }).click();
  await expect(d.getByText('Muito curto')).toBeVisible();
});
