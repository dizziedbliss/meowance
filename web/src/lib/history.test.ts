import { describe, expect, it } from "vitest";
import { bunksLeft, squares } from "./history";
import { classAvg, partLabel } from "./scores";
import type { CIEDetails, ClassRecord } from "./types";

const cls = (date: string, present = true, start = "11:00"): ClassRecord => ({ date, start, end: "12:00", present });

describe("bunksLeft", () => {
  // the portal's counts can be stale, so it's phrased as "you can bunk", never "you must attend"
  it("counts how many of the classes left you can bunk and still end on 85", () => {
    expect(bunksLeft(20, 25, 20)).toEqual({ zone: "fine", line: "you can bunk 1 of the 20 left before the sem ends." });
    expect(bunksLeft(40, 42, 20)).toEqual({ zone: "safe", line: "you can bunk 7 of the 20 left before the sem ends." });
  });
  it("says so when every class left is bunkable", () => {
    expect(bunksLeft(40, 40, 5)).toEqual({ zone: "safe", line: "you could bunk all 5 left and still end on 85." });
  });
  it("says no bunks when one more drops you under 85", () => {
    expect(bunksLeft(19, 25, 20)).toEqual({ zone: "fine", line: "no bunks left. miss one more and you end under 85." });
  });
  it("falls back to 75 when 85 is out of reach", () => {
    // 15 of 25, 20 to go: ending on 75 allows 1 bunk (34 of 45)
    expect(bunksLeft(15, 25, 20)).toEqual({ zone: "dead", line: "85's gone this sem. you can bunk 1 of the 20 left and still end on 75." });
  });
  it("says cooked when even every class left misses 75", () => {
    expect(bunksLeft(10, 25, 10)).toEqual({ zone: "dead", line: "cooked. even all 10 left won't get you to 75." });
  });
  it("says nothing when no classes are left", () => {
    expect(bunksLeft(20, 25, 0)).toBeNull();
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
