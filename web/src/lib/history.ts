import type { ClassRecord, CourseHistory } from "./types";

/** The portal's planned classes for the whole sem (holidays already left out), once the history is in. */
export function semTotal(h: CourseHistory | null | undefined): number | null {
  if (!h) return null;
  const total = h.present + h.absent + h.stillToGo;
  return total > 0 ? total : null;
}

/**
 * The course's classes in real order, for dated tally boxes; null when the
 * class-by-class page disagrees with the dashboard's counts (then the boxes stay undated).
 */
export function squares(attended: number, conducted: number, classes: ClassRecord[] | undefined): ClassRecord[] | null {
  if (!classes || classes.length !== conducted || classes.filter((c) => c.present).length !== attended) return null;
  return classes;
}
