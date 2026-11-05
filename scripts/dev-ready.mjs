import { readFileSync } from 'node:fs';
import { createConnection } from 'node:net';
import { setTimeout } from 'node:timers/promises';

// Accept a port or a JSON/TOML config without printing configuration contents.
const [mode, source, pid] = process.argv.slice(2);
let port = Number(source);
if (!Number.isInteger(port)) {
  const text = readFileSync(source, 'utf8');
  port = source.endsWith('.json') ? JSON.parse(text).port
    : Number(text.split(/^\s*\[/m)[0].match(/^port\s*=\s*(\d+)/m)?.[1]);
}
if (!Number.isInteger(port) || port < 1 || port > 65535) throw new Error('Invalid backend port');
const timeout = Number(process.env.DEV_STARTUP_TIMEOUT || 120);
if (!Number.isFinite(timeout) || timeout <= 0) throw new Error('DEV_STARTUP_TIMEOUT must be positive');
const listening = () => new Promise(resolve => {
  const socket = createConnection({ host: '127.0.0.1', port });
  const finish = value => { socket.destroy(); resolve(value); };
  socket.setTimeout(500);
  socket.once('connect', () => finish(true));
  socket.once('error', () => finish(false));
  socket.once('timeout', () => finish(false));
});
if (mode === 'check') {
  if (await listening()) throw new Error(`Backend port ${port} is occupied; stop the existing process first`);
} else if (mode === 'wait') {
  console.log(`Waiting for backend 127.0.0.1:${port} (up to ${timeout}s)…`);
  const deadline = Date.now() + timeout * 1000;
  while (true) {
    try { process.kill(Number(pid), 0); }
    catch { throw new Error('Backend exited; clients were not started'); }
    if (await listening()) break;
    if (Date.now() >= deadline) throw new Error('Backend startup timed out; clients were not started');
    await setTimeout(200);
  }
  console.log('Backend ready; starting clients.');
} else throw new Error('Expected check or wait');
