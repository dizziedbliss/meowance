import { useState } from "react";
import { type CourseNeed, paperMarks, planTarget } from "../lib/cgpa";
import type { Attendance } from "../lib/types";

const tone = { open: "text-ink", stretch: "text-fine", capped: "text-fine" } as const;

/** The line for a target that isn't a cgpa at all: empty, zero or below, or past 10. */
function invalidJoke(raw: string | null, target: number): string {
  if (raw === "" || Number.isNaN(target)) return "type a number. any number. we'll judge it after.";
  if (target > 10) return "it's out of 10. not 11, not 'trust me bro'. pick a real one.";
  return "a cgpa of zero or less? the f's would have to try really hard. pick something above 0.";
}

/** "40–49" while a higher score would be the next grade up; just "40" when there's no room to show. */
const seeRange = (c: CourseNeed) =>
  c.seeMax > c.seeNeeded ? `${paperMarks(c.seeNeeded)}–${paperMarks(c.seeMax)}` : `${paperMarks(c.seeNeeded)}`;

/** Pick a cgpa, see what this sem has to deliver: the sgpa, the grade, the see marks per course. */
export function Target({ data, loading }: { data: Attendance; loading: boolean }) {
  const exams = data.exams;
  const [raw, setRaw] = useState<string | null>(null);
  if (!exams) {
    return (
      <p className="mt-8 max-w-[44ch] font-display text-2xl font-bold leading-tight text-ink-2">
        {exams === undefined && loading ? "asking the portal for your past sems…" : "need your past results for this. hit sync."}
      </p>
    );
  }
  const target = raw === null ? Math.min(10, Math.round((exams.cgpa + 0.2) * 10) / 10) : Number(raw);
  const valid = raw === null || (raw !== "" && target > 0 && target <= 10);
  const plan = valid ? planTarget(exams, data.courses, target) : null;
  const lower = valid && target < exams.cgpa; // asking for a worse cgpa than the one you have

  return (
    <div>
      <div className="flex flex-wrap items-end justify-between gap-x-8 gap-y-5 border-b border-hair py-6">
        <div>
          <label htmlFor="target" className="mb-2 block font-mono text-xs text-ink-2">
            target cgpa · now {exams.cgpa.toFixed(2)}
          </label>
          <input
            id="target"
            type="number"
            inputMode="decimal"
            step="0.05"
            min="0"
            max="10"
            value={raw ?? target}
            onChange={(e) => setRaw(e.target.value)}
            className="w-32 border border-brown-line bg-brown px-3 py-2 font-mono text-lg text-ink focus:bg-brown-focus focus:outline-none"
          />
        </div>
        {plan && (
          <div className="text-right">
            <div className={`numeral text-[clamp(72px,12vw,140px)] leading-[.8] tracking-[-.03em] ${plan.reachable ? "text-orange" : "text-dead"}`}>
              {plan.reachable ? Math.max(plan.needSgpa, 0).toFixed(2) : "nope"}
            </div>
            <p className="mt-3 font-mono text-xs text-mut">sgpa you need this sem</p>
          </div>
        )}
      </div>

      {!plan ? (
        <p className="mt-6 max-w-[52ch] font-mono text-xs text-mut">{valid ? "no courses this sem to plan with." : invalidJoke(raw, target)}</p>
      ) : (
        <>
          <p className="py-4 font-mono text-xs text-mut">
            {lower && "you want your cgpa to go down? respect the honesty. "}
            {!plan.reachable
              ? `even your best possible sem only gets you to ${plan.maxCgpa.toFixed(2)}. the portal doesn't do miracles and neither do we. pick a lower one.`
              : plan.needSgpa <= 4
                ? "any pass in everything gets you there. relax, a little."
                : `${plan.courses.some((c) => c.status === "capped") ? "some courses can't climb any higher, so the rest cover for them. " : ""}around ${plan.band.grade} in most courses gets you there. the see range keeps that grade: below it you drop a grade, above it you bump one up (or, for an o, just flex). see marks are the paper's, out of 100, on top of your cie so far.`}
          </p>
          {plan.reachable && (
            <ul>
              {plan.courses.map((c) => (
                <li key={c.code} className="grid items-center gap-x-4 gap-y-1.5 border-t border-hair py-3 md:grid-cols-[minmax(0,1fr)_auto]">
                  <span>
                    <span className="block font-semibold leading-snug">{c.name}</span>
                    <span className="font-mono text-xs text-mut">
                      {c.code.toLowerCase()} · {c.credits} credits · cie {c.cie}/50
                    </span>
                  </span>
<span className="md:text-right">
                    <span className={`block font-mono text-sm ${tone[c.status]}`}>
                      {c.status === "stretch"
                        ? `${c.band.grade} · ${c.seeNeeded - 50} more cie + a perfect see`
                        : c.status === "capped"
                          ? `${c.band.grade} is its best · see ${seeRange(c)}/100`
                          : `${c.band.grade} · see ${seeRange(c)}/100`}
                    </span>
                    {c.seeIfCieFull !== null && (
                      <span className="mt-0.5 block font-mono text-xs text-mut">see {paperMarks(c.seeIfCieFull)}/100 if you win the cie left</span>
                    )}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </div>
  );
}
