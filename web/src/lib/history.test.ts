import { describe, expect, it } from "vitest";
import { byMonth, endOfSem, lastRun } from "./history";
import { classAvg } from "./scores";
import type { CIEDetails, ClassRecord } from "./types";

const cls = (date: string, present = true, start = "11:00"): ClassRecord => ({ date, start, end: "12:00", present });

describe("byMonth", () => {
  it("groups classes by month, newest month first, oldest class first inside", () => {
    const m = byMonth([cls("2026-09-01"), cls("2026-09-08", false), cls("2026-10-06", false), cls("2026-10-07")]);
    expect(m.map((x) => x.label)).toEqual(["oct", "sep"]);
    expect(m[1].classes.map((c) => c.date)).toEqual(["2026-09-01", "2026-09-08"]);
  });
});

describe("lastRun", () => {
  it("counts the run at the end: pawsent or bunked in a row", () => {
    expect(lastRun([cls("2026-09-01", false), cls("2026-09-02"), cls("2026-09-03")])).toEqual({ present: true, n: 2 });
    expect(lastRun([cls("2026-09-01"), cls("2026-09-02", false)])).toEqual({ present: false, n: 1 });
    expect(lastRun([])).toBeNull();
  });
});

describe("endOfSem", () => {
  // 20 of 25 so far (80%), 20 still to go
  it("says how many of the classes left reach 85", () => {
    expect(endOfSem(20, 25, 20)).toEqual({ zone: "fine", line: "20 to go. attend 19 of them and you end on 85." });
  });
  it("falls back to 75 when 85 is out of reach", () => {
    // 15 of 25, 20 to go: 85 needs 24 (> 20), 75 needs 19
    expect(endOfSem(15, 25, 20)).toEqual({ zone: "fine", line: "85's out of reach this sem. attend 19 of the 20 left to end on 75." });
  });
  it("says cooked when even every class left misses 75", () => {
    expect(endOfSem(10, 25, 10)).toEqual({ zone: "dead", line: "cooked. even all 10 left won't get you to 75." });
  });
  it("is safe when even bunking every class left keeps 85", () => {
    expect(endOfSem(40, 40, 5)).toEqual({ zone: "safe", line: "safe even if you bunk all 5 left. you won't, but you could." });
  });
  it("says nothing when no classes are left", () => {
    expect(endOfSem(20, 25, 0)).toBeNull();
  });
});

describe("classAvg", () => {
  const d: CIEDetails = { code: "23AI701", stats: [
    { name: "CIE1", obtained: 0, classAvg: 7, max: 10 },
    { name: "CIE2", obtained: 9, classAvg: 8, max: 10 },
    { name: "Final IA", obtained: 9, classAvg: 15, max: 50 },
  ] };
  it("matches the dashboard's SE1 to the cie page's CIE1 by number", () => {
    expect(classAvg("SE2", d)).toBe(8);
    expect(classAvg("SE3", d)).toBeNull();
    expect(classAvg("SE1", null)).toBeNull();
  });
  it("reads the total's average from Final IA", () => {
    expect(classAvg("total", d)).toBe(15);
  });
});
