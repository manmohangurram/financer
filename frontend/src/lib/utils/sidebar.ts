import { LayoutDashboard, CreditCard, BarChart3, PieChart, Tag, SlidersHorizontal, type LucideIcon } from '@lucide/vue';

export interface NavItem {
  to: string;
  label: string;
  icon: LucideIcon;
}

export interface NavSection {
  label: string | null;
  items: NavItem[];
}

export const NAV_SECTIONS: NavSection[] = [
  {
    label: null,
    items: [
      { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
      { to: '/accounts', label: 'Accounts', icon: CreditCard },
      { to: '/investments', label: 'Investments', icon: BarChart3 }
    ]
  },
  { label: 'Analysis', items: [{ to: '/accounts/spending', label: 'Spending', icon: PieChart }] },
  {
    label: 'Manage',
    items: [
      { to: '/accounts/categories', label: 'Categories', icon: Tag },
      { to: '/accounts/rules', label: 'Rules', icon: SlidersHorizontal }
    ]
  }
];

export const NAV_HREFS = NAV_SECTIONS.flatMap((s) => s.items.map((i) => i.to));

/**
 * The nav href that owns `path` — the longest match, so `/accounts/spending`
 * lights up Spending alone rather than Spending and Accounts together.
 */
export function resolveActiveHref(path: string, hrefs: string[] = NAV_HREFS): string | null {
  let best: string | null = null;
  for (const href of hrefs) {
    if (path !== href && !path.startsWith(`${href}/`)) continue;
    if (best === null || href.length > best.length) best = href;
  }
  return best;
}

/** First `limit` accounts, plus how many were left out (for the "+N more" row). */
export function sliceAccounts<T>(accounts: T[], limit: number): { shown: T[]; more: number } {
  return { shown: accounts.slice(0, limit), more: Math.max(0, accounts.length - limit) };
}
