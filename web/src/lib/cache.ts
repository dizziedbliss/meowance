import type { Attendance, CIEDetails, Course, CourseHistory } from "./types";

// The browser keeps the slow-changing pages so most logins only fetch the
// dashboard: fewer proxy bytes, fewer portal hits, faster syncs. Each cached
// page goes stale when what it reflects changes, never just because time passed
// (except the calendar and exam history, which the dashboard can't tell us about).

/** Pages fetched only when a screen needs them. Per-course ones carry the course code. */
export type Extra = "timetable" | "calendar" | "exams" | `history:${string}` | `cie:${string}`;

/**
 * What's in localStorage. got = when each extra was last fetched successfully;
 * basis = what it reflected then (a course's counts or marks, the semester).
 */
export type Stored = { at: number; a: Attendance; got: Partial<Record<Extra, number>>; basis?: Partial<Record<Extra, string>> };

const DAY = 86_400_000;
const CALENDAR_DAYS = 7; // the calendar barely changes; a week is fresh enough
const EXAM_DAYS = 30; // results land once a semester

/** Midnight on the Monday of t's week, local time. */
function weekStart(t: number): number {
  const d = new Date(t);
  d.setHours(0, 0, 0, 0);
  d.setDate(d.getDate() - ((d.getDay() + 6) % 7));
  return d.getTime();
}

const courseOf = (a: Attendance, code: string): Course | undefined => a.courses.find((c) => c.code === code);

/** What a page reflects in this dashboard; it's refetched when that changes. */
function basisOf(a: Attendance, e: Extra): string {
  if (e === "exams") return a.student.sem;
  const [kind, code] = e.split(":");
  const c = courseOf(a, code);
  if (!c) return "";
  if (kind === "history") return `${c.attended}/${c.conducted}`;
  if (kind === "cie") return JSON.stringify([c.cieTotal, c.cie?.map((p) => [p.name, p.obtained, p.absent, p.pending])]);
  return "";
}

function get(a: Attendance, e: Extra): unknown {
  if (e === "timetable" || e === "calendar" || e === "exams") return a[e];
  const [kind, code] = e.split(":");
  return (kind === "history" ? a.history : a.cieDetails)?.[code];
}

function set(a: Attendance, e: Extra, v: unknown): void {
  if (e === "timetable" || e === "calendar" || e === "exams") {
    (a as Record<string, unknown>)[e] = v;
    return;
  }
  const [kind, code] = e.split(":");
  if (kind === "history") a.history = { ...a.history, [code]: v as CourseHistory | null };
  else a.cieDetails = { ...a.cieDetails, [code]: v as CIEDetails | null };
}

/** Whether this extra must be fetched (never fetched, failed last time, or what it reflects changed). */
export function needs(s: Stored | null, e: Extra, now: number): boolean {
  if (!s) return true;
  if (e.includes(":") && !courseOf(s.a, e.split(":")[1])) return false; // not a course this sem: nothing to fetch
  const got = s.got[e];
  if (got === undefined || get(s.a, e) == null) return true;
  if (e === "timetable") return weekStart(got) !== weekStart(now);
  if (e === "calendar") return now - got >= CALENDAR_DAYS * DAY;
  if (e === "exams" && now - got >= EXAM_DAYS * DAY) return true;
  return s.basis?.[e] !== basisOf(s.a, e);
}

/** The extras in this list that need fetching. */
export const stale = (s: Stored | null, list: Extra[], now: number): Extra[] => list.filter((e) => needs(s, e, now));

/** Folds a fresh response into the cache: asked extras replace the cached ones; everything else is kept. */
export function merge(prev: Stored | null, next: Attendance, asked: Extra[], now: number): Stored {
  const same = prev?.a.student.usn === next.student.usn ? prev : null;
  const a: Attendance = { student: next.student, courses: next.courses };
  const out: Stored = { at: now, a, got: { ...same?.got }, basis: { ...same?.basis } };
  if (same) {
    for (const f of ["timetable", "calendar", "exams"] as const) if (same.a[f] !== undefined) set(a, f, same.a[f]);
    if (same.a.history) a.history = { ...same.a.history };
    if (same.a.cieDetails) a.cieDetails = { ...same.a.cieDetails };
  }
  for (const e of asked) {
    const v = get(next, e);
    if (v != null) {
      set(a, e, v);
      out.got[e] = now;
      out.basis![e] = basisOf(next, e);
    } else if (get(a, e) == null) {
      set(a, e, null); // asked, failed, nothing cached: the screen says so
    } // asked and failed with an old copy: keep it on screen, and its age, so it's retried
  }
  return out;
}

/** Reads what was stored, including the old {at, a} shape from before extras were lazy. */
export function revive(raw: unknown): Stored | null {
  const r = raw as Partial<Stored> | null;
  if (!r || typeof r.at !== "number" || !r.a?.student) return null;
  if (r.got) return { ...(r as Stored), basis: r.basis ?? {} };
  const got: Stored["got"] = {};
  if (r.a.timetable != null) got.timetable = r.at;
  if (r.a.calendar != null) got.calendar = r.at;
  return { at: r.at, a: r.a, got, basis: {} };
}
