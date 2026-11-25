import { execFileSync } from 'node:child_process';
import { ADMIN } from './helpers';
import { serverEnv } from './playwright.config';

/** Creates the e2e admin. The CLI migrates the database itself, so it does not matter whether the server is up yet. */
export default function globalSetup(): void {
  execFileSync('go', ['run', './cmd/patchboard', 'admin', 'create', '--username', ADMIN.username, '--name', 'E2E Admin'], {
    cwd: '..',
    input: ADMIN.password + '\n',
    env: { ...process.env, ...serverEnv },
    stdio: ['pipe', 'inherit', 'inherit'],
  });
}
