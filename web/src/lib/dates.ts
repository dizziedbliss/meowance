// The portal prints dates three different ways; these turn them all into "8 nov".
const months = ["jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"];

/** "08/11/2026" (last updated) or "02-11-2026" (timetable) → "8 nov". */
export function dayMonth(s: string): string {
  const m = /^(\d{1,2})[/-](\d{1,2})[/-]\d{4}$/.exec(s.trim());
  return m ? `${+m[1]} ${months[+m[2] - 1] ?? ""}`.trim() : s;
}

/** "2026-11-12" (calendar) → "12 nov". */
export function isoDayMonth(s: string): string {
  const m = /^\d{4}-(\d{2})-(\d{2})$/.exec(s);
  return m ? `${+m[2]} ${months[+m[1] - 1] ?? ""}`.trim() : s;
}

/** Today in the viewer's timezone, as YYYY-MM-DD. */
export function todayISO(now = new Date()): string {
  const p = (n: number) => String(n).padStart(2, "0");
  return `${now.getFullYear()}-${p(now.getMonth() + 1)}-${p(now.getDate())}`;
}

/** Whole days from a to b (YYYY-MM-DD), b later = positive. */
export function daysBetween(a: string, b: string): number {
  return Math.round((Date.parse(`${b}T00:00:00Z`) - Date.parse(`${a}T00:00:00Z`)) / 86_400_000);
}

/** "13th Oct 26" (CIE marks entered) → "13 oct". */
export function shortDate(s: string): string {
  const m = /^(\d{1,2})\w*\s+([A-Za-z]{3})/.exec(s.trim());
  return m ? `${+m[1]} ${m[2].toLowerCase()}` : s.toLowerCase();
}

/** The hour (0–23) in India, wherever the viewer is. */
export function istHour(now = new Date()): number {
  return new Date(now.getTime() + 5.5 * 3_600_000).getUTCHours(); // IST is UTC+5:30, no DST
}

/** The portal goes down most nights, roughly midnight to 6am IST. */
export const portalAsleep = (now = new Date()) => istHour(now) < 6;
