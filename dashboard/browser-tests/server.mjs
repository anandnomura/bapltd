import { spawn, spawnSync } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
const work = mkdtempSync(join(tmpdir(), 'bap-browser-'));
const binary = join(work, process.platform === 'win32' ? 'bapcontrolplane.exe' : 'bapcontrolplane');
const root = resolve('..');
const build = spawnSync('go', ['build', '-o', binary, './cmd/server'], { cwd: join(root, 'bap-controlplane'), stdio: 'inherit', windowsHide: true });
if (build.status !== 0) process.exit(build.status || 1);
const server = spawn(binary, ['-port', '18480', '-db', 'memory', '-policy', join(root, 'bap-edge/policy.cedar'), '-schema', join(root, 'bap-edge/schema.json')], { cwd: work, env: { ...process.env, BAP_ADMIN_TOKEN: 'browser-test-admin', BAP_SECRET_KEY: 'browser-test-signing-secret' }, stdio: 'inherit', windowsHide: true });
function stop() { server.kill(); }
process.on('SIGTERM', stop); process.on('SIGINT', stop); process.on('exit', stop);
server.on('exit', code => process.exit(code || 0));
