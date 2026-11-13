import { useEffect, useState } from "react";
import type { ClassRecord, Course } from "../lib/types";
import { fmtPct, predicted, verdict, zoneName } from "../lib/verdict";
import { Headline } from "./Headline";
import { toggleProps } from "./CourseRow";
import { Tally } from "./Tally";

const bg = { dead: "bg-dead", fine: "bg-fine", safe: "bg-safe" } as const;

/** Count up to the number once, unless the viewer prefers reduced motion. */
function useCountUp(target: number): number {
  const reduce = typeof matchMedia !== "undefined" && matchMedia("(prefers-reduced-motion: reduce)").matches;
  const [v, setV] = useState(reduce ? target : 0);
  useEffect(() => {
    if (reduce) return setV(target);
    let raf = 0;
    const t0 = performance.now();
    const step = (t: number) => {
      const p = Math.min(1, (t - t0) / 700);
      setV(target * (1 - Math.pow(1 - p, 3)));
      if (p < 1) raf = requestAnimationFrame(step);
    };
    raf = requestAnimationFrame(step);
    return () => cancelAnimationFrame(raf);
  }, [target, reduce]);
  return v;
}

/** Your worst subject, as a slab in its zone's color. */
type SlabProps = { course: Course; classes: ClassRecord[] | null; total: number | null; open: boolean; onToggle: () => void };

export function Slab({ course, classes, total, open, onToggle }: SlabProps) {
  const v = verdict(course);
  const guess = total ? predicted(course, total) : null; // an extra, never the main verdict
  const shown = useCountUp(course.percent);

  return (
    <section
      {...toggleProps(open, onToggle, `lowest: ${course.name}. ${open ? "hide" : "show"} every class`)}
      className={`${bg[v.zone]} text-on-slab grid cursor-pointer gap-y-6 px-[22px] pt-6 pb-[26px] md:grid-cols-[minmax(0,1.15fr)_minmax(0,.85fr)] md:gap-x-11 md:gap-y-1.5 md:px-[34px] md:pt-[30px] md:pb-8`}
    >
      <div className="col-span-full mb-1.5 flex items-center justify-between font-mono text-xs">
        <span>your problem child rn</span>
        <span className="border-[1.5px] border-current px-2.5 py-0.5 font-medium">{zoneName[v.zone]}</span>
      </div>

      <div>
        <div className="numeral -ml-[.04em] mt-2.5 text-[clamp(140px,24vw,300px)] leading-[.78] tracking-[-.025em]">
          {fmtPct(shown)}
          <small className="ml-1.5 text-[.24em] font-bold tracking-normal opacity-60">%</small>
        </div>
        <div className="mt-4 font-display text-[clamp(20px,2.3vw,27px)] font-bold leading-[1.12] tracking-[-.01em]">
          {course.name}
          <span className="mt-1.5 block font-mono text-xs font-normal tracking-normal opacity-70">
            {course.code.toLowerCase()} · pawsent {course.attended} of {course.conducted} · {open ? "hide ↑" : "when did i bunk ↓"}
          </span>
        </div>
      </div>

      <div className="self-end pb-1">
        <p className="mb-2.5 font-display text-[clamp(34px,4.3vw,52px)] font-extrabold leading-[.96] tracking-[-.025em] text-balance">
          <Headline head={v.head} numberClass="text-cream" />
        </p>
        <p className="max-w-[34ch] text-base opacity-85">{v.sub}</p>
        {v.then && (
          <div className="mt-5 border-t-[1.5px] border-on-slab/25 pt-4">
            <p className="mb-1 font-display text-[clamp(24px,2.7vw,32px)] font-extrabold leading-[.96] tracking-[-.025em]">
              <Headline head={v.then.head} numberClass="text-cream" />
            </p>
            <p className="max-w-[34ch] text-base opacity-85">{v.then.sub}</p>
          </div>
        )}
      </div>

      <Tally course={course} zone={v.zone} classes={classes} onSlab pop className="col-span-full mt-5" />
      {guess && <p className="col-span-full -mt-4 font-mono text-[11px] opacity-70 md:mt-2">{guess.line}*</p>}
    </section>
  );
}
