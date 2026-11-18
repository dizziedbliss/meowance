import type { Attendance, Course, CourseHistory } from "./types";

// The browser keeps the slow-changing pages so most logins only fetch the
// dashboard: fewer proxy bytes, fewer portal hits, faster syncs. Each cached
// page goes stale when what it reflects changes, never just because time passed
// (except the calendar and exam history, which the dashboard can't tell us about).

/** Pages fetched only when a screen needs them. Per-course ones carry the course code. */
export type Extra = "timetable" | "calendar" | "exams" | `history:${string}`;

/**
 * What's in localStorage. got = when each extra was last fetched successfully;
 * basis = what it reflected then (a course's counts, the semester); failed = when
 * it last came back empty, so a page the portal keeps failing isn't refetched (a
 * whole portal login) on every screen visit.
 */
export type Stored = {
  at: number;
  a: Attendance;
  got: Partial<Record<Extra, number>>;
  basis?: Partial<Record<Extra, string>>;
  failed?: Partial<Record<Extra, number>>;
};

const DAY = 86_400_000;
const CALENDAR_DAYS = 7; // the calendar barely changes; a week is fresh enough
const EXAM_DAYS = 30; // results land once a semester
const RETRY_AFTER = 10 * 60_000; // a page that just failed waits this long before it's auto-fetched again (sync still forces it)

/** Midnight on the Monday of t's week, local time. */
function weekStart(t: number): number {
  const d = new Date(t);
  d.setHours(0, 0, 0, 0);
  d.setDate(d.getDate() - ((d.getDay() + 6) % 7));
  return d.getTime();
}

const courseOf = (a: Attendance, code: string): Course | undefined => a.courses.find((c) => c.code === code);
/** "history:23AI701" → "23AI701". */
const codeOf = (e: string) => e.slice("history:".length);

/** What a page reflects in this dashboard; it's refetched when that changes. */
function basisOf(a: Attendance, e: Extra): string {
  if (e === "exams" || e === "calendar") return a.student.sem; // each semester has its own calendar
  if (e === "timetable") return `${a.student.sem}|${a.student.section}`; // a new sem or section is a new timetable
  const c = courseOf(a, codeOf(e));
  return c ? `${c.attended}/${c.conducted}` : "";
}

function get(a: Attendance, e: Extra): unknown {
  if (e === "timetable" || e === "calendar" || e === "exams") return a[e];
  return a.history?.[codeOf(e)];
}

function set(a: Attendance, e: Extra, v: unknown): void {
  if (e === "timetable" || e === "calendar" || e === "exams") {
    (a as Record<string, unknown>)[e] = v;
    return;
  }
  a.history = { ...a.history, [codeOf(e)]: v as CourseHistory | null };
}

/** Whether this extra must be fetched (never fetched, failed a while ago, stale, or what it reflects changed). */
export function needs(s: Stored | null, e: Extra, now: number): boolean {
  if (!s) return true;
  if (e.startsWith("history:") && !courseOf(s.a, codeOf(e))) return false; // not a course this sem: nothing to fetch
  const failed = s.failed?.[e];
  if (failed !== undefined && now - failed < RETRY_AFTER) return false;
  const got = s.got[e];
  if (got === undefined || get(s.a, e) == null) return true;
  if (e === "timetable" && weekStart(got) !== weekStart(now)) return true;
  if (e === "calendar" && now - got >= CALENDAR_DAYS * DAY) return true;
  if (e === "exams" && now - got >= EXAM_DAYS * DAY) return true;
  return s.basis?.[e] !== basisOf(s.a, e);
}

/** The extras in this list that need fetching. */
export const stale = (s: Stored | null, list: Extra[], now: number): Extra[] => list.filter((e) => needs(s, e, now));

/**
 * The extras as sent to the server. A course whose class list is cached goes as
 * "history:<code>@<attended>/<conducted>" (the counts it reflects): the server
 * skips that page when the fresh dashboard shows the same counts, so a sync only
 * fetches the courses that changed.
 */
export function wire(s: Stored | null, list: Extra[]): string[] {
  return list.map((e) => {
    const basis = s?.basis?.[e];
    return e.startsWith("history:") && basis && get(s!.a, e) != null ? `${e}@${basis}` : e;
  });
}

/** Folds a fresh response into the cache: asked extras replace the cached ones; everything else is kept. */
export function merge(prev: Stored | null, next: Attendance, asked: Extra[], now: number): Stored {
  const same = prev?.a.student.usn === next.student.usn ? prev : null;
  const a: Attendance = { student: next.student, courses: next.courses };
  const out: Stored = { at: now, a, got: { ...same?.got }, basis: { ...same?.basis }, failed: { ...same?.failed } };
  if (same) {
    for (const f of ["timetable", "calendar", "exams"] as const) if (same.a[f] !== undefined) set(a, f, same.a[f]);
    // only this sem's courses: last sem's class lists would pile up in localStorage forever
    const codes = new Set(next.courses.map((c) => c.code));
    const keepCode = (e: string) => !e.startsWith("history:") || codes.has(codeOf(e));
    if (same.a.history) a.history = Object.fromEntries(Object.entries(same.a.history).filter(([code]) => codes.has(code)));
    for (const m of [out.got, out.basis!, out.failed!]) for (const k of Object.keys(m)) if (!keepCode(k)) delete m[k as Extra];
  }
  for (const e of asked) {
    const v = get(next, e);
    if (v != null) {
      set(a, e, v);
      out.got[e] = now;
      out.basis![e] = basisOf(next, e);
      delete out.failed![e];
    } else if (v === undefined && e.startsWith("history:") && get(a, e) != null) {
      // sent with its counts and skipped by the server: unchanged, the cached copy stands
    } else {
      out.failed![e] = now;
      if (get(a, e) == null) set(a, e, null); // asked, failed, nothing cached: the screen says so
      // asked and failed with an old copy: keep it on screen, and its age, so it's retried later
    }
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
