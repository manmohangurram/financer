export interface Extraction {
  amount?: number;
  direction?: string;
  occurredAt?: string | null;
  merchant?: string;
  accountHint?: string | null;
  confidence?: number;
}

export interface MailMessage {
  id: string;
  messageId: string;
  mailAccountId: string;
  fromAddr: string;
  fromName: string;
  subject: string;
  receivedAt?: string | null;
  status: string;
  parsedJson?: string | null;
  confidence: number;
  bodyPreview?: string;
  transactionId?: string | null;
}

export interface MailMessageDetail extends MailMessage {
  bodyText?: string | null;
  rawText?: string | null;
}

export function parseExtraction(raw?: string | null): Extraction | null {
  if (!raw) return null;
  try {
    return JSON.parse(raw) as Extraction;
  } catch {
    return null;
  }
}
