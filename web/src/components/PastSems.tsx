import { useState } from "react";
import type { ExamHistory } from "../lib/types";
import { SubTabs } from "./SubTabs";

const gradeTone = (gpa: number) => (gpa >= 8 ? "text-safe" : gpa >= 6 ? "text-ink" : gpa > 0 ? "text-fine" : "text-dead");

/** Past semesters, one at a time: the cgpa up top, then one sem's grades. */
export function PastSems({ exams, loading }: { exams: ExamHistory | null | undefined; loading: boolean }) {
  const [picked, setPicked] = useState<number | null>(null);
  if (!exams) {
    return (
      <p className="mt-8 max-w-[44ch] font-display text-2xl font-bold leading-tight text-ink-2">
        {exams === undefined && loading ? "asking the portal for your past sems…" : "couldn't grab your results this time. hit sync."}
      </p>
    );
  }
  if (exams.sems.length === 0) {
    return <p className="mt-8 font-display text-2xl font-bold text-ink-2">no results yet. first sem, huh.</p>;
  }
  const sem = exams.sems.find((s) => s.sem === picked) ?? exams.sems[exams.sems.length - 1];

  return (
    <div>
      <div className="flex flex-wrap items-end justify-between gap-x-6 gap-y-2 border-b border-hair py-6">
        <div>
          <div className="numeral text-[clamp(72px,12vw,140px)] leading-[.8] tracking-[-.03em] text-orange">{exams.cgpa.toFixed(2)}</div>
          <p className="mt-3 font-mono text-xs text-mut">
            cgpa · {exams.creditsEarned} credits earned · {exams.creditsToEarn} to go
          </p>
        </div>
        <div className="flex items-center gap-3">
          <span className="font-mono text-xs text-mut">sem</span>
          <SubTabs
            label="Semester"
            tabs={exams.sems.map((s) => ({ id: String(s.sem), text: String(s.sem) }))}
            active={String(sem.sem)}
            onTab={(id) => setPicked(+id)}
          />
        </div>
      </div>

      <p className="py-4 font-mono text-xs text-mut">
        sem {sem.sem} · sgpa <b className="font-medium text-ink">{sem.sgpa.toFixed(2)}</b> · cgpa after it {sem.cgpa.toFixed(2)}
      </p>
      <ul>
        {sem.courses.map((c) => (
          <li
            key={c.code}
            className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 border-t border-hair py-3 md:grid-cols-[minmax(0,1fr)_180px_60px]"
          >
            <span>
              <span className="block font-semibold leading-snug">{c.name}</span>
              <span className="font-mono text-xs text-mut">
                {c.code.toLowerCase()} · {c.audit ? "audit" : `${c.credits} credits`}
                {c.date && ` · ${c.date.toLowerCase()}`}
              </span>
            </span>
            <span className="hidden font-mono text-xs text-ink-2 md:block">
              cie {c.cie} + see {c.see} = {c.total}
            </span>
            <span className={`numeral text-right text-[34px] leading-none ${gradeTone(c.gpa)}`}>{c.grade}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
