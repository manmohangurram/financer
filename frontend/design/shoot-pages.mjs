// Capture every page at desktop and mobile widths into frontend/design/shots.
// Usage (servers must be running): node frontend/design/shoot-pages.mjs
import { spawn } from 'node:child_process';
import { mkdir, writeFile } from 'node:fs/promises';
import path from 'node:path';

const CHROME = 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';
const OUT = path.resolve('frontend/design/shots');
const PORT = 9241;
const PROFILE = path.join(process.env.TEMP ?? '.', 'financer-pages-cdp');
const BASE = process.env.BASE_URL ?? 'http://localhost:5173';
const API = 'http://127.0.0.1:8080';
const THEME = process.argv[2] ?? 'dark';
// Optional name filter: `node … dark spending` captures only the matching routes.
const ONLY = process.argv[3] ?? '';
// Optional click target: `SHOOT_CLICK="HDFC Main" node … dark accounts` clicks it
// after the page settles, so a selected state can be photographed.
const CLICK = process.env.SHOOT_CLICK ?? '';
// Optional key presses after the clicks, comma separated: `SHOOT_KEY="ArrowRight,ArrowDown"`.
const KEYS = process.env.SHOOT_KEY ?? '';
// Optional scroll offset into the page's own scroll container, for content below
// the fold on mobile: `SHOOT_SCROLL=900 node … dark spending`.
const SCROLL = Number(process.env.SHOOT_SCROLL ?? 0);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const ROUTES = [
  ['dashboard', '/dashboard'],
  ['accounts', '/accounts'],
  ['accounts-manage', '/accounts/manage'],
  ['spending', '/accounts/spending'],
  ['categories', '/accounts/categories'],
  ['rules', '/accounts/rules'],
  ['settings', '/settings'],
  ['ui-kit', '/ui-kit'],
  ['login', '/login']
];
const VIEWS = [
  ['desktop', 1440, 1000, 1, false],
  ['mobile', 390, 844, 2, true]
];

const CREDS = {
  name: 'Demo',
  email: 'demo@financer.app',
  password: 'password123'
};
// The dev database is empty on a fresh checkout: create the demo account once,
// then sign in with it.
await fetch(`${API}/api/auth/signup`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(CREDS)
}).catch(() => {});

const auth = await (
  await fetch(`${API}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email: CREDS.email, password: CREDS.password })
  })
).json();
if (!auth.accessToken) throw new Error(`login failed: ${JSON.stringify(auth)}`);

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
const errors = [];
ws.addEventListener('message', (ev) => {
  const msg = JSON.parse(ev.data);
  if (msg.id && pending.has(msg.id)) {
    const { resolve, reject } = pending.get(msg.id);
    pending.delete(msg.id);
    if (msg.error) reject(new Error(`${msg.error.code}: ${msg.error.message}`));
    else resolve(msg.result);
    return;
  }
  if (msg.method === 'Runtime.consoleAPICalled' && msg.params.type === 'error') {
    errors.push(
      msg.params.args
        .map((a) => a.value ?? a.description ?? '')
        .join(' ')
        .slice(0, 160)
    );
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
  const res = await send('Runtime.evaluate', {
    expression,
    returnByValue: true,
    awaitPromise: true
  });
  if (res.exceptionDetails) throw new Error(res.exceptionDetails.text);
  return res.result.value;
};

await send('Page.enable');
await send('Runtime.enable');
await mkdir(OUT, { recursive: true });

// Seed the session once; every later navigation reuses localStorage.
await send('Emulation.setDeviceMetricsOverride', {
  width: 1440,
  height: 900,
  deviceScaleFactor: 1,
  mobile: false
});
await send('Page.navigate', { url: `${BASE}/login` });
await sleep(1500);
await evaluate(`(() => {
  localStorage.setItem('access_token', ${JSON.stringify(auth.accessToken)});
  localStorage.setItem('refresh_token', ${JSON.stringify(auth.refreshToken ?? '')});
  localStorage.setItem('financer-theme', ${JSON.stringify(THEME)});
  return true;
})()`);

for (const [label, width, height, dpr, mobile] of VIEWS) {
  for (const [name, route] of ROUTES) {
    await send('Emulation.setDeviceMetricsOverride', {
      width,
      height,
      deviceScaleFactor: dpr,
      mobile,
      screenWidth: width,
      screenHeight: height
    });
    if (ONLY && !name.includes(ONLY)) continue;
    await send('Page.navigate', { url: `${BASE}${route}` });
    await sleep(2600);
    await evaluate(`document.documentElement.dataset.theme = ${JSON.stringify(THEME)}`);
    await evaluate('document.fonts.ready.then(() => true)');
    await sleep(350);

    if (CLICK) {
      for (const raw of CLICK.split(',')) {
        const target = raw.trim();
        if (!target) continue;
        const needle = JSON.stringify(target);
        // `css:<selector>` clicks the first element matching the selector;
        // anything else is matched against button text.
        const expr = target.startsWith('css:')
          ? '(() => { const b = document.querySelector(' +
            JSON.stringify(target.slice(4)) +
            '); if (!b) return false; b.click(); return true; })()'
          : '(() => { const b = [...document.querySelectorAll("button")].find((x) => (x.textContent || "").includes(' +
            needle +
            ')); if (!b) return false; b.click(); return true; })()';
        const clicked = await evaluate(expr);
        if (!clicked) console.log(`  ! no button matching "${target}"`);
        await sleep(1200);
      }
    }

    if (KEYS) {
      for (const raw of KEYS.split(',')) {
        const key = raw.trim();
        if (!key) continue;
        await evaluate(
          '(() => { const el = document.activeElement || document.body; el.dispatchEvent(new KeyboardEvent("keydown", { key: ' +
            JSON.stringify(key) +
            ', bubbles: true })); return true; })()'
        );
        await sleep(220);
      }
      const focused = await evaluate(
        'document.activeElement?.getAttribute("data-day") ?? document.activeElement?.getAttribute("aria-label") ?? null'
      );
      console.log(`  focus after ${KEYS}: ${focused}`);
    }

    const m = JSON.parse(
      await evaluate(`JSON.stringify({
      vw: innerWidth,
      scrollWidth: document.documentElement.scrollWidth,
      scrollHeight: document.documentElement.scrollHeight,
      title: document.querySelector('h1')?.textContent ?? null
    })`)
    );

    if (SCROLL) {
      await evaluate(
        `(() => { const el = document.querySelector('#main-content > div'); if (el) el.scrollTop = ${SCROLL}; return true; })()`
      );
      await sleep(500);
    }

    const full = SCROLL ? height : Math.max(height, Math.ceil(m.scrollHeight));
    await send('Emulation.setDeviceMetricsOverride', {
      width,
      height: full,
      deviceScaleFactor: dpr,
      mobile,
      screenWidth: width,
      screenHeight: full
    });
    await sleep(400);
    const shot = await send('Page.captureScreenshot', {
      format: 'png',
      fromSurface: true,
      captureBeyondViewport: false
    });
    const suffix = CLICK
      ? `-${CLICK.replace(/[\s,]+/g, '-')
          .replace(/[^\w-]/g, '')
          .toLowerCase()}`
      : '';
    const file = `${THEME}-${label}-${name}${suffix}${SCROLL ? `-s${SCROLL}` : ''}.png`;
    await writeFile(path.join(OUT, file), Buffer.from(shot.data, 'base64'));

    const overflow = m.scrollWidth > width + 1 ? ` OVERFLOW(+${m.scrollWidth - width}px)` : '';
    console.log(`${file}  ${m.title ?? '(no h1)'}${overflow}`);
  }
}

console.log(errors.length ? `console errors: ${errors.slice(0, 6).join(' | ')}` : 'console errors: none');
ws.close();
chrome.kill();
