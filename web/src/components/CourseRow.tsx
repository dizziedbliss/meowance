import type { ClassRecord, Course } from "../lib/types";
import { fmtPct, predicted, verdict, zoneName } from "../lib/verdict";
import { Headline } from "./Headline";
import { Tally } from "./Tally";

const color = { dead: "text-dead", fine: "text-fine", safe: "text-safe" } as const;

/** Tap (or Enter) toggles the course's class-by-class receipts below it. */
export const toggleProps = (open: boolean, onToggle: () => void, label: string) => ({
  role: "button",
  tabIndex: 0,
  "aria-expanded": open,
  "aria-label": label,
  onClick: onToggle,
  onKeyDown: (e: React.KeyboardEvent) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      onToggle();
    }
  },
});

type RowProps = { course: Course; classes: ClassRecord[] | null; total: number | null; open: boolean; onToggle: () => void };

export function CourseRow({ course, classes, total, open, onToggle }: RowProps) {
  const v = verdict(course);
  const guess = total ? predicted(course, total) : null; // an extra, never the main verdict
  return (
    <div
      {...toggleProps(open, onToggle, `${course.name}: ${open ? "hide" : "show"} every class`)}
      className={`grid cursor-pointer grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 gap-y-3.5 py-[22px] md:grid-cols-[minmax(0,1.25fr)_150px_minmax(0,1.25fr)] md:gap-x-8 md:py-[26px] ${open ? "" : "border-b border-hair"}`}
    >
      <div>
        <div className="text-lg font-semibold leading-tight">{course.name}</div>
        <div className="mt-1 font-mono text-xs text-mut">
          {course.code.toLowerCase()} · pawsent {course.attended} of {course.conducted} ·{" "}
          <span className="text-ink-2">{open ? "hide ↑" : "when did i bunk ↓"}</span>
        </div>
      </div>
      <div className={`numeral text-[60px] leading-[.8] tracking-[-.02em] md:text-[72px] ${color[v.zone]}`}>
        {fmtPct(course.percent)}
        <small className="ml-0.5 text-[.3em] font-semibold tracking-normal text-mut">%</small>
      </div>
      <div className="col-span-full md:col-span-1">
        <span className={`mb-1.5 inline-block font-mono text-[11px] ${color[v.zone]}`}>{zoneName[v.zone]}</span>
        <b className="block font-display text-[22px] font-extrabold leading-[1.15] tracking-[-.01em]">
          <Headline head={v.head} numberClass="text-orange" />
        </b>
        <span className="mt-1 block text-sm text-ink-2">{v.sub}</span>
      </div>
      <Tally course={course} zone={v.zone} classes={classes} className="col-span-full text-ink" />
      {guess && <p className="col-span-full -mt-1.5 font-mono text-[11px] text-mut">{guess.line}*</p>}
    </div>
  );
}
