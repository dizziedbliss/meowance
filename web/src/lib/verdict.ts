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
};

export function zoneOf(percent: number): Zone {
  if (percent >= 85) return "safe";
  if (percent >= 75) return "fine";
  return "dead";
}

const classes = (n: number) => (n === 1 ? "class" : "classes");
const lives = (n: number) => (n === 1 ? "life" : "lives");
export const fmtPct = (p: number) => (Math.round(p * 10) / 10).toFixed(1);

export function verdict(c: Course): Verdict {
  if (c.conducted === 0) {
    return { zone: "safe", head: { before: "nothing yet", n: null, after: "" }, sub: "no classes conducted. enjoy it while it lasts." };
  }
  const zone = zoneOf(c.percent);

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
    return {
      zone,
      head: { before: "attend ", n, after: ` more ${classes(n)}` },
      sub: n <= 4 ? "so close it's embarrassing. lock in." : "then you're off the fine list. lock in fr.",
    };
  }

  return {
    zone,
    head: { before: "attend ", n: c.need75, after: ` more ${classes(c.need75)}` },
    sub: "that's just to crawl back to 75. teachers won't carry you past that, no cap.",
    then: {
      head: { before: "", n: c.need85, after: " more for 85 tho." },
      sub: "that's where the fine stops. zero bunks till then, you're on thin ice.",
    },
  };
}

/** Worst first. */
export function byPercent(courses: Course[]): Course[] {
  return [...courses].sort((a, b) => a.percent - b.percent);
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
