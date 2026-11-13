import type { ClassRecord } from "./types";
import { type Zone, zoneOf } from "./verdict";

/** Classes you can bunk out of `left` and still end the sem at `target` (negative: out of reach). */
const canBunk = (attended: number, conducted: number, left: number, target: number) =>
  Math.floor(attended + left - target * (conducted + left) + 1e-9);

/**
 * How many of the classes the portal still has planned you can bunk. The portal's
 * counts are often stale, so this is "you can bunk", never "you must attend";
 * the screen adds that it only holds if the faculty takes every class left.
 */
export function bunksLeft(attended: number, conducted: number, left: number): { zone: Zone; line: string } | null {
  if (left <= 0) return null;
  const zone = zoneOf(conducted ? (attended / conducted) * 100 : 100);
  const k85 = canBunk(attended, conducted, left, 0.85);
  if (k85 >= left) return { zone, line: `you could bunk all ${left} left and still end on 85.` };
  if (k85 > 0) return { zone, line: `you can bunk ${k85} of the ${left} left before the sem ends.` };
  if (k85 === 0) return { zone, line: "no bunks left. miss one more and you end under 85." };
  const k75 = canBunk(attended, conducted, left, 0.75);
  if (k75 > 0) return { zone, line: `85's gone this sem. you can bunk ${k75} of the ${left} left and still end on 75.` };
  if (k75 === 0) return { zone, line: `85's gone this sem. bunk none of the ${left} left and you scrape 75.` };
  return { zone, line: `cooked. even all ${left} left won't get you to 75.` };
}

/**
 * The course's classes in real order, for dated tally boxes; null when the
 * class-by-class page disagrees with the dashboard's counts (then the boxes stay undated).
 */
export function squares(attended: number, conducted: number, classes: ClassRecord[] | undefined): ClassRecord[] | null {
  if (!classes || classes.length !== conducted || classes.filter((c) => c.present).length !== attended) return null;
  return classes;
}
