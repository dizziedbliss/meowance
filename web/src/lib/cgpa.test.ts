import { describe, expect, it } from "vitest";
import { bandFor, paperMarks, planTarget, pointsSoFar } from "./cgpa";
import type { Course, ExamCourse, ExamHistory } from "./types";

const done = (code: string, credits: number, gpa: number, audit = false): ExamCourse =>
  ({ code, name: code, cie: 0, see: 0, total: 0, date: "", credits, earned: credits, audit, gpa, grade: "" });
// 10 credits at 8 + 10 credits at 7 = 150 points over 20 credits = 7.5 cgpa
const exams = (): ExamHistory => ({
  creditsEarned: 20,
  creditsToEarn: 140,
  cgpa: 7.5,
  sems: [
    { sem: 1, credits: 10, sgpa: 8, cgpa: 8, courses: [done("A", 10, 8), done("NSS", 0, 9, true)] },
    { sem: 2, credits: 20, sgpa: 7, cgpa: 7.5, courses: [done("B", 10, 7)] },
  ],
});
const course = (code: string, credits: number, got: number, left = 0): Course =>
  ({ code, name: code, credits, attended: 1, conducted: 1, percent: 100, need75: 0, need85: 0, skip85: 0, cieTotal: got, cieMax: 50, cie: [{ name: "SE1", max: 50 - left, obtained: got, absent: false, pending: false, entered: "" }] });

describe("bandFor (total out of 100)", () => {
  it("maps marks to the college's grade points", () => {
    expect(bandFor(90)).toMatchObject({ grade: "O", points: 10 });
    expect(bandFor(89)).toMatchObject({ grade: "A+", points: 9 });
    expect(bandFor(70)).toMatchObject({ grade: "A", points: 8 });
    expect(bandFor(60)).toMatchObject({ grade: "B+", points: 7 });
    expect(bandFor(55)).toMatchObject({ grade: "B", points: 6 });
    expect(bandFor(50)).toMatchObject({ grade: "C", points: 5 });
    expect(bandFor(40)).toMatchObject({ grade: "P", points: 4 });
    expect(bandFor(39)).toMatchObject({ grade: "F", points: 0 });
  });
});

describe("paperMarks", () => {
  it("turns the portal's see (out of 50) into marks on the paper (out of 100)", () => {
    expect(paperMarks(40)).toBe(80);
    expect(paperMarks(17.5)).toBe(35); // the 35% pass floor is a clean 35/100
    expect(paperMarks(0)).toBe(0);
    expect(paperMarks(45.1 - 10)).toBe(70.2); // no float noise like 70.19999999999999
  });
});

describe("pointsSoFar", () => {
  it("sums credit × grade point, leaving audit courses out", () => {
    expect(pointsSoFar(exams())).toEqual({ credits: 20, points: 150 });
  });
  it("falls back to the portal's cgpa when no sem is listed", () => {
    expect(pointsSoFar({ creditsEarned: 20, creditsToEarn: 0, cgpa: 7.5, sems: [] })).toEqual({ credits: 20, points: 150 });
  });
});

describe("pointsSoFar and F grades (handbook: F is left out of both sums)", () => {
  it("doesn't count a failed course's credits or points", () => {
    const e = exams();
    e.sems[1].courses.push(done("FAILED", 4, 0));
    expect(pointsSoFar(e)).toEqual({ credits: 20, points: 150 });
  });
});

describe("planTarget", () => {
  // this sem: 10 credits. cgpa 7.5 over 20 credits → target 8 needs (8*30 - 150)/10 = 9 sgpa
  const sem = [course("X", 6, 40), course("Y", 4, 30)];
  it("works out the sgpa the target needs", () => {
    const p = planTarget(exams(), sem, 8);
    expect(p?.needSgpa).toBeCloseTo(9);
    expect(p?.reachable).toBe(true);
    expect(p?.band.grade).toBe("A+");
  });
  it("tells each course its grade and what the see has to bring (cie so far counts)", () => {
    const p = planTarget(exams(), sem, 8)!;
    // A+ is 80: 80 - 40 cie = 40 from the see; 80 - 30 = 50
    expect(p.courses.map((c) => [c.code, c.band.grade, c.seeNeeded, c.status])).toEqual([["X", "A+", 40, "open"], ["Y", "A+", 50, "open"]]);
  });
  it("gives the top of the grade too: the most the see can score before it becomes the next grade", () => {
    const p = planTarget(exams(), sem, 8)!;
    // A+ is 80-89 total: cie 40 → see 40..49 (50 would make it an O); cie 30 → see 50, nothing above
    expect(p.courses.map((c) => [c.seeNeeded, c.seeMax])).toEqual([[40, 49], [50, 50]]);
  });
  it("has no ceiling to show for an O, or under the pass floor", () => {
    const o = planTarget(exams(), [course("X", 10, 45)], 9.5)!; // needs O: 90 - 45 = 45, up to a perfect 50
    expect(o.courses[0]).toMatchObject({ seeNeeded: 45, seeMax: 50 });
    // cie 40 + the 17.5 pass floor is already 57.5, a B: nobody with this cie can end up on a P
    const p = planTarget(exams(), [course("X", 10, 40)], 6)!;
    expect(p.courses[0]).toMatchObject({ seeNeeded: 17.5, seeMax: 19 });
    expect(p.courses[0].band.grade).toBe("B");
  });
  it("shows the grade a bare pass really gives, never a lower one", () => {
    const p = planTarget(exams(), [course("X", 10, 45)], 6)!; // 45 + 17.5 = 62.5
    expect(p.courses[0].band.grade).toBe("B+");
  });
  it("calls a capped course that also needs more cie a stretch, not a see above 100", () => {
    // X: cie 15 with 20 to come, best is A+ (needs 65 from the see now); Y carries with an O
    const p = planTarget(exams(), [course("X", 5, 15, 20), course("Y", 5, 50)], 8.1)!;
    expect(p.courses[0]).toMatchObject({ band: { grade: "A+" }, seeNeeded: 65, seeMax: 50, status: "stretch" });
  });
  it("also gives the see needed if every cie mark still to come is won", () => {
    // A+ is 80: cie 40 with 10 still up for grabs → 40 from the see now, 30 if the last 10 are won
    const p = planTarget(exams(), [course("X", 10, 40, 10)], 8)!;
    expect(p.courses[0]).toMatchObject({ seeNeeded: 40, seeIfCieFull: 30 });
  });
  it("keeps the pass floor in that figure, and skips it when no cie is left", () => {
    const p = planTarget(exams(), [course("X", 10, 40, 10), course("Y", 10, 40)], 8)!;
    expect(p.courses[1].seeIfCieFull).toBeNull();
    const easy = planTarget(exams(), [course("X", 10, 45, 5)], 6)!; // a P needs 40: 17.5 floor either way
    expect(easy.courses[0].seeIfCieFull).toBeNull(); // same as seeNeeded, so nothing to add
  });
  it("never asks for less than the pass floor in the see: 35% of 50", () => {
    // target 6 needs only a P (40), and cie 40 would "need" 0 from the see, but the see must still clear 17.5
    const p = planTarget(exams(), [course("X", 10, 40)], 6)!;
    expect(p.courses[0]).toMatchObject({ seeNeeded: 17.5, status: "open" });
  });
  it("lets the other courses cover for one that can't get there", () => {
    // X tops out at A (cie 20 + a perfect see = 70), so Y has to carry it with an O
    const p = planTarget(exams(), [course("X", 5, 20), course("Y", 5, 50)], 8)!;
    expect(p.reachable).toBe(true);
    expect(p.courses.map((c) => [c.code, c.band.grade, c.seeNeeded, c.status])).toEqual([["X", "A", 50, "capped"], ["Y", "O", 40, "open"]]);
  });
  it("is unreachable when even every course at its best isn't enough", () => {
    const p = planTarget(exams(), [course("X", 10, 20)], 8)!; // best is an A: 80 points, 90 needed
    expect(p.reachable).toBe(false);
    expect(p.maxCgpa).toBeCloseTo((150 + 80) / 30);
  });
  it("says a course is a stretch if only the cie still to come saves it", () => {
    const p = planTarget(exams(), [course("X", 10, 25, 20)], 8)!;
    expect(p.courses[0]).toMatchObject({ seeNeeded: 55, status: "stretch" });
  });
  it("drops a course to a lower grade when the others already cover the target", () => {
    // target 8.1 needs 93 points over 10 credits: a level of O in both overshoots, X at O + Y at A+ (95) is enough
    const p = planTarget(exams(), [course("X", 5, 50), course("Y", 5, 40)], 8.1)!;
    const [x, y] = p.courses;
    expect(x.band.points * 5 + y.band.points * 5).toBeGreaterThanOrEqual(p.needSgpa * 10);
    expect(Math.min(x.band.points, y.band.points)).toBeLessThan(p.band.points);
  });
  it("is already done when the target is at or below the current cgpa", () => {
    const p = planTarget(exams(), sem, 6)!; // needs an sgpa of 3: any pass does it
    expect(p.needSgpa).toBeCloseTo(3);
    expect(p.band.grade).toBe("P");
  });
  it("is unreachable above the best a single sem can do", () => {
    const p = planTarget(exams(), sem, 9.5)!;
    expect(p.reachable).toBe(false);
    expect(p.maxCgpa).toBeCloseTo((150 + 96) / 30); // X at O (60) + Y at A+ (36): cie 30 + 50 = 80
  });
  it("is null with nothing to plan", () => {
    expect(planTarget(exams(), [], 8)).toBeNull();
    expect(planTarget(undefined, sem, 8)).toBeNull();
  });
});
