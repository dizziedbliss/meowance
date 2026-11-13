import type { Attendance } from "./types";

// The browser keeps the slow-changing pages so most logins only fetch the
// dashboard: fewer proxy bytes, fewer portal hits, faster syncs.

/** Pages fetched only when their tab needs them. */
export type Extra = "timetable" | "calendar";

/** What's in localStorage. got = when each extra was last fetched successfully. */
export type Stored = { at: number; a: Attendance; got: Partial<Record<Extra, number>> };

const DAY = 86_400_000;
const CALENDAR_DAYS = 7; // the calendar barely changes; a week is fresh enough

/** Midnight on the Monday of t's week, local time. */
function weekStart(t: number): number {
  const d = new Date(t);
  d.setHours(0, 0, 0, 0);
  d.setDate(d.getDate() - ((d.getDay() + 6) % 7));
  return d.getTime();
}

/** Whether this extra must be fetched (never fetched, failed last time, or too old). */
export function needs(s: Stored | null, extra: Extra, now: number): boolean {
  const got = s?.got[extra];
  if (!s || got === undefined || s.a[extra] == null) return true;
  return extra === "timetable" ? weekStart(got) !== weekStart(now) : now - got >= CALENDAR_DAYS * DAY;
}

/** Folds a fresh response into the cache: asked extras replace the cached ones; the rest are kept. */
export function merge(prev: Stored | null, next: Attendance, asked: Extra[], now: number): Stored {
  const same = prev?.a.student.usn === next.student.usn ? prev : null;
  const out: Stored = { at: now, a: { ...next }, got: {} };
  for (const e of ["timetable", "calendar"] as const) {
    const fresh = asked.includes(e) ? next[e] : undefined;
    if (fresh != null) {
      out.a[e] = fresh as never;
      out.got[e] = now;
    } else if (same?.a[e] != null) {
      out.a[e] = same.a[e] as never; // not asked, or the refresh failed: keep the old copy and its age
      if (same.got[e] !== undefined) out.got[e] = same.got[e];
    } else if (asked.includes(e)) {
      out.a[e] = null; // asked, failed, nothing cached: the tab says so
    } else {
      delete out.a[e];
    }
  }
  return out;
}

/** Reads what was stored, including the old {at, a} shape from before extras were lazy. */
export function revive(raw: unknown): Stored | null {
  const r = raw as Partial<Stored> | null;
  if (!r || typeof r.at !== "number" || !r.a?.student) return null;
  if (r.got) return r as Stored;
  const got: Stored["got"] = {};
  if (r.a.timetable != null) got.timetable = r.at;
  if (r.a.calendar != null) got.calendar = r.at;
  return { at: r.at, a: r.a, got };
}
