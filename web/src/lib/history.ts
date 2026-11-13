import type { ClassRecord, Course, CourseHistory } from "./types";
import { semBudget } from "./verdict";

/** The sem view of a course once its history is in: the planned total, and the bunks it allows. */
export type Sem = { total: number; budget: { n: number; label: string } };

export function semOf(c: Course, h: CourseHistory | null | undefined): Sem | null {
  if (!h) return null;
  const total = h.present + h.absent + h.stillToGo; // the portal's plan for the sem, holidays already left out
  if (total <= 0) return null;
  const b = semBudget(c, total);
  const budget = b.b85 >= 0 ? { n: b.b85, label: b.b85 === 1 ? "life" : "lives" } : b.b75 >= 0 ? { n: b.b75, label: "before 75" } : { n: 0, label: "" };
  return { total, budget };
}

/**
 * The course's classes in real order, for dated tally boxes; null when the
 * class-by-class page disagrees with the dashboard's counts (then the boxes stay undated).
 */
export function squares(attended: number, conducted: number, classes: ClassRecord[] | undefined): ClassRecord[] | null {
  if (!classes || classes.length !== conducted || classes.filter((c) => c.present).length !== attended) return null;
  return classes;
}
