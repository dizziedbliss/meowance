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

const IST_MS = 5.5 * 3_600_000; // IST is UTC+5:30, no DST
const ist = (now: Date) => new Date(now.getTime() + IST_MS); // read with getUTC*

/** Today in India (the portal's dates are IST), as YYYY-MM-DD, wherever the viewer is. */
export function todayISO(now = new Date()): string {
  return ist(now).toISOString().slice(0, 10);
}

/** The Monday of that YYYY-MM-DD's week (mon–sun), as YYYY-MM-DD. */
export function mondayOf(iso: string): string {
  const d = new Date(`${iso}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() - ((d.getUTCDay() + 6) % 7));
  return d.toISOString().slice(0, 10);
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
  return ist(now).getUTCHours();
}

/** Today's weekday in India, "Monday". */
export function todayName(now = new Date()): string {
  return ist(now).toLocaleDateString("en-US", { weekday: "long", timeZone: "UTC" });
}

/** The portal goes down most nights, roughly midnight to 6am IST. */
export const portalAsleep = (now = new Date()) => istHour(now) < 6;
