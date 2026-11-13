import { describe, expect, it } from "vitest";
import { semTotal, squares } from "./history";
import { partLabel, seeMin } from "./scores";
import type { ClassRecord } from "./types";

const cls = (date: string, present = true, start = "11:00"): ClassRecord => ({ date, start, end: "12:00", present });

describe("semTotal", () => {
  const hist = (present: number, absent: number, stillToGo: number) =>
    ({ code: "X", name: "X", faculty: "", present, absent, stillToGo, classes: [] });
  it("is the portal's plan for the sem: present + absent + still to go", () => {
    expect(semTotal(hist(20, 5, 20))).toBe(45);
    expect(semTotal(hist(0, 0, 0))).toBeNull();
    expect(semTotal(undefined)).toBeNull();
  });
});

describe("squares", () => {
  it("uses the real order and dates when the history matches the dashboard's counts", () => {
    const h = [cls("2026-08-11"), cls("2026-08-18", false), cls("2026-08-25")];
    expect(squares(2, 3, h)).toEqual(h);
  });
  it("falls back to undated boxes when the two pages disagree", () => {
    expect(squares(3, 4, [cls("2026-08-11")])).toBeNull();
    expect(squares(2, 3, undefined)).toBeNull();
  });
});

describe("partLabel", () => {
  it("calls the dashboard's SE tests what they are: cie", () => {
    expect(partLabel("SE1")).toBe("cie1");
    expect(partLabel("se2")).toBe("cie2");
    expect(partLabel("Activity1")).toBe("activity1");
  });
});

describe("seeMin (seniors: 12 of 30 from 3 cies, 8 of 20 from 2 activities)", () => {
  const part = (name: string, obtained: number, absent = false, pending = false) => ({ name, max: 10, obtained, absent, pending, entered: "" });
  const course = (cie: ReturnType<typeof part>[]) =>
    ({ code: "X", name: "X", credits: 3, attended: 1, conducted: 1, percent: 100, need75: 0, need85: 0, skip85: 0, cie });
  it("counts what's left to reach 12 from the cies still to come", () => {
    const m = seeMin(course([part("SE1", 0, true), part("SE2", 9)]), "4MC23CI000")!;
    expect(m.cie).toEqual({ got: 9, need: 12, left: 1, status: "open" });
    expect(m.act).toEqual({ got: 0, need: 8, left: 2, status: "open" });
  });
  it("marks a part cleared, or out of reach", () => {
    expect(seeMin(course([part("SE1", 6), part("SE2", 7)]), "4MC23CI000")!.cie.status).toBe("done");
    expect(seeMin(course([part("SE1", 0, true), part("SE2", 0, true), part("SE3", 1)]), "4MC23CI000")!.cie.status).toBe("gone");
  });
  it("counts non-cie tests (lab, activity) as activities, and skips unmarked ones", () => {
    const m = seeMin(course([part("SE1", 5), part("Lab1", 7), part("SE2", 0, false, true)]), "4MC23CI000")!;
    expect(m.cie).toEqual({ got: 5, need: 12, left: 2, status: "open" });
    expect(m.act).toEqual({ got: 7, need: 8, left: 1, status: "open" });
  });
  it("says nothing for juniors: their scheme changed and isn't known yet", () => {
    expect(seeMin(course([part("SE1", 5)]), "4MC24CI000")).toBeNull();
  });
});
