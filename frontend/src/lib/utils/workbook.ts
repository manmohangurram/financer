import { parseCsvText } from './csv';
import { toLocalDateString } from './format';

export interface WorkbookRows {
  headers: string[];
  rows: string[][];
}

export function cellToText(cell: unknown): string {
  if (cell === null || cell === undefined) return '';
  if (cell instanceof Date) return toLocalDateString(cell);
  return String(cell).trim();
}

export function parseWorkbookText(text: string): WorkbookRows | null {
  return parseCsvText(text);
}

// readWorkbook reads a .csv or .xlsx file into { headers, rows }.
// xlsx goes through read-excel-file (dynamically imported so it stays out of
// the main bundle); the first sheet's rows are stringified.
export async function readWorkbook(file: File): Promise<WorkbookRows | null> {
  const name = file.name.toLowerCase();
  if (name.endsWith('.csv')) return parseWorkbookText(await file.text());
  if (name.endsWith('.xlsx')) {
    const readXlsxFile = (await import('read-excel-file/browser')).default;
    const raw = (await readXlsxFile(file)) as unknown[][];
    if (!raw || raw.length < 2) return null;
    return { headers: raw[0].map(cellToText), rows: raw.slice(1).map((r) => r.map(cellToText)) };
  }
  return null;
}
