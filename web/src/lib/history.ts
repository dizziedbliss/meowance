import { workingDaysLeft } from "./events";
import type { Calendar, ClassRecord, Course, CourseHistory } from "./types";

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

const WORKING_DAYS_PER_WEEK = 6; // mon-sat; Sundays and marked holidays are already out of the count

/**
 * The portal's planned total, capped by the credits: a course holds about one
 * class a week per credit, so a 3-credit course over a 15-week sem is 45. Two
 * caps on top of the portal's number: credits x the sem's weeks, and the
 * classes already held plus credits x the weeks left till the last working day.
 * Without the calendar the portal's number stands.
 */
export function cappedTotal(c: Course, planned: number | null, cal: Calendar | null | undefined, today: string): number | null {
  if (planned == null || !cal || !c.credits) return planned;
  const weeks = (from: string) => Math.round(workingDaysLeft(cal, from) / WORKING_DAYS_PER_WEEK);
  const left = weeks(today < cal.start ? cal.start : today > cal.end ? cal.end : today);
  const whole = c.credits * weeks(cal.start);
  const pace = c.conducted + c.credits * left;
  return Math.min(planned, whole, pace);
}
