import { isoDayMonth } from "../lib/dates";
import { byMonth, endOfSem, lastRun } from "../lib/history";
import type { Course, CourseHistory } from "../lib/types";

const tone = { dead: "text-dead", fine: "text-fine", safe: "text-safe" } as const;

/** One course's receipts: every class by month, who teaches it, and where the sem ends. */
export function HistoryPanel({ course, history, loading }: { course: Course; history: CourseHistory | null | undefined; loading: boolean }) {
  if (history === undefined) {
    return <p className="py-4 font-mono text-xs text-mut">{loading ? "asking the portal for the receipts…" : "couldn't grab this one. hit sync."}</p>;
  }
  if (history === null) {
    return <p className="py-4 font-mono text-xs text-mut">couldn't grab this one. the portal's being the portal. hit sync.</p>;
  }
  const end = endOfSem(course.attended, course.conducted, history.stillToGo);
  const run = lastRun(history.classes);

  return (
    <div className="grid gap-3 py-4 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] md:gap-x-8">
      <div>
        <p className="font-mono text-xs text-mut">
          {history.faculty && <>taught by {history.faculty.toLowerCase()} · </>}
          {history.stillToGo} still to go this sem
        </p>
        {end && <p className={`mt-2 font-display text-lg font-bold leading-snug ${tone[end.zone]}`}>{end.line}</p>}
        {run && (
          <p className="mt-1 text-sm text-ink-2">
            {run.present
              ? `last ${run.n === 1 ? "class" : `${run.n} classes`}: pawsent.`
              : `bunked the last ${run.n === 1 ? "one" : run.n}.`}
          </p>
        )}
      </div>
      <ul className="space-y-1.5">
        {byMonth(history.classes).map((m) => (
          <li key={m.label} className="grid grid-cols-[32px_minmax(0,1fr)] items-center gap-2">
            <span className="font-mono text-[11px] text-mut">{m.label}</span>
            <span className="tally" role="img" aria-label={`${m.label}: ${m.classes.filter((c) => c.present).length} pawsent, ${m.classes.filter((c) => !c.present).length} bunked`}>
              {m.classes.map((c) => (
                <i key={c.date + c.start} className={c.present ? "at" : "ms"} title={`${isoDayMonth(c.date)} · ${c.start}–${c.end} · ${c.present ? "pawsent" : "bunked"}`} />
              ))}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
