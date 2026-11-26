// Screenshot a local HTML mockup at a given width, full page, both themes.
// Usage: node frontend/design/shoot-file.mjs <file> [width] [name]
// e.g.   node frontend/design/shoot-file.mjs frontend/design/preview/pagination-options.html 1440 pagination
import { spawn } from 'node:child_process';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const CHROME = 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';
const OUT = path.resolve('frontend/design/shots');
const PORT = 9251;
const PROFILE = path.join(process.env.TEMP ?? '.', 'financer-file-cdp');
const file = path.resolve(process.argv[2] ?? '');
const width = Number(process.argv[3] ?? 1440);
const name = process.argv[4] ?? path.basename(file, '.html');
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const chrome = spawn(
  CHROME,
  [
    '--headless=new',
    '--disable-gpu',
    '--hide-scrollbars',
    '--no-first-run',
    '--remote-allow-origins=*',
    `--remote-debugging-port=${PORT}`,
    `--user-data-dir=${PROFILE}`,
    'about:blank'
  ],
  { stdio: 'ignore', windowsHide: true }
);
for (let i = 0; i < 100; i++) {
  try {
    if ((await fetch(`http://127.0.0.1:${PORT}/json/version`)).ok) break;
  } catch {}
  await sleep(100);
}

const targets = await (await fetch(`http://127.0.0.1:${PORT}/json`)).json();
const ws = new WebSocket(targets.find((t) => t.type === 'page').webSocketDebuggerUrl);
let nextId = 1;
const pending = new Map();
ws.addEventListener('message', (ev) => {
  const msg = JSON.parse(ev.data);
  if (msg.id && pending.has(msg.id)) {
    const { resolve, reject } = pending.get(msg.id);
    pending.delete(msg.id);
    if (msg.error) reject(new Error(`${msg.error.code}: ${msg.error.message}`));
    else resolve(msg.result);
  }
});
await new Promise((resolve, reject) => {
  ws.addEventListener('open', resolve, { once: true });
  ws.addEventListener('error', reject, { once: true });
});
const send = (method, params = {}) => {
  const id = nextId++;
  return new Promise((resolve, reject) => {
    pending.set(id, { resolve, reject });
    ws.send(JSON.stringify({ id, method, params }));
  });
};
const evaluate = async (expression) => {
  const res = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true });
  if (res.exceptionDetails) throw new Error(res.exceptionDetails.text);
  return res.result.value;
};

await send('Page.enable');
await send('Runtime.enable');
await mkdir(OUT, { recursive: true });

for (const theme of ['dark', 'light']) {
  await send('Emulation.setDeviceMetricsOverride', {
    width,
    height: 1000,
    deviceScaleFactor: 1,
    mobile: width < 600
  });
  await send('Page.navigate', { url: `${pathToFileURL(file).href}?shot=${theme}` });
  await sleep(700);
  await evaluate(`document.body.classList.toggle('light', ${theme === 'light'})`);
  await evaluate('document.fonts.ready.then(() => true)');

  const height = await evaluate('Math.ceil(document.documentElement.scrollHeight)');
  await send('Emulation.setDeviceMetricsOverride', {
    width,
    height,
    deviceScaleFactor: 1,
    mobile: width < 600
  });
  await sleep(300);
  const shot = await send('Page.captureScreenshot', { format: 'png', fromSurface: true });
  const out = `${theme}-${name}-${width}.png`;
  await writeFile(path.join(OUT, out), Buffer.from(shot.data, 'base64'));
  console.log(`${out}  ${width}x${height}`);
}

ws.close();
chrome.kill();
