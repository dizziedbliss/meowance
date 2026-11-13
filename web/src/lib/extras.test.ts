import { describe, expect, it } from "vitest";
import { dayMonth, istHour, portalAsleep, shortDate } from "./dates";
import { scoreVerdict } from "./scores";
import { classTag, courseName, niceName, room } from "./week";
import type { CIEPart, Class, Course } from "./types";

const part = (name: string, max: number, obtained: number, absent = false, pending = false): CIEPart => ({
  name, max, obtained, absent, pending, entered: "13th Oct 26",
});
const course = (o: Partial<Course>): Course => ({
  code: "X", name: "X", credits: 3, attended: 20, conducted: 25, percent: 80, need75: 0, need85: 9, skip85: 0, ...o,
});

describe("scoreVerdict", () => {
  it("matches the fixture's cloud computing: ghosted SE1, 9/10 in SE2", () => {
    const v = scoreVerdict(course({ cie: [part("SE1", 10, 0, true), part("SE2", 10, 9)], cieTotal: 9, cieMax: 50 }));
    expect([v.got, v.lost, v.left]).toEqual([9, 11, 30]);
    expect(v.zone).toBe("dead"); // 9 of 20 = 45%
    expect(v.sub).toContain("ghosted cie1");
  });
  it("all tests ghosted", () => {
    const v = scoreVerdict(course({ cie: [part("SE1", 10, 0, true)], cieTotal: 0, cieMax: 50 }));
    expect(v.head).toBe("ghosted the only test");
    expect(v.left).toBe(40);
  });
  it("nothing marked yet, including old cached data with no cie field", () => {
    expect(scoreVerdict(course({})).head).toBe("no marks yet");
    expect(scoreVerdict(course({ cie: [part("SE1", 10, 0, false, true)] })).zone).toBeNull();
  });
  it("cooking", () => {
    expect(scoreVerdict(course({ cie: [part("SE1", 10, 9), part("SE2", 10, 8)], cieTotal: 17 })).head).toBe("cooking fr");
  });
  it("short dates", () => expect(shortDate("13th Oct 26")).toBe("13 oct"));
});

describe("timetable helpers", () => {
  const cls = (code: string): Class => ({ start: "09:30", end: "10:30", code, name: "AI FOR PREDICTIVE ANALYTICS AND IOT", room: "LABORATARY", batch: "BATCH 1" });
  const courses = [
    course({ code: "DEAD", percent: 71.4 }),
    course({ code: "FINE", percent: 80 }),
    course({ code: "SAFE", percent: 100, skip85: 2 }),
    course({ code: "ZERO", percent: 100, skip85: 0 }),
  ];
  it("tags each class by that subject's attendance", () => {
    expect(classTag(cls("DEAD"), courses)?.tone).toBe("dead");
    expect(classTag(cls("FINE"), courses)?.text).toBe("show up");
    expect(classTag(cls("SAFE"), courses)?.text).toBe("bunkable · 2 lives");
    expect(classTag(cls("ZERO"), courses)?.tone).toBe("mut");
    expect(classTag(cls("UNKNOWN"), courses)).toBeNull();
  });
  it("tidies names, rooms and dates", () => {
    expect(niceName("AI FOR PREDICTIVE ANALYTICS AND IOT")).toBe("AI for Predictive Analytics and IoT");
    expect(courseName(cls("UNKNOWN"), courses)).toBe("AI for Predictive Analytics and IoT");
    expect(room("LABORATARY")).toBe("lab");
    expect(room("CLASS ROOM")).toBe("classroom");
    expect(dayMonth("02-11-2026")).toBe("2 nov");
    expect(dayMonth("08/11/2026")).toBe("8 nov");
  });
});

describe("portal sleep hours (IST)", () => {
  const at = (iso: string) => new Date(iso);
  it("reads the hour in india, not the viewer's zone", () => {
    expect(istHour(at("2026-11-12T18:29:00Z"))).toBe(23); // 11:59pm IST
    expect(istHour(at("2026-11-12T18:30:00Z"))).toBe(0); // midnight IST, next day
  });
  it("is asleep from midnight to 6am IST only", () => {
    expect(portalAsleep(at("2026-11-12T18:29:00Z"))).toBe(false); // 11:59pm
    expect(portalAsleep(at("2026-11-12T18:30:00Z"))).toBe(true); // 12:00am
    expect(portalAsleep(at("2026-11-13T00:29:00Z"))).toBe(true); // 5:59am
    expect(portalAsleep(at("2026-11-13T00:30:00Z"))).toBe(false); // 6:00am
  });
});
