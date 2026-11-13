import { isoDayMonth } from "../lib/dates";
import type { CourseHistory } from "../lib/types";

/** Under a tapped course: when you bunked, who teaches it, and how far behind the portal is. */
export function HistoryPanel({ history, loading }: { history: CourseHistory | null | undefined; loading: boolean }) {
  if (!history) {
    return (
      <p className="pb-5 font-mono text-xs text-mut">
        {history === undefined && loading ? "asking the portal for the receipts…" : "couldn't grab this one. the portal's being the portal. hit sync."}
      </p>
    );
  }
  const bunked = history.classes.filter((c) => !c.present);
  const last = history.classes[history.classes.length - 1];

  return (
    <div className="space-y-1.5 pb-5">
      <p className="text-sm text-ink-2">
        <span className="font-mono text-xs text-mut">bunked on </span>
        {bunked.length === 0 ? "never. respect." : bunked.map((c) => `${isoDayMonth(c.date)} (${c.start})`).join(" · ")}
      </p>
      <p className="font-mono text-xs text-mut">
        {history.faculty && <>taught by {history.faculty.toLowerCase()} · </>}
        {last ? `the portal's last entry is ${isoDayMonth(last.date)}, anything after isn't counted yet` : "nothing entered on the portal yet"}
      </p>
    </div>
  );
}
