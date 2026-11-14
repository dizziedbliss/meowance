import { scoreVerdict } from "./scores";
import type { Course, ExamHistory } from "./types";

export type Band = { min: number; grade: string; points: number };

/** The college's grade bands: total marks out of 100 (cie 50 + see 50) → letter and grade point. */
export const BANDS: Band[] = [
  { min: 90, grade: "O", points: 10 },
  { min: 80, grade: "A+", points: 9 },
  { min: 70, grade: "A", points: 8 },
  { min: 60, grade: "B+", points: 7 },
  { min: 55, grade: "B", points: 6 },
  { min: 50, grade: "C", points: 5 },
  { min: 40, grade: "P", points: 4 },
];
const FAIL: Band = { min: 0, grade: "F", points: 0 };
const SEE_MAX = 50;
/** Handbook: the see needs 35% on its own to pass, whatever the cie. */
const SEE_PASS = 17.5;

/** The see paper is out of 100; the portal and the totals use half of it. */
export const paperMarks = (see50: number) => Math.round(see50 * 20) / 10;

export const bandFor = (total: number): Band => BANDS.find((b) => total >= b.min) ?? FAIL;

/** Credits and credit × grade-point so far. Audit courses carry no credits; an F is left out of both sums. */
export function pointsSoFar(exams: ExamHistory): { credits: number; points: number } {
  const done = exams.sems.flatMap((s) => s.courses).filter((c) => !c.audit && c.credits > 0 && c.gpa > 0);
  if (done.length === 0) return { credits: exams.creditsEarned, points: exams.cgpa * exams.creditsEarned };
  return { credits: done.reduce((s, c) => s + c.credits, 0), points: done.reduce((s, c) => s + c.credits * c.gpa, 0) };
}

export type CourseNeed = {
  code: string;
  name: string;
  credits: number;
  cie: number; // marks so far
  band: Band; // the grade this course is asked for
  seeNeeded: number; // see marks (of 50) for that grade, counting only the cie so far; never under the pass floor
  seeMax: number; // the most the see can score and still be this grade (the next grade starts a mark higher); at most 50
  /** The see needed if every cie mark still to come is won; null when no cie is left or it changes nothing. */
  seeIfCieFull: number | null;
  /**
   * open: doable; stretch: only if you also grab the cie marks still to come;
   * capped: this is the best grade the course can still give, so the others cover for it
   */
  status: "open" | "stretch" | "capped";
};
export type Plan = {
  needSgpa: number; // the sgpa this sem has to reach, unrounded (can be above 10 or below 0)
  reachable: boolean; // false when even every course at its best grade can't get there
  maxCgpa: number; // the cgpa after every course lands its best possible grade
  band: Band; // the level most courses are asked for; capped courses sit below it, easy ones may too
  courses: CourseNeed[];
};

const idx = (b: Band) => BANDS.indexOf(b);

/**
 * What this sem has to look like to reach a target cgpa; null with nothing to plan.
 * The sgpa is one number across all courses, so a course that can't reach a grade is
 * capped at its best and the others make up for it.
 */
export function planTarget(exams: ExamHistory | null | undefined, courses: Course[], target: number): Plan | null {
  const sem = courses.filter((c) => c.credits > 0);
  const credits = sem.reduce((s, c) => s + c.credits, 0);
  if (!exams || credits === 0) return null;
  const sofar = pointsSoFar(exams);
  const total = sofar.credits + credits;
  const needSgpa = (target * total - sofar.points) / credits;
  const needPoints = needSgpa * credits;

  // The best each course can still do: its cie so far, the cie still to come, and a perfect see.
  const cies = sem.map(scoreVerdict); // marks so far and marks still up for grabs, as the "this sem" tab counts them
  const caps = cies.map((v) => bandFor(v.got + v.left + SEE_MAX));
  const points = (bands: Band[]) => bands.reduce((s, b, i) => s + b.points * sem[i].credits, 0);
  const maxPoints = points(caps);
  const maxCgpa = (sofar.points + maxPoints) / total;
  const reachable = maxPoints >= needPoints;

  // Lowest level L where every course at min(L, its best) is enough, then ease off the hardest ones.
  // A course can't land below what a bare pass in the see gives (cie + 17.5), nor above its best.
  const floors = cies.map((v) => bandFor(v.got + SEE_PASS));
  const clamp = (i: number, b: Band) => (b.points > caps[i].points ? caps[i] : b.points < floors[i].points ? floors[i] : b);
  const at = (b: Band) => caps.map((_, i) => clamp(i, b));
  const level = [...BANDS].reverse().find((b) => points(at(b)) >= needPoints) ?? BANDS[0];
  const pick = at(level);
  const need = (i: number) => pick[i].min - cies[i].got;
  if (reachable) {
    for (let moved = true; moved; ) {
      moved = false;
      const order = pick.map((_, i) => i).sort((a, b) => need(b) - need(a));
      for (const i of order) {
        const lower = BANDS[idx(pick[i]) + 1];
        if (!lower || lower.points < floors[i].points) continue;
        const trial = pick.map((b, j) => (j === i ? lower : b));
        if (points(trial) >= needPoints) {
          pick[i] = lower;
          moved = true;
          break;
        }
      }
    }
  }

  return {
    needSgpa,
    reachable,
    maxCgpa,
    band: level,
    courses: sem.map((c, i) => {
      const cie = cies[i].got;
      const band = pick[i];
      const seeNeeded = Math.max(SEE_PASS, band.min - cie);
      const above = BANDS[idx(band) - 1];
      const seeMax = Math.min(SEE_MAX, Math.max(seeNeeded, (above ? above.min - 1 : 100) - cie));
      const full = Math.max(SEE_PASS, band.min - cie - cies[i].left);
      const seeIfCieFull = full < seeNeeded ? full : null;
      const status = seeNeeded > SEE_MAX ? "stretch" : band === caps[i] && caps[i].points < level.points ? "capped" : "open";
      return { code: c.code, name: c.name, credits: c.credits, cie, band, seeNeeded, seeMax, seeIfCieFull, status };
    }),
  };
}
