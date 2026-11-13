import { describe, expect, it } from "vitest";
import { merge, needs, revive, type Stored } from "./cache";
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
