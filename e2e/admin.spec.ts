import { expect, test } from '@playwright/test';
import { ADMIN } from './helpers';

test('admin logs in, renames the board and creates an invite', async ({ page }) => {
  await page.goto('/admin');
  await expect(page).toHaveURL(/\/admin\/login$/);
  await page.getByLabel('Usuário').fill(ADMIN.username);
  await page.getByLabel('Senha').fill(ADMIN.password);
  await page.getByRole('button', { name: 'Entrar' }).click();
  await expect(page).toHaveURL(/\/admin\/settings$/);
  await page.getByLabel('Título (PT)').fill('Venore Survivors • Comunidade');
  await page.getByRole('button', { name: 'Salvar' }).click();
  await expect(page.getByText('Salvo')).toBeVisible();
  await page.goto('/');
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Venore Survivors • Comunidade');
  await page.goto('/admin/team');
  await page.getByRole('button', { name: 'Convidar' }).click();
  await expect(page.getByTestId('invite-url')).toHaveValue(/\/admin\/invite\//);
});

test('a new status can be added and shows on the roadmap', async ({ page }) => {
  await page.request.post('/api/admin/login', { data: ADMIN });
  await page.goto('/admin/statuses');
  const form = page.getByTestId('new-status');
  await form.getByLabel('Nome (PT)').fill('Em teste');
  await form.getByLabel('Nome (EN)').fill('Testing');
  await form.getByRole('button', { name: 'Adicionar' }).click();
  await expect(page.getByTestId('status-row').last().getByLabel('Nome (PT)')).toHaveValue('Em teste');
  await page.goto('/roadmap');
  await expect(page.getByRole('region', { name: 'Em teste' })).toBeVisible();
});

test('wrong password shows an error', async ({ page }) => {
  await page.goto('/admin/login');
  await page.getByLabel('Usuário').fill(ADMIN.username);
  await page.getByLabel('Senha').fill('not-the-password');
  await page.getByRole('button', { name: 'Entrar' }).click();
  await expect(page.getByText('Usuário ou senha incorretos')).toBeVisible();
  await expect(page).toHaveURL(/\/admin\/login$/);
});
