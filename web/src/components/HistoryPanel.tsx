import { isoDayMonth } from "../lib/dates";
import { bunksLeft } from "../lib/history";
import type { Course, CourseHistory } from "../lib/types";

const tone = { dead: "text-dead", fine: "text-fine", safe: "text-safe" } as const;

/** Under a tapped course: when you bunked, who teaches it, and how many more bunks the sem allows. */
export function HistoryPanel({ course, history, loading }: { course: Course; history: CourseHistory | null | undefined; loading: boolean }) {
  if (!history) {
    return (
      <p className="pb-5 font-mono text-xs text-mut">
        {history === undefined && loading ? "asking the portal for the receipts…" : "couldn't grab this one. the portal's being the portal. hit sync."}
      </p>
    );
  }
  const bunked = history.classes.filter((c) => !c.present);
  const advice = bunksLeft(course.attended, course.conducted, history.stillToGo);

  return (
    <div className="space-y-2 pb-5">
      <p className="text-sm text-ink-2">
        <span className="font-mono text-xs text-mut">bunked on </span>
        {bunked.length === 0 ? "never. respect." : bunked.map((c) => `${isoDayMonth(c.date)} (${c.start})`).join(" · ")}
      </p>
      {advice && (
        <p className={`font-display text-lg font-bold leading-snug ${tone[advice.zone]}`}>
          {advice.line} <span className="font-body text-sm font-normal text-mut">only if your faculty takes every class left lol</span>
        </p>
      )}
      {history.faculty && <p className="font-mono text-xs text-mut">taught by {history.faculty.toLowerCase()}</p>}
    </div>
  );
}
