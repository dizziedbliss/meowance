import { describe, expect, it } from "vitest";
import { byPercent, semVerdict, verdict, zoneOf } from "./verdict";
import type { Course } from "./types";

// Numbers as the Go API returns them for the scrubbed fixture.
const c = (code: string, attended: number, conducted: number, percent: number, need75: number, need85: number, skip85: number): Course => ({
  code, name: code, credits: 3, attended, conducted, percent, need75, need85, skip85,
});
const cloud = c("23AI701", 20, 25, 80, 0, 9, 0);
const ai = c("23AI704D", 10, 14, 71.4, 2, 13, 0);
const fullstack = c("23AI703", 25, 30, 83.3, 0, 4, 0);
const sensors = c("23OEEC71", 5, 5, 100, 0, 0, 0);

const text = (h: { before: string; n: number | null; after: string }) => `${h.before}${h.n ?? ""}${h.after}`;

describe("zoneOf", () => {
  it("uses MCE's 85 fine line and 75 dead line", () => {
    expect(zoneOf(85)).toBe("safe");
    expect(zoneOf(84.9)).toBe("fine");
    expect(zoneOf(75)).toBe("fine");
    expect(zoneOf(74.9)).toBe("dead");
  });
});

describe("verdict", () => {
  it("dead zone: attend to 75, then to 85", () => {
    const v = verdict(ai);
    expect(v.zone).toBe("dead");
    expect(text(v.head)).toBe("attend 2 more classes");
    expect(v.then && text(v.then.head)).toBe("13 more for 85 tho.");
  });
  it("fine zone counts classes, not days", () => {
    expect(text(verdict(cloud).head)).toBe("attend 9 more classes");
    expect(verdict(fullstack).sub).toMatch(/lock in/);
  });
  it("safe with no spare skips says 0 lives and the real drop", () => {
    const v = verdict(sensors);
    expect(text(v.head)).toBe("0 lives left");
    expect(v.sub).toContain("83.3%");
  });
  it("safe with spare skips counts lives", () => {
    expect(text(verdict(c("X", 30, 30, 100, 0, 0, 5)).head)).toBe("5 lives left");
    expect(text(verdict(c("X", 30, 31, 96.8, 0, 0, 1)).head)).toBe("1 life left");
  });
  it("singular class", () => {
    expect(text(verdict(c("X", 4, 5, 80, 0, 1, 0)).head)).toBe("attend 1 more class");
  });
});

describe("byPercent", () => {
  it("puts the worst subject first", () => {
    expect(byPercent([cloud, sensors, ai]).map((x) => x.code)).toEqual(["23AI704D", "23AI701", "23OEEC71"]);
  });
});

describe("semVerdict (from the sem's total classes, so stale portal counts don't cause panic)", () => {
  const c = (attended: number, conducted: number) =>
    ({ code: "X", name: "X", credits: 3, attended, conducted, percent: (attended / conducted) * 100, need75: 0, need85: 0, skip85: 0 });
  it("counts the bunks the whole sem still allows for 85", () => {
    // 45 classes: 85% lets you miss 6; 5 missed → 1 left
    const v = semVerdict(c(20, 25), 45);
    expect(v.head).toEqual({ before: "you can bunk ", n: 1, after: " more this sem" });
    expect(v.zone).toBe("fine");
    expect(v.sub).toBe("5 bunked so far. the sem has 45 classes, and 85 lets you miss 6.");
  });
  it("is calm early in the sem even when today's % looks bad", () => {
    const v = semVerdict(c(10, 14), 45); // 71% today, but only 4 missed of the 6 allowed
    expect(v.head.n).toBe(2);
    expect(v.zone).toBe("fine");
  });
  it("is safe when you're above 85 and still have bunks", () => {
    expect(semVerdict(c(24, 25), 45).zone).toBe("safe");
  });
  it("says zero when the budget is used up", () => {
    const v = semVerdict(c(19, 25), 45);
    expect(v.head).toEqual({ before: "", n: 0, after: " bunks left this sem" });
  });
  it("falls back to 75 once 85 is gone", () => {
    const v = semVerdict(c(16, 25), 45); // 9 missed: 85 allowed 6, 75 allows 11
    expect(v.head).toEqual({ before: "", n: 2, after: " bunks left before 75" });
    expect(v.zone).toBe("fine");
  });
  it("says cooked past the 75 budget", () => {
    const v = semVerdict(c(10, 25), 45); // 15 missed > 11
    expect(v.zone).toBe("dead");
    expect(v.head.n).toBeNull();
  });
});
