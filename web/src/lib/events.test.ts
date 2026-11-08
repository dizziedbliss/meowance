import { describe, expect, it } from "vitest";
import { daysBetween, isoDayMonth, todayISO } from "./dates";
import { countdown, eventTag, rangeLabel, splitEvents, workingDaysLeft } from "./events";
import type { Calendar, CalendarEvent } from "./types";

const ev = (from: string, to: string, title: string, kind: CalendarEvent["kind"] = ""): CalendarEvent => ({ from, to, title, kind });

// The tail of the real (fixture) calendar: sem ends Fri 20 Nov 2026.
const cal: Calendar = {
  start: "2026-08-03",
  end: "2026-11-20",
  holidays: ["2026-11-08", "2026-11-10", "2026-11-15"], // two Sundays + Balipadyami
  examDays: ["2026-11-12", "2026-11-13", "2026-11-14"],
  events: [
    ev("2026-11-05", "2026-11-07", "BOE Meeting by Departments"),
    ev("2026-11-10", "2026-11-10", "Balipadyami", "holiday"),
    ev("2026-11-12", "2026-11-14", "Minor Exam 3", "exam"),
    ev("2026-11-20", "2026-11-20", "LAST WORKING DAY", "lastday"),
  ],
};

describe("events", () => {
  it("counts working days left: no Sundays, no holidays", () => {
    // 9 nov (mon) .. 20 nov (fri) = 12 days, minus sun 15 and holiday tue 10 = 10
    expect(workingDaysLeft(cal, "2026-11-09")).toBe(10);
    expect(workingDaysLeft(cal, "2026-11-21")).toBe(0);
  });
  it("splits upcoming and past around today, soonest first", () => {
    const { upcoming, past } = splitEvents(cal, "2026-11-09");
    expect(upcoming.map((e) => e.title)).toEqual(["Balipadyami", "Minor Exam 3", "LAST WORKING DAY"]);
    expect(past.map((e) => e.title)).toEqual(["BOE Meeting by Departments"]);
  });
  it("countdowns and ranges", () => {
    expect(countdown(cal.events[1], "2026-11-09")).toBe("tomorrow");
    expect(countdown(cal.events[2], "2026-11-09")).toBe("in 3 days");
    expect(countdown(cal.events[2], "2026-11-13")).toBe("happening now");
    expect(rangeLabel(cal.events[2])).toBe("12–14 nov");
    expect(rangeLabel(ev("2026-10-30", "2026-11-02", "x"))).toBe("30 oct – 2 nov");
  });
  it("tags by kind", () => {
    expect(eventTag(cal.events[1])?.text).toBe("free real estate");
    expect(eventTag(cal.events[2])?.text).toBe("lock in");
    expect(eventTag(cal.events[3])?.text).toBe("last day");
    expect(eventTag(cal.events[0])).toBeNull();
  });
  it("date helpers", () => {
    expect(isoDayMonth("2026-08-03")).toBe("3 aug");
    expect(daysBetween("2026-11-09", "2026-11-20")).toBe(11);
    expect(todayISO(new Date(2026, 10, 8))).toBe("2026-11-08");
  });
});
