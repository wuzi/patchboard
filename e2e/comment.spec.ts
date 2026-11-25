import { expect, test } from '@playwright/test';
import { createPost, uid } from './helpers';

test('comment, reply, like, close', async ({ page, request }) => {
  const title = `Aumentar tamanho do chat ${uid()}`;
  const post = await createPost(request, title, 'O chat é **pequeno** demais.');
  await page.goto(`/p/${post.id}-wrong-slug`);
  await expect(page).toHaveURL(new RegExp(`/p/${post.id}-${post.slug}$`));
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByRole('heading', { name: title })).toBeVisible();
  await expect(dialog.locator('strong', { hasText: 'pequeno' })).toBeVisible();

  await dialog.getByPlaceholder('Adicione um comentário').fill('**pls**, meu olhos agradecem');
  await dialog.getByPlaceholder('Seu nome (opcional)').fill('WuzeraProMAX');
  await dialog.getByRole('button', { name: 'Comentar' }).click();
  await expect(dialog.locator('.thread strong', { hasText: 'pls' })).toBeVisible();
  await expect(dialog.getByText('WuzeraProMAX')).toBeVisible();

  await dialog.getByRole('button', { name: 'Responder' }).first().click();
  const reply = dialog.getByTestId('reply-form');
  await reply.getByPlaceholder('Adicione um comentário').fill('concordo');
  await reply.getByRole('button', { name: 'Comentar' }).click();
  await expect(dialog.getByTestId('replies').getByText('concordo')).toBeVisible();

  const like = dialog.getByRole('button', { name: 'Curtir' }).first();
  await like.click();
  await expect(dialog.getByRole('button', { name: 'Descurtir' }).first()).toHaveText('1');

  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole('dialog')).toHaveCount(0);
  const card = page.getByTestId('post-card').filter({ hasText: title });
  await expect(card).toContainText('2');
});

test('markdown preview renders like the final post', async ({ page, request }) => {
  const post = await createPost(request, `Prévia ${uid()}`);
  await page.goto(`/p/${post.id}-${post.slug}`);
  const dialog = page.getByRole('dialog');
  await dialog.getByPlaceholder('Adicione um comentário').fill('# Título\n\n- item');
  await dialog.getByRole('tab', { name: 'Visualizar' }).click();
  await expect(dialog.locator('.preview h1')).toHaveText('Título');
  await expect(dialog.locator('.preview li')).toHaveText('item');
});
