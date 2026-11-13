import type { CIEDetails, Course } from "./types";
import type { Zone } from "./verdict";

export type ScoreVerdict = {
  zone: Zone | null; // null = nothing marked yet
  got: number; // marks obtained (portal's total)
  lost: number; // marks gone for good (wrong answers + ghosted tests)
  left: number; // marks still up for grabs this sem
  max: number;
  head: string;
  sub: string;
};

const listNames = (n: string[]) => (n.length <= 1 ? n.join("") : `${n.slice(0, -1).join(", ")} and ${n[n.length - 1]}`);

export function scoreVerdict(c: Course): ScoreVerdict {
  const parts = c.cie ?? [];
  const max = c.cieMax || 50;
  const taken = parts.filter((p) => !p.pending);
  const takenMax = taken.reduce((s, p) => s + p.max, 0);
  const got = c.cieTotal ?? taken.reduce((s, p) => s + (p.absent ? 0 : p.obtained), 0);
  const lost = Math.max(0, takenMax - got);
  const left = Math.max(0, max - takenMax);
  const ghosted = taken.filter((p) => p.absent).map((p) => partLabel(p.name));

  if (taken.length === 0) {
    return { zone: null, got: 0, lost: 0, left: max, max, head: "no marks yet", sub: "the calm before the storm." };
  }
  if (ghosted.length === taken.length) {
    return {
      zone: "dead", got, lost, left, max,
      head: `ghosted ${ghosted.length === 1 ? "the only test" : "every test"}`,
      sub: `0 marks from ${listNames(ghosted)}. bold strategy. ${left} still up for grabs, so it's not over.`,
    };
  }

  const pct = got / takenMax;
  const zone: Zone = pct >= 0.8 ? "safe" : pct >= 0.5 ? "fine" : "dead";
  const head = zone === "safe" ? "cooking fr" : zone === "fine" ? "mid, but alive" : "it's giving retest energy";
  const tail = ghosted.length ? ` also you ghosted ${listNames(ghosted)}. bold move.` : "";
  return { zone, got, lost, left, max, head, sub: `${got} out of ${takenMax} so far.${tail}` };
}

/**
 * The class average for one of the dashboard's tests ("SE1" matches the CIE
 * page's "CIE1" by number), or for the total ("total" → "Final IA").
 */
export function classAvg(part: string, d: CIEDetails | null | undefined): number | null {
  if (!d) return null;
  if (part === "total") return d.stats.find((s) => /final/i.test(s.name))?.classAvg ?? null;
  const n = /(\d+)\s*$/.exec(part)?.[1];
  if (!n) return null;
  return d.stats.find((s) => /^CIE/i.test(s.name) && s.name.endsWith(n))?.classAvg ?? null;
}

/** What a test is called on screen: the dashboard prints "SE1", students call it cie1. */
export const partLabel = (name: string) => name.toLowerCase().replace(/^se(\d+)$/, "cie$1");
