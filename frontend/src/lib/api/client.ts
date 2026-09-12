import { getAccessToken } from './transport';

// window.__API_BASE__ is injected at runtime by the Go server from the
// FINANCER_DOMAIN_URL env (empty → same-origin). VITE_API_URL/PROD fallbacks
// cover dev and non-container production builds.
export const API_BASE: string =
  (window as any).__API_BASE__ ||
  import.meta.env.VITE_API_URL ||
  (import.meta.env.PROD ? '' : 'http://localhost:8080');

export interface AccountServiceClient {
  createAccount: (req: unknown) => Promise<any>;
  updateAccount: (req: unknown) => Promise<any>;
  deleteAccount: (req: unknown) => Promise<any>;
  listAccounts: (req: unknown) => Promise<any>;
}

export interface CategoryServiceClient {
  createCategories: (req: unknown) => Promise<any>;
  updateCategories: (req: unknown) => Promise<any>;
  deleteCategories: (req: unknown) => Promise<any>;
  listCategories: (req: unknown) => Promise<any>;
}

export interface TransactionServiceClient {
  createTransactions: (req: unknown) => Promise<any>;
  updateTransactions: (req: unknown) => Promise<any>;
  deleteTransactions: (req: unknown) => Promise<any>;
  listTransactions: (req: unknown) => Promise<any>;
  importFile: (file: File) => Promise<{ id: string; headers: string[]; rowCount: number }>;
  commitImportFile: (req: unknown) => Promise<any>;
}

export interface RuleServiceClient {
  createRule: (req: unknown) => Promise<any>;
  updateRule: (req: unknown) => Promise<any>;
  deleteRule: (req: unknown) => Promise<any>;
  listRules: (req: unknown) => Promise<any>;
  previewRule: (req: unknown) => Promise<any>;
  runRule: (req: unknown) => Promise<any>;
}

export interface TransferServiceClient {
  linkTransfers: (req: unknown) => Promise<any>;
  unlinkTransfers: (req: unknown) => Promise<any>;
  createCounterpart: (req: unknown) => Promise<any>;
}

export interface InvestmentServiceClient {
  createInvestment: (req: unknown) => Promise<any>;
  listInvestments: (req: unknown) => Promise<any>;
  getInvestment: (req: unknown) => Promise<any>;
  updateInvestment: (req: unknown) => Promise<any>;
  deleteInvestment: (req: unknown) => Promise<any>;
  addLot: (req: unknown) => Promise<any>;
  updateLot: (req: unknown) => Promise<any>;
  deleteLot: (req: unknown) => Promise<any>;
  listLots: (req: unknown) => Promise<any>;
  importInvestments: (req: unknown) => Promise<any>;
  importFile: (file: File) => Promise<{ id: string; headers: string[]; rowCount: number }>;
  commitImportFile: (req: unknown) => Promise<any>;
  searchSymbols: (req: unknown) => Promise<any>;
  refreshPrices: (req: unknown) => Promise<any>;
  getPortfolioSummary: (req: unknown) => Promise<any>;
  getPriceHistory: (req: unknown) => Promise<any>;
}

export interface AnalyticsServiceClient {
  dashboard: (req: unknown) => Promise<any>;
  spending: (req: unknown) => Promise<any>;
}

export interface ProfileServiceClient {
  getProfile: (req: unknown) => Promise<any>;
  updateProfile: (req: unknown) => Promise<any>;
  changePassword: (req: unknown) => Promise<any>;
  logoutAll: (req: unknown) => Promise<any>;
}

export interface UserKeyServiceClient {
  listKeys: (req: unknown) => Promise<any>;
  createKey: (req: unknown) => Promise<any>;
  deleteKey: (req: unknown) => Promise<any>;
}

let _accounts: AccountServiceClient | null = null;
let _categories: CategoryServiceClient | null = null;
let _transactions: TransactionServiceClient | null = null;
let _rules: RuleServiceClient | null = null;
let _transfers: TransferServiceClient | null = null;
let _investments: InvestmentServiceClient | null = null;
let _analytics: AnalyticsServiceClient | null = null;
let _profile: ProfileServiceClient | null = null;
let _userKeys: UserKeyServiceClient | null = null;

function api(method: 'GET' | 'POST' | 'PUT' | 'DELETE', path: string) {
  return (req: unknown) => {
    const rest: Record<string, any> = { ...(req || {}) };
    let url = API_BASE + path.replace(/\{([^}]+)\}/g, (_, k: string) => {
      const v = rest[k];
      delete rest[k];
      return encodeURIComponent(v ?? '');
    });
    let body: string | undefined;

    if (method === 'GET') {
      const qs = new URLSearchParams();
      for (const [k, v] of Object.entries(rest)) {
        if (v === undefined || v === null || v === '') continue;
        qs.set(k, Array.isArray(v) ? v.join(',') : String(v));
      }
      const s = qs.toString();
      if (s) url += '?' + s;
    } else {
      body = JSON.stringify(rest);
    }

    return fetch(url, {
      method,
      headers: { 'Content-Type': 'application/json', ...getAuthHeaders() },
      ...(body !== undefined ? { body } : {})
    }).then(async (r) => {
      if (r.status === 204) return undefined;
      const data = await r.json().catch(() => ({}));
      if (!r.ok) {
        const err = new Error(data?.message || `Request failed with ${r.status}`);
        (err as any).status = r.status;
        throw err;
      }
      return data;
    });
  };
}

export function accounts(): AccountServiceClient {
  if (!_accounts) {
    _accounts = {
      createAccount: api('POST', '/api/accounts'),
      updateAccount: api('PUT', '/api/accounts/{id}'),
      deleteAccount: api('DELETE', '/api/accounts/{id}'),
      listAccounts: api('GET', '/api/accounts')
    };
  }
  return _accounts;
}

export function categories(): CategoryServiceClient {
  if (!_categories) {
    _categories = {
      createCategories: api('POST', '/api/categories'),
      updateCategories: api('PUT', '/api/categories'),
      deleteCategories: api('DELETE', '/api/categories'),
      listCategories: api('GET', '/api/categories')
    };
  }
  return _categories;
}

/** Upload a CSV/XLSX/PDF to an import endpoint; returns headers + row count. */
async function uploadImportFile(path: string, file: File): Promise<{ id: string; headers: string[]; rowCount: number }> {
  const fd = new FormData();
  fd.append('file', file);
  const resp = await fetch(API_BASE + path, {
    method: 'POST',
    headers: getAuthHeaders(),
    body: fd
  });
  const body = await resp.json().catch(() => ({}));
  if (!resp.ok) throw new Error(body.message || 'Could not read that file');
  return { id: body.id || '', headers: body.headers || [], rowCount: body.rowCount || 0 };
}

const importFile = (file: File) => uploadImportFile('/api/transactions/import/file', file);
const importInvestmentFile = (file: File) => uploadImportFile('/api/investments/import/file', file);

export function transactions(): TransactionServiceClient {
  if (!_transactions) {
    _transactions = {
      createTransactions: api('POST', '/api/transactions'),
      updateTransactions: api('PUT', '/api/transactions'),
      deleteTransactions: api('DELETE', '/api/transactions'),
      listTransactions: api('GET', '/api/transactions'),
      importFile,
      commitImportFile: api('POST', '/api/transactions/import/file/commit')
    };
  }
  return _transactions;
}

export function rules(): RuleServiceClient {
  if (!_rules) {
    _rules = {
      createRule: api('POST', '/api/rules'),
      updateRule: api('PUT', '/api/rules/{id}'),
      deleteRule: api('DELETE', '/api/rules/{id}'),
      listRules: api('GET', '/api/rules'),
      previewRule: api('POST', '/api/rules/preview'),
      runRule: api('POST', '/api/rules/{id}/run')
    };
  }
  return _rules;
}

export function transfers(): TransferServiceClient {
  if (!_transfers) {
    _transfers = {
      linkTransfers: api('POST', '/api/transfer-links'),
      unlinkTransfers: api('DELETE', '/api/transfer-links'),
      createCounterpart: api('POST', '/api/transfer-links/counterpart')
    };
  }
  return _transfers;
}

export function investments(): InvestmentServiceClient {
  if (!_investments) {
    _investments = {
      createInvestment: api('POST', '/api/investments'),
      listInvestments: api('GET', '/api/investments'),
      getInvestment: api('GET', '/api/investments/{id}'),
      updateInvestment: api('PUT', '/api/investments/{id}'),
      deleteInvestment: api('DELETE', '/api/investments/{id}'),
      addLot: api('POST', '/api/investments/{investmentId}/lots'),
      updateLot: api('PUT', '/api/investments/{id}/lots/{lotId}'),
      deleteLot: api('DELETE', '/api/investments/{id}/lots/{lotId}'),
      listLots: api('GET', '/api/investments/{id}/lots'),
      importInvestments: api('POST', '/api/investments/import'),
      importFile: importInvestmentFile,
      commitImportFile: api('POST', '/api/investments/import/file/commit'),
      searchSymbols: api('GET', '/api/investments/search'),
      refreshPrices: api('POST', '/api/investments/refresh-prices'),
      getPortfolioSummary: api('GET', '/api/portfolio/summary'),
      getPriceHistory: api('GET', '/api/investments/{id}/price-history')
    };
  }
  return _investments;
}

export function analytics(): AnalyticsServiceClient {
  if (!_analytics) {
    _analytics = {
      dashboard: api('GET', '/api/dashboard'),
      spending: api('GET', '/api/spending')
    };
  }
  return _analytics;
}

export function profile(): ProfileServiceClient {
  if (!_profile) {
    _profile = {
      getProfile: api('GET', '/api/me/profile'),
      updateProfile: api('PUT', '/api/me/profile'),
      changePassword: api('POST', '/api/me/password'),
      logoutAll: api('POST', '/api/me/logout-all')
    };
  }
  return _profile;
}

export function apiKeys(): UserKeyServiceClient {
  if (!_userKeys) {
    _userKeys = {
      listKeys: api('GET', '/api/me/keys'),
      createKey: api('POST', '/api/me/keys'),
      deleteKey: api('DELETE', '/api/me/keys/{id}')
    };
  }
  return _userKeys;
}

function getAuthHeaders(): Record<string, string> {
  if (typeof window === 'undefined') return {};
  const token = getAccessToken();
  return token ? { Authorization: `Bearer ${token}` } : {};
}
