import type { Course } from "./types";

// MCE rules: 85+ is safe, 75–85 means a fine, under 75 nobody (not even a teacher) gets you past 75.
export type Zone = "dead" | "fine" | "safe";

export const zoneName: Record<Zone, string> = { dead: "dead zone", fine: "fine zone", safe: "purring" };

/** A headline with one number to highlight: `${before}${n}${after}`. */
export type Head = { before: string; n: number | null; after: string };

export type Verdict = {
  zone: Zone;
  head: Head;
  sub: string;
  then?: { head: Head; sub: string };
  cooked?: boolean; // the target can't be hit with the classes left: headline is a roast, sub is the small why
};

/**
 * MCE's lines: 85+ safe, 75–85 fine zone, under 75 dead. Read from the server's
 * exact need counts (pkg/attend), not the one-decimal percent, so a rounded
 * "85.0%" that is really 84.96 still says fine zone, matching its "attend 1 more".
 */
export function zoneOf(c: Pick<Course, "need75" | "need85">): Zone {
  if (c.need85 === 0) return "safe";
  if (c.need75 === 0) return "fine";
  return "dead";
}

const classes = (n: number) => (n === 1 ? "class" : "classes");
const lives = (n: number) => (n === 1 ? "life" : "lives");
export const fmtPct = (p: number) => (Math.round(p * 10) / 10).toFixed(1);

/** Same course, same line every render (no flicker), different courses get different ones. */
function pick(pool: string[], seed: string): string {
  let h = 0;
  for (const ch of seed) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
  return pool[h % pool.length];
}

const cooked = [
  "ur beyond cooked-",
  "beyond cooked fr-",
  "ur cooked. no cap.",
  "cooked. well done.",
  "ur already burnt lol",
  "beyond cooked heh-",
];

/**
 * `left` is the classes still to be held this sem (null = unknown). When a
 * target needs more than that, it's out of reach: say so instead of a number
 * nobody can attend.
 */
export function verdict(c: Course, left: number | null = null): Verdict {
  if (c.conducted === 0) {
    return { zone: "safe", head: { before: "nothing yet", n: null, after: "" }, sub: "no classes conducted. enjoy it while it lasts." };
  }
  const zone = zoneOf(c);

  if (zone === "safe") {
    const s = c.skip85;
    if (s > 0) {
      return {
        zone,
        head: { before: "", n: s, after: ` ${lives(s)} left` },
        sub: `that's ${s} ${classes(s)} you can bunk and still sit above 85. touch grass, you earned it.`,
      };
    }
    const drop = fmtPct((c.attended / (c.conducted + 1)) * 100);
    return { zone, head: { before: "", n: 0, after: " lives left" }, sub: `one bunk and you drop to ${drop}%. do not even think about it.` };
  }

  if (zone === "fine") {
    const n = c.need85;
    if (left !== null && n > left) {
      return {
        zone,
        head: { before: pick(cooked, c.code), n: null, after: "" },
        sub: `need ${n} more for 85 but only ${left} ${classes(left)} left. the fine's coming.`,
        cooked: true,
      };
    }
    return {
      zone,
      head: { before: "attend ", n, after: ` more ${classes(n)}` },
      sub: n <= 4 ? "so close it's embarrassing. lock in." : "then you're off the fine list. lock in fr.",
    };
  }

  if (left !== null && c.need75 > left) {
    return {
      zone,
      head: { before: pick(cooked, c.code), n: null, after: "" },
      sub: `need ${c.need75} more just for 75 but only ${left} ${classes(left)} left.`,
      cooked: true,
    };
  }
  const gone85 = left !== null && c.need85 > left;
  return {
    zone,
    head: { before: "attend ", n: c.need75, after: ` more ${classes(c.need75)}` },
    sub: "that's just to crawl back to 75. teachers won't carry you past that, no cap.",
    then: {
      head: gone85 ? { before: "85's out of reach", n: null, after: "" } : { before: "", n: c.need85, after: " more for 85 tho." },
      sub: gone85 ? `only ${left} ${classes(left!)} left, so the fine's staying.` : "that's where the fine stops. zero bunks till then, you're on thin ice.",
    },
  };
}

/** Worst first. */
/** Worst first; a course with no classes held yet goes last (its 0% isn't a problem child). */
export function byPercent(courses: Course[]): Course[] {
  const held = (c: Course) => (c.conducted > 0 ? 0 : 1);
  return [...courses].sort((a, b) => held(a) - held(b) || a.percent - b.percent);
}

/**
 * The predicted bunks the whole sem still allows, from the portal's planned
 * total (holidays already left out). An extra under each course, never the
 * main verdict: a prediction mustn't talk anyone out of classes. Its
 * disclaimer lives in the dashboard's "facts to know".
 */
export function predicted(c: Course, total: number): { n: number; line: string } {
  const missed = c.conducted - c.attended;
  const b85 = Math.floor(total * 0.15 + 1e-9) - missed;
  const b75 = Math.floor(total * 0.25 + 1e-9) - missed;
  const bunks = (n: number) => `${n} ${n === 1 ? "bunk" : "bunks"}`;
  if (b85 > 0) return { n: b85, line: `predicted: ${bunks(b85)} left this sem` };
  if (b85 === 0) return { n: 0, line: "predicted: no bunks left this sem" };
  if (b75 > 0) return { n: b75, line: `predicted: 85's gone, ${bunks(b75)} left before 75` };
  if (b75 === 0) return { n: 0, line: "predicted: 85's gone, no bunks left before 75" };
  return { n: 0, line: "predicted: even 75's out of reach this sem" };
}
