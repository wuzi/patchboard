import { expect, test } from '@playwright/test';
import { adminLogin, createPost, uid } from './helpers';

test('admin changes a status with a response and publishes an update', async ({ page, request }) => {
  const title = `Aumentar tamanho do chat ${uid()}`;
  const p = await createPost(request, title);
  await adminLogin(page);
  await page.goto(`/p/${p.id}-${p.slug}`);
  const d = page.getByRole('dialog', { name: title });
  await d.getByLabel('Status').selectOption({ label: 'Planejado' });
  const s = page.getByRole('dialog', { name: 'Alterar status' });
  await s.getByPlaceholder('Resposta da equipe').fill('Vamos fazer!');
  await s.getByLabel('Publicar como atualização').check();
  await expect(s.getByLabel('Título da atualização')).toHaveValue(`${title}: Planejado`);
  await s.getByRole('button', { name: 'Confirmar' }).click();
  await expect(s).toHaveCount(0);
  await expect(d.getByText('Vamos fazer!')).toBeVisible();
  await expect(d.getByTestId('status-chip')).toHaveText(/Planejado/);
  await page.goto('/updates');
  await expect(page.getByRole('heading', { name: `${title}: Planejado` })).toBeVisible();
});

test('admin drags a roadmap card to another column', async ({ page, request }) => {
  const u = uid();
  const title = `Arrastar ${u}`;
  await createPost(request, title);
  await adminLogin(page);
  await page.goto(`/roadmap?q=${u}`);
  const from = page.getByRole('region', { name: 'Sugestões' });
  const to = page.getByRole('region', { name: 'Em análise' });
  const handle = from.getByRole('button', { name: 'Mover para outro status' });
  const hb = (await handle.boundingBox())!;
  const tb = (await to.boundingBox())!;
  await page.mouse.move(hb.x + hb.width / 2, hb.y + hb.height / 2);
  await page.mouse.down();
  await page.mouse.move(hb.x + 30, hb.y + 30, { steps: 5 });
  await page.mouse.move(tb.x + tb.width / 2, tb.y + 90, { steps: 25 });
  await page.mouse.up();
  const s = page.getByRole('dialog', { name: 'Alterar status' });
  await expect(s.getByLabel('Status')).toHaveValue('2');
  await s.getByRole('button', { name: 'Confirmar' }).click();
  await expect(to.getByText(title)).toBeVisible();
  await expect(from.getByText(title)).toHaveCount(0);
});

test('cancelling a roadmap drop puts the card back', async ({ page, request }) => {
  const u = uid();
  const title = `Voltar ${u}`;
  await createPost(request, title);
  await adminLogin(page);
  await page.goto(`/roadmap?q=${u}`);
  const from = page.getByRole('region', { name: 'Sugestões' });
  const to = page.getByRole('region', { name: 'Planejado' });
  const hb = (await from.getByRole('button', { name: 'Mover para outro status' }).boundingBox())!;
  const tb = (await to.boundingBox())!;
  await page.mouse.move(hb.x + hb.width / 2, hb.y + hb.height / 2);
  await page.mouse.down();
  await page.mouse.move(hb.x + 30, hb.y + 30, { steps: 5 });
  await page.mouse.move(tb.x + tb.width / 2, tb.y + 90, { steps: 25 });
  await page.mouse.up();
  await page.getByRole('dialog', { name: 'Alterar status' }).getByRole('button', { name: 'Cancelar' }).click();
  await expect(from.getByText(title)).toBeVisible();
  await expect(to.getByText(title)).toHaveCount(0);
});
