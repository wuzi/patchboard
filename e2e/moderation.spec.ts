import { expect, test } from '@playwright/test';
import { adminApi, adminLogin, uid } from './helpers';

test('pending posts are approved from the moderation page', async ({ page, playwright }) => {
  const admin = await adminApi(playwright);
  const board = await (await admin.get('/api/board')).json();
  const settings = { title: board.title, description: board.description, teamName: board.teamName, accentColor: board.accentColor,
    logoUploadId: null, uploadsEnabled: board.uploadsEnabled };
  await admin.put('/api/admin/settings', { data: { ...settings, requireApproval: true } });
  try {
    const title = `Pendente ${uid()}`;
    const guest = await playwright.request.newContext({ baseURL: 'http://localhost:8099', extraHTTPHeaders: { 'X-PB-CSRF': '1' } });
    expect((await guest.post('/api/posts', { data: { title } })).status()).toBe(201);
    await adminLogin(page);
    await page.goto('/admin/moderation');
    const row = page.getByTestId('pending-row').filter({ hasText: title });
    await row.getByRole('button', { name: 'Aprovar' }).click();
    await expect(row).toHaveCount(0);
    await page.goto(`/?q=${encodeURIComponent(title)}`);
    await expect(page.getByTestId('post-card')).toContainText(title);
  } finally {
    await admin.put('/api/admin/settings', { data: { ...settings, requireApproval: false } });
  }
});
