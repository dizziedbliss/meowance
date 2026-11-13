import { describe, expect, it } from "vitest";
import { merge, needs, revive, stale, type Stored } from "./cache";
import type { Attendance, Calendar, Day } from "./types";

const day = (date: string): Day => ({ weekday: "Monday", date, classes: [] });
const cal: Calendar = { start: "2026-08-03", end: "2026-11-20", holidays: [], examDays: [], events: [] };
const att = (o: Partial<Attendance> = {}): Attendance => ({
  student: { usn: "4MC00AA000", name: "STUDENT", dept: "AA", sem: "7", section: "A", lastUpdated: "08/11/2026" },
  courses: [],
  ...o,
});
// Local-time dates: the week rule follows the viewer's calendar, like the UI does.
const at = (y: number, m: number, d: number, h = 12) => new Date(y, m - 1, d, h).getTime();
const FRI = at(2026, 11, 13);
const stored = (o: Partial<Stored> = {}): Stored => ({ at: FRI, a: att(), got: {}, ...o });

describe("needs", () => {
  it("fetches what was never fetched, or failed last time", () => {
    expect(needs(null, "timetable", FRI)).toBe(true);
    expect(needs(stored(), "calendar", FRI)).toBe(true);
    expect(needs(stored({ a: att({ calendar: null }), got: {} }), "calendar", FRI)).toBe(true);
  });
  it("keeps the timetable for the rest of its week (mon–sun)", () => {
    const s = stored({ a: att({ timetable: [day("09-11-2026")] }), got: { timetable: at(2026, 11, 9, 8) } }); // monday
    expect(needs(s, "timetable", at(2026, 11, 15, 23))).toBe(false); // sunday night, same week
    expect(needs(s, "timetable", at(2026, 11, 16, 0))).toBe(true); // next monday
  });
  it("keeps the calendar for 7 days", () => {
    const s = stored({ a: att({ calendar: cal }), got: { calendar: FRI } });
    expect(needs(s, "calendar", FRI + 6 * 86_400_000)).toBe(false);
    expect(needs(s, "calendar", FRI + 7 * 86_400_000)).toBe(true);
  });
});

describe("merge", () => {
  it("keeps cached extras when only the dashboard came back", () => {
    const prev = stored({ a: att({ timetable: [day("09-11-2026")], calendar: cal }), got: { timetable: 1, calendar: 2 } });
    const next = merge(prev, att({ courses: [{ code: "NEW" } as never] }), [], FRI);
    expect(next.a.courses).toHaveLength(1);
    expect(next.a.timetable).toEqual([day("09-11-2026")]);
    expect(next.a.calendar).toBe(cal);
    expect(next.got).toEqual({ timetable: 1, calendar: 2 });
    expect(next.at).toBe(FRI);
  });
  it("takes asked extras, and remembers a failure as not fetched", () => {
    const prev = stored({ a: att({ calendar: cal }), got: { calendar: 2 } });
    const next = merge(prev, att({ timetable: [], calendar: null }), ["timetable", "calendar"], FRI);
    expect(next.a.timetable).toEqual([]); // an empty week is a real answer
    expect(next.got.timetable).toBe(FRI);
    expect(next.a.calendar).toBe(cal); // a failed refresh keeps the old copy on screen…
    expect(next.got.calendar).toBe(2); // …and its age, so it's retried next time
  });
  it("never mixes two students", () => {
    const prev = stored({ a: att({ calendar: cal }), got: { calendar: 2 } });
    const other = att({ student: { ...prev.a.student, usn: "4MC00AA001" } });
    const next = merge(prev, other, [], FRI);
    expect(next.a.calendar).toBeUndefined();
    expect(next.got).toEqual({});
  });
});

describe("revive", () => {
  it("reads the old {at, a} format: extras count as fetched at `at`", () => {
    const old = { at: 5, a: att({ timetable: [day("09-11-2026")], calendar: null }) };
    expect(revive(old)?.got).toEqual({ timetable: 5 });
  });
  it("rejects junk", () => {
    expect(revive(null)).toBeNull();
    expect(revive({ at: 1 })).toBeNull();
  });
});

// --- per-course pages and exam history ---
const course = (code: string, attended: number, conducted: number, cieTotal = 9) =>
  ({ code, name: code, credits: 3, attended, conducted, percent: 0, need75: 0, need85: 0, skip85: 0, cie: [], cieTotal, cieMax: 50 });
const hist = (code: string) => ({ code, name: code, faculty: "FACULTY", present: 20, absent: 5, stillToGo: 20, classes: [] });
const exams = { creditsEarned: 124, creditsToEarn: 36, cgpa: 7.9, sems: [] };

describe("per-course and exam pages", () => {
  const base = att({ courses: [course("23AI701", 20, 25), course("23AI702", 10, 12)] });

  it("fetches a course's history once, and again only when its counts change", () => {
    let s = merge(null, base, [], FRI);
    expect(needs(s, "history:23AI701", FRI)).toBe(true);
    s = merge(s, { ...base, history: { "23AI701": hist("23AI701") } }, ["history:23AI701"], FRI);
    expect(needs(s, "history:23AI701", FRI + 60 * 86_400_000)).toBe(false); // old, but nothing changed
    expect(needs(s, "history:23AI702", FRI)).toBe(true); // a different course
    const counted = merge(s, att({ courses: [course("23AI701", 21, 26), course("23AI702", 10, 12)] }), [], FRI);
    expect(counted.a.history?.["23AI701"]).toBeTruthy(); // still shown…
    expect(needs(counted, "history:23AI701", FRI)).toBe(true); // …but refetched: a class was taken
  });
  it("refetches a course's cie averages when its marks change", () => {
    let s = merge(null, base, [], FRI);
    s = merge(s, { ...base, cieDetails: { "23AI701": { code: "23AI701", stats: [] } } }, ["cie:23AI701"], FRI);
    expect(needs(s, "cie:23AI701", FRI)).toBe(false);
    const marked = merge(s, att({ courses: [course("23AI701", 20, 25, 18), course("23AI702", 10, 12)] }), [], FRI);
    expect(needs(marked, "cie:23AI701", FRI)).toBe(true);
  });
  it("keeps one course's page when another's comes back, and a failed one stays stale", () => {
    let s = merge(null, { ...base, history: { "23AI701": hist("23AI701") } }, ["history:23AI701"], FRI);
    s = merge(s, { ...base, history: { "23AI702": null } }, ["history:23AI702"], FRI);
    expect(s.a.history?.["23AI701"]).toBeTruthy();
    expect(s.a.history?.["23AI702"]).toBeNull();
    expect(needs(s, "history:23AI702", FRI)).toBe(true);
  });
  it("keeps exam history for 30 days, or until the semester changes", () => {
    const s = merge(null, { ...base, exams }, ["exams"], FRI);
    expect(needs(s, "exams", FRI + 29 * 86_400_000)).toBe(false);
    expect(needs(s, "exams", FRI + 30 * 86_400_000)).toBe(true);
    const nextSem = merge(s, att({ student: { ...base.student, sem: "8" } }), [], FRI);
    expect(nextSem.a.exams).toBe(exams);
    expect(needs(nextSem, "exams", FRI)).toBe(true);
  });
  it("lists every stale page for a set of courses in one go", () => {
    const s = merge(null, { ...base, history: { "23AI701": hist("23AI701") } }, ["history:23AI701"], FRI);
    expect(stale(s, ["history:23AI701", "history:23AI702", "exams"], FRI)).toEqual(["history:23AI702", "exams"]);
  });
});
