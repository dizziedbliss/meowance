import type { Attendance } from "../lib/types";
import { byPercent } from "../lib/verdict";
import { CourseRow } from "./CourseRow";
import { Slab } from "./Slab";

export function Dashboard({ data }: { data: Attendance }) {
  const [worst, ...rest] = byPercent(data.courses);
  return (
    <>
      {worst && <Slab course={worst} />}

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
            <CourseRow key={c.code} course={c} />
          ))}
        </>
      )}

      <footer className="mt-10 grid gap-1.5 md:grid-cols-[140px_minmax(0,1fr)] md:gap-5">
        <span className="font-mono text-xs text-mut">facts to know</span>
        <div className="max-w-[68ch] space-y-2 text-[13.5px] text-ink-2">
          <p>we're not mce. not affiliated, not sponsored, not even friends. just kids who got tired of doing this math by hand.</p>
          <p>
            we don't keep your data, never. usn and birthday go in, attendance come out, we forget you even exist. your attendance keeps
            the bill. <strong className="text-ink">we don't.</strong>
          </p>
          <p>use your own usn. stalking on someone else's is weird wihtout their permission. don't be that guy.</p>
        </div>
      </footer>
    </>
  );
}
