import { expect, type APIRequestContext, type Page, type Playwright } from '@playwright/test';

export const ADMIN = { username: 'e2e', password: 'e2e-password-1' };
const BASE = 'http://localhost:8099';

export type PostDetail = { id: number; slug: string; title: string; statusId: number };

/** Unique suffix so tests sharing one server don't see each other's posts in searches. */
export const uid = (): string => Math.random().toString(36).slice(2, 8);

async function loginApi(request: APIRequestContext): Promise<void> {
  const r = await request.post('/api/admin/login', { data: ADMIN });
  expect(r.status()).toBe(200);
}

/** Creates a post as the e2e admin (admins are exempt from the per-IP post limit). */
export async function createPost(request: APIRequestContext, title: string, body = ''): Promise<PostDetail> {
  await loginApi(request);
  const r = await request.post('/api/posts', { data: { title, body } });
  expect(r.status(), await r.text()).toBe(201);
  return r.json();
}

/** A fresh API context logged in as the e2e admin. */
export async function adminApi(playwright: Playwright): Promise<APIRequestContext> {
  const ctx = await playwright.request.newContext({ baseURL: BASE, extraHTTPHeaders: { 'X-PB-CSRF': '1' } });
  await loginApi(ctx);
  return ctx;
}

/** Signs the browser page in as the e2e admin. */
export async function adminLogin(page: Page): Promise<void> {
  const r = await page.request.post('/api/admin/login', { data: ADMIN });
  expect(r.status()).toBe(200);
}
