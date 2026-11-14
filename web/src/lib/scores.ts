import type { CIEPart, Course } from "./types";
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

/** One half of the SEE minimum: marks so far, the line, tests still to come. */
export type SeePart = { got: number; need: number; left: number; status: "done" | "open" | "gone" };

/** The last batch on the handbook's scheme (2024-25: 3 cie + 2 activities, bands, pass marks); later intakes changed it. */
export const SCHEME_LAST_BATCH = 24;
export const onHandbookScheme = (usn: string) => Number(usn.slice(3, 5)) <= SCHEME_LAST_BATCH;
const isCie = (name: string) => /^(se|cie)\s*\d/i.test(name);

function seePart(parts: CIEPart[], need: number, tests: number): SeePart {
  const marked = parts.filter((p) => !p.pending);
  const got = marked.reduce((n, p) => n + (p.absent ? 0 : p.obtained), 0);
  const left = Math.max(0, tests - marked.length);
  const status = got >= need ? "done" : got + left * 10 >= need ? "open" : "gone";
  return { got, need, left, status };
}

/**
 * The minimum CIE marks to sit the SEE, for the 24 batch and older: 12 of 30
 * from three cies and 8 of 20 from two activities. Null for juniors.
 */
export function seeMin(c: Course, usn: string): { cie: SeePart; act: SeePart } | null {
  if (!onHandbookScheme(usn)) return null;
  const parts = c.cie ?? [];
  return { cie: seePart(parts.filter((p) => isCie(p.name)), 12, 3), act: seePart(parts.filter((p) => !isCie(p.name)), 8, 2) };
}

/** What a test is called on screen: the dashboard prints "SE1", students call it cie1. */
export const partLabel = (name: string) => name.toLowerCase().replace(/^se(\d+)$/, "cie$1");
