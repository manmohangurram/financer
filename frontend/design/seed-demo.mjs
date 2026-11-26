// Seed the demo account with one believable month, so the redesigned pages have
// real content to render (the dashboard is empty without accounts).
//
//   node frontend/design/seed-demo.mjs           # top up / no-op if already seeded
//   node frontend/design/seed-demo.mjs --reset   # wipe the demo user's data first
//
// The API must be running on FINANCER_API (default http://127.0.0.1:8080).
const API = process.env.FINANCER_API ?? 'http://127.0.0.1:8080';
const RESET = process.argv.includes('--reset');
const CREDS = { name: 'Demo', email: 'demo@financer.app', password: 'password123' };

async function call(method, path, body, token) {
  const res = await fetch(`${API}${path}`, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {})
    },
    body: body === undefined ? undefined : JSON.stringify(body)
  });
  const text = await res.text();
  const data = text ? JSON.parse(text) : null;
  if (!res.ok) throw new Error(`${method} ${path} → ${res.status} ${text.slice(0, 220)}`);
  return data;
}

const ACCOUNTS = [
  { key: 'main', bankName: 'HDFC Bank', nickname: 'HDFC Main', endingNumbers: '4821', type: 'CURRENT' },
  { key: 'fund', bankName: 'SBI', nickname: 'Emergency Fund', endingNumbers: '9930', type: 'SAVINGS' },
  { key: 'card', bankName: 'ICICI Bank', nickname: 'ICICI Platinum', endingNumbers: '3344', type: 'CREDIT_CARD' }
];

const CATEGORIES = [
  { name: 'Rent', monthlyBudget: 25000 },
  { name: 'Groceries', monthlyBudget: 12000 },
  { name: 'Dining', monthlyBudget: 6000 },
  { name: 'Transport', monthlyBudget: 4000 },
  { name: 'Utilities', monthlyBudget: 3500 },
  { name: 'Shopping', monthlyBudget: 5000 },
  { name: 'Health', monthlyBudget: 3000 },
  { name: 'Salary', monthlyBudget: 0 }
];

// [date, name, amount, type, account, category, refNo]
const TXNS = [
  // --- November (the month the dashboard renders) ---
  ['2026-11-01', 'Monthly rent', 25000, 'DEBIT', 'main', 'Rent', 'NEFT-1101-RENT'],
  ['2026-11-01', 'Salary — Acme Corp', 145000, 'CREDIT', 'main', 'Salary', 'NEFT-1101-SAL'],
  ['2026-11-03', 'Big Bazaar', 4820, 'DEBIT', 'main', 'Groceries', 'POS-1103-8821'],
  ['2026-11-05', 'Emergency fund top-up', 20000, 'CREDIT', 'fund', null, 'IMPS-1105-002'],
  ['2026-11-05', 'Uber', 640, 'DEBIT', 'main', 'Transport', 'UPI-1105-4410'],
  ['2026-11-07', 'Amazon — headphones', 3299, 'DEBIT', 'card', 'Shopping', 'POS-1107-1190'],
  ['2026-11-09', 'Electricity bill', 2180, 'DEBIT', 'main', 'Utilities', 'BILL-1109-771'],
  ['2026-11-11', 'Swiggy', 890, 'DEBIT', 'card', 'Dining', 'UPI-1111-2231'],
  ['2026-11-13', 'More Supermarket', 3150, 'DEBIT', 'card', 'Groceries', 'POS-1113-5567'],
  ['2026-11-15', 'Fuel — HP pump', 2400, 'DEBIT', 'main', 'Transport', 'POS-1115-3390'],
  ['2026-11-17', 'Apollo Pharmacy', 1250, 'DEBIT', 'main', 'Health', 'POS-1117-7724'],
  ['2026-11-18', 'PVR — evening show', 760, 'DEBIT', 'card', 'Dining', 'POS-1118-1188'],
  ['2026-11-20', 'Freelance invoice', 18000, 'CREDIT', 'main', null, 'NEFT-1120-045'],
  ['2026-11-21', 'Big Bazaar', 2650, 'DEBIT', 'main', 'Groceries', 'POS-1121-4412'],
  ['2026-11-22', 'Broadband bill', 999, 'DEBIT', 'main', 'Utilities', 'BILL-1122-336'],
  ['2026-11-23', 'Swiggy', 1120, 'DEBIT', 'card', 'Dining', 'UPI-1123-8890'],
  ['2026-11-24', 'Amazon — books', 1899, 'DEBIT', 'card', 'Shopping', 'POS-1124-6603'],
  ['2026-11-25', 'Metro card recharge', 500, 'DEBIT', 'main', 'Transport', 'POS-1125-9002'],
  ['2026-11-26', 'DMart', 1980, 'DEBIT', 'main', 'Groceries', 'POS-1126-1157'],
  ['2026-11-26', 'Third Wave Coffee', 240, 'DEBIT', 'card', 'Dining', 'UPI-1126-0044'],

  // --- October (previous month, for the comparison line) ---
  ['2026-10-01', 'Monthly rent', 25000, 'DEBIT', 'main', 'Rent', 'NEFT-1001-RENT'],
  ['2026-10-01', 'Salary — Acme Corp', 145000, 'CREDIT', 'main', 'Salary', 'NEFT-1001-SAL'],
  ['2026-10-05', 'Emergency fund top-up', 20000, 'CREDIT', 'fund', null, 'IMPS-1005-001'],
  ['2026-10-04', 'Big Bazaar', 5200, 'DEBIT', 'main', 'Groceries', 'POS-1004-2211'],
  ['2026-10-08', 'Amazon — monitor', 4100, 'DEBIT', 'card', 'Shopping', 'POS-1008-7754'],
  ['2026-10-12', 'Fuel — HP pump', 2600, 'DEBIT', 'main', 'Transport', 'POS-1012-6612'],
  ['2026-10-16', 'Swiggy', 1450, 'DEBIT', 'card', 'Dining', 'UPI-1016-3321'],
  ['2026-10-19', 'Electricity bill', 1980, 'DEBIT', 'main', 'Utilities', 'BILL-1019-556'],
  ['2026-10-23', 'More Supermarket', 3600, 'DEBIT', 'main', 'Groceries', 'POS-1023-8801'],
  ['2026-10-27', 'Barbeque Nation', 1200, 'DEBIT', 'card', 'Dining', 'POS-1027-4409']
];

async function login() {
  await call('POST', '/api/auth/signup', CREDS).catch(() => {});
  const auth = await call('POST', '/api/auth/login', {
    email: CREDS.email,
    password: CREDS.password
  });
  if (!auth?.accessToken) throw new Error('login failed');
  return auth.accessToken;
}

async function wipe(token) {
  const txns = await call('GET', '/api/transactions?pageSize=1000', undefined, token);
  if (txns.transactions.length) {
    await call('DELETE', '/api/transactions', { ids: txns.transactions.map((t) => t.id) }, token);
  }
  const { accounts } = await call('GET', '/api/accounts', undefined, token);
  for (const a of accounts) await call('DELETE', `/api/accounts/${a.id}`, undefined, token);
  const { categories } = await call('GET', '/api/categories', undefined, token);
  if (categories.length) {
    await call('DELETE', '/api/categories', { ids: categories.map((c) => c.id) }, token);
  }
  console.log(
    `reset: removed ${txns.transactions.length} transactions, ${accounts.length} accounts, ${categories.length} categories`
  );
}

const token = await login();
if (RESET) await wipe(token);

// Accounts — skip a nickname that already exists so a second run is harmless.
const existingAccounts = (await call('GET', '/api/accounts', undefined, token)).accounts;
const accountIDs = {};
for (const a of ACCOUNTS) {
  const found = existingAccounts.find((x) => x.nickname === a.nickname);
  if (found) {
    accountIDs[a.key] = found.id;
    continue;
  }
  const created = await call('POST', '/api/accounts', a, token);
  accountIDs[a.key] = created.id;
}

const existingCats = (await call('GET', '/api/categories', undefined, token)).categories;
const missing = CATEGORIES.filter((c) => !existingCats.some((x) => x.name === c.name));
if (missing.length) await call('POST', '/api/categories', { categories: missing }, token);
const categories = (await call('GET', '/api/categories', undefined, token)).categories;
const catID = (name) => categories.find((c) => c.name === name)?.id;

const existingTxns = (await call('GET', '/api/transactions?pageSize=1000', undefined, token)).transactions;
const seenRefs = new Set(existingTxns.map((t) => t.refNo).filter(Boolean));
const batch = TXNS.filter(([, , , , , , ref]) => !seenRefs.has(ref)).map(
  ([occurredAt, name, amount, type, account, category, refNo]) => ({
    name,
    amount,
    type,
    occurredAt,
    accountId: accountIDs[account],
    categoryIds: category ? [catID(category)].filter(Boolean) : [],
    refNo
  })
);
if (batch.length) {
  const out = await call('POST', '/api/transactions', { transactions: batch }, token);
  if (!out.success) throw new Error(`transaction create failed: ${JSON.stringify(out)}`);
}

const dash = await call('GET', '/api/dashboard', undefined, token);
console.log(`seeded: ${dash.accounts.length} accounts, ${batch.length} new transactions`);
console.log(`  net worth   ${dash.totalBalance}`);
console.log(`  income      ${dash.month.income}   expenses ${dash.month.expenses}   net ${dash.month.net}`);
console.log(
  `  budget      planned ${dash.month.budget.planned}, spent ${dash.month.budget.spent}, used ${Math.round(dash.month.budget.used)}%`
);
console.log(`  top category ${dash.month.topCategory?.name ?? '—'}   peak day ${dash.month.peakBucket?.label ?? '—'}`);
console.log(`  recent      ${dash.recent.length} rows, buckets ${dash.month.buckets.length}`);
