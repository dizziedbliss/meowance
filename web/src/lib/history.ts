import type { ClassRecord } from "./types";
import type { Zone } from "./verdict";

const months = ["jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"];

/** Classes grouped by month: newest month first, classes oldest first inside each. */
export function byMonth(classes: ClassRecord[]): { label: string; classes: ClassRecord[] }[] {
  const groups = new Map<string, ClassRecord[]>();
  for (const c of classes) {
    const key = c.date.slice(0, 7); // YYYY-MM
    groups.set(key, [...(groups.get(key) ?? []), c]);
  }
  return [...groups.keys()]
    .sort()
    .reverse()
    .map((k) => ({ label: months[+k.slice(5, 7) - 1] ?? k, classes: groups.get(k)! }));
}

/** The run of pawsents (or bunks) at the end of the record. */
export function lastRun(classes: ClassRecord[]): { present: boolean; n: number } | null {
  if (classes.length === 0) return null;
  const present = classes[classes.length - 1].present;
  let n = 0;
  for (let i = classes.length - 1; i >= 0 && classes[i].present === present; i--) n++;
  return { present, n };
}

/** Classes you must attend out of `left` to end the sem at `target`; ≤ 0 means none. */
const toEnd = (attended: number, conducted: number, left: number, target: number) =>
  Math.ceil(target * (conducted + left) - attended - 1e-9);

/**
 * Where the sem ends, using the portal's own count of classes still planned.
 * Null when nothing is left to plan around.
 */
export function endOfSem(attended: number, conducted: number, left: number): { zone: Zone; line: string } | null {
  if (left <= 0) return null;
  const k85 = toEnd(attended, conducted, left, 0.85);
  if (k85 <= 0) return { zone: "safe", line: `safe even if you bunk all ${left} left. you won't, but you could.` };
  if (k85 <= left) return { zone: "fine", line: `${left} to go. attend ${k85} of them and you end on 85.` };
  const k75 = toEnd(attended, conducted, left, 0.75);
  if (k75 <= left) return { zone: "fine", line: `85's out of reach this sem. attend ${Math.max(k75, 0)} of the ${left} left to end on 75.` };
  return { zone: "dead", line: `cooked. even all ${left} left won't get you to 75.` };
}
