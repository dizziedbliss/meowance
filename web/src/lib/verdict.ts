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
 * The verdict from the whole sem instead of "classes in a row": the portal's
 * planned total for the course (holidays already left out) sets how many bunks
 * 85 (or 75) allows all sem. Calm early in the sem, and not thrown by the
 * portal being weeks behind on entering classes.
 */
/** Bunks the whole sem still allows: for 85, or for 75 once 85 is gone (negative: over even 75's). */
export function semBudget(c: Course, total: number) {
  const missed = c.conducted - c.attended;
  const allow85 = Math.floor(total * 0.15 + 1e-9);
  const allow75 = Math.floor(total * 0.25 + 1e-9);
  return { missed, allow85, allow75, b85: allow85 - missed, b75: allow75 - missed };
}

export function semVerdict(c: Course, total: number): Verdict {
  const { missed, allow85, allow75, b85, b75 } = semBudget(c, total);
  if (b85 > 0) {
    return {
      zone: c.percent >= 85 ? "safe" : "fine",
      head: { before: "you can bunk ", n: b85, after: " more this sem" },
      sub: `${missed} bunked so far. the sem has ${total} classes, and 85 lets you miss ${allow85}.`,
    };
  }
  if (b85 === 0) {
    return {
      zone: "fine",
      head: { before: "", n: 0, after: " bunks left this sem" },
      sub: `you've used all ${allow85} the sem allows for 85. show up to every class from here.`,
    };
  }
  if (b75 >= 0) {
    return {
      zone: "fine",
      head: { before: "", n: b75, after: " bunks left before 75" },
      sub: `85's gone this sem: ${missed} missed, it only allowed ${allow85}. that's the fine. hold 75 at least.`,
    };
  }
  return {
    zone: "dead",
    head: { before: "cooked for the sem", n: null, after: "" },
    sub: `${missed} missed, more than the ${allow75} even 75 allows. talk to your faculty.`,
  };
}
