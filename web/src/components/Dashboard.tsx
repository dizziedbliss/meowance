import { squares } from "../lib/history";
import type { Attendance, Course } from "../lib/types";
import { byPercent } from "../lib/verdict";
import { CourseRow } from "./CourseRow";
import { HistoryPanel } from "./HistoryPanel";
import { Slab } from "./Slab";

type Props = { data: Attendance; open: string | null; onOpen: (code: string | null) => void; loading: boolean };

/** One course open at a time, so the page never grows into a scroll marathon. */
export function Dashboard({ data, open, onOpen, loading }: Props) {
  const [worst, ...rest] = byPercent(data.courses);
  const toggle = (code: string) => () => onOpen(open === code ? null : code);
  const dated = (c: Course) => squares(c.attended, c.conducted, data.history?.[c.code]?.classes);
  const panel = (code: string) =>
    open === code && (
      <div className={`border-b border-hair ${code === worst?.code ? "pt-4" : ""}`}>
        <HistoryPanel course={data.courses.find((c) => c.code === code)!} history={data.history?.[code]} loading={loading} />
      </div>
    );
  return (
    <>
      {worst && <Slab course={worst} classes={dated(worst)} open={open === worst.code} onToggle={toggle(worst.code)} />}
      {worst && panel(worst.code)}

      {rest.length > 0 && (
        <>
          <div className="flex flex-wrap items-baseline justify-between gap-3 border-b border-hair-2 pt-10 pb-1.5">
            <h2 className="font-display text-2xl font-extrabold tracking-[-.01em]">everything else</h2>
            <div aria-label="Tally key" className="flex flex-wrap gap-4 font-mono text-[11px] text-mut">
              <span className="inline-flex items-center gap-1.5"><i className="inline-block h-2.5 w-2.5 bg-ink" />pawsent</span>
              <span className="inline-flex items-center gap-1.5"><i className="inline-block h-2.5 w-2.5 shadow-[inset_0_0_0_1.5px_var(--color-mut)]" />bunked</span>
              <span className="inline-flex items-center gap-1.5"><i className="inline-block h-2.5 w-2.5 shadow-[inset_0_0_0_1.5px_var(--color-orange)]" />still owe</span>
            </div>
          </div>
          {rest.map((c) => (
            <div key={c.code}>
              <CourseRow course={c} classes={dated(c)} open={open === c.code} onToggle={toggle(c.code)} />
              {panel(c.code)}
            </div>
          ))}
        </>
      )}

      <footer className="mt-10 grid gap-1.5 md:grid-cols-[140px_minmax(0,1fr)] md:gap-5">
        <span className="font-mono text-xs text-mut">facts to know</span>
        <div className="max-w-[68ch] space-y-2 text-[13.5px] text-ink-2">
          <p>we're not mce. not affiliated, not sponsored, not even friends. just kids who got tired of doing this math by hand.</p>
          <p>
            we don't keep your data, never. usn and birthday go in, attendance comes out, we forget you even exist. your attendance keeps
            the bill. <strong className="text-ink">we don't.</strong>
          </p>
          <p>use your own usn. stalking on someone else's is weird without their permission. don't be that guy.</p>
        </div>
      </footer>
    </>
  );
}
