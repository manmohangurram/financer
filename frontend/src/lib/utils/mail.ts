import { formatCurrency } from '@/lib/utils/format';
import { parseExtraction, type MailMessage } from '@/lib/types/mail';

/** daisyUI badge tone for a queue status; neutral statuses stay unstyled. */
export function stampClass(status: string): string {
  if (status === 'AUTO') return 'badge-success';
  if (status === 'UNPARSED') return 'badge-error';
  if (status === 'PENDING') return 'badge-warning';
  return '';
}

/** Signed amount for a queue row, empty when the extraction has none. */
export function amountLabel(m: MailMessage): string {
  const p = parseExtraction(m.parsedJson);
  if (!p?.amount) return '';
  const sign = p.direction === 'CREDIT' ? '+' : '−';
  return `${sign} ${formatCurrency(p.amount)}`;
}
