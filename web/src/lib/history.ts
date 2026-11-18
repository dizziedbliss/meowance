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

/** The sem's last working day (the freeze), not the end of the exam period: classes stop here. */
export function lastWorkingDay(cal: Calendar): string {
  const days = cal.events.filter((e) => e.kind === "lastday").map((e) => e.to);
  return days.length ? days.sort().at(-1)! : cal.end;
}

/**
 * The portal's planned total, capped by the credits: a course holds about one
 * class a week per credit, so a 3-credit course over a 15-week sem is 45. Two
 * caps on top of the portal's number: credits x the sem's weeks, and the
 * classes already held plus the classes the days left till the last working day
 * can hold (credits x days / 6). Without the portal's number (history not
 * fetched) the two caps alone decide. Null without the calendar: then we don't guess.
 */
export function cappedTotal(c: Course, planned: number | null, cal: Calendar | null | undefined, today: string): number | null {
  if (!cal || !c.credits) return null;
  const upto = { ...cal, end: lastWorkingDay(cal) };
  const days = (from: string) => workingDaysLeft(upto, from);
  const first = today < cal.start ? cal.start : today;
  const whole = c.credits * Math.round(days(cal.start) / WORKING_DAYS_PER_WEEK);
  const pace = c.conducted + Math.ceil((c.credits * days(first)) / WORKING_DAYS_PER_WEEK);
  return Math.min(planned ?? Infinity, whole, pace);
}

/** Classes still to be held this sem (capped total minus held); null when we can't tell. Never negative. */
export function classesLeft(c: Course, h: CourseHistory | null | undefined, cal: Calendar | null | undefined, today: string): number | null {
  const total = cappedTotal(c, semTotal(h), cal, today);
  return total == null ? null : Math.max(0, total - c.conducted);
}
