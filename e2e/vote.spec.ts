import { expect, test } from '@playwright/test';
import { createPost, uid } from './helpers';

test('a player votes and unvotes from the list', async ({ page, request }) => {
  const title = `Aumentar tamanho do chat ${uid()}`;
  await createPost(request, title);
  await page.goto('/');
  const card = page.getByTestId('post-card').filter({ hasText: title });
  const vote = card.getByRole('button', { name: /vot/i });
  await expect(vote).toHaveText('0');
  await vote.click();
  await expect(vote).toHaveText('1');
  await expect(vote).toHaveAttribute('aria-pressed', 'true');
  await page.reload();
  await expect(vote).toHaveText('1');
  await expect(vote).toHaveAttribute('aria-pressed', 'true');
  await vote.click();
  await expect(vote).toHaveText('0');
  await expect(page).toHaveURL(/\/$/);
});

test('list sorts and searches', async ({ page, request }) => {
  const tag = uid();
  await createPost(request, `Chat maior ${tag}`);
  await createPost(request, `Outra ideia ${tag}`);
  await page.goto(`/?q=${tag}`);
  await expect(page.getByTestId('post-card')).toHaveCount(2);
  await page.goto(`/?q=chat%20${tag}`);
  await expect(page.getByTestId('post-card')).toHaveCount(1);
  await expect(page.getByTestId('post-card')).toContainText('Chat maior');
  await page.getByRole('tab', { name: 'Mais votados' }).click();
  await expect(page).toHaveURL(/sort=top/);
  await expect(page).toHaveURL(/q=chat/);
  await page.getByRole('button', { name: 'Buscar' }).click();
  await page.getByPlaceholder('Buscar posts…').fill(`ideia ${tag}`);
  await expect(page.getByTestId('post-card')).toHaveCount(1);
  await expect(page.getByTestId('post-card')).toContainText('Outra ideia');
});
