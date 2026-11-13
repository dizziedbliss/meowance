import { shortDate } from "../lib/dates";
import { partLabel, scoreVerdict, type SeePart, seeMin } from "../lib/scores";
import type { Attendance } from "../lib/types";
import { PastSems } from "./PastSems";
import { SubTabs } from "./SubTabs";

export type ScoresView = "now" | "past";

const text = { dead: "text-dead", fine: "text-fine", safe: "text-safe" } as const;
const fill = { dead: "bg-dead", fine: "bg-fine", safe: "bg-safe" } as const;

type Props = { data: Attendance; view: ScoresView; onView: (v: ScoresView) => void; loading: boolean };

const seeTone = { done: "text-safe", open: "text-fine", gone: "text-dead" } as const;

/** One half of the SEE minimum, e.g. "cie 9/12 · 3 more from the 1 left". */
function seeLine(label: string, p: SeePart) {
  const tail =
    p.status === "done" ? "cleared" : p.status === "gone" ? "can't reach it anymore" : p.left === 0 ? "nothing left to score it in" : `${p.need - p.got} more from the ${p.left} left`;
  return (
    <span>
      {label} {p.got}/{p.need} · <span className={seeTone[p.status]}>{tail}</span>
    </span>
  );
}

/** This sem's CIE with the minimum to sit the SEE, or past semesters' results: one at a time. */
export function Scores({ data, view, onView, loading }: Props) {
  const rows = data.courses
    .map((c) => ({ c, v: scoreVerdict(c) }))
    // worst first; subjects with nothing marked go last
    .sort((a, b) => (a.v.zone === null ? 1 : 0) - (b.v.zone === null ? 1 : 0) || a.v.got / a.v.max - b.v.got / b.v.max);

  return (
    <section className="pt-4 md:pt-8">
      <div className="flex flex-wrap items-end justify-between gap-x-6 gap-y-3 border-b border-hair-2 pb-5">
        <div>
          <h2 className="numeral text-[clamp(64px,11vw,128px)] leading-[.8] tracking-[-.03em]">
            scores<span className="text-orange">.</span>
          </h2>
          <SubTabs
            label="Which scores"
            className="mt-4"
            tabs={[
              { id: "now", text: "this sem" },
              { id: "past", text: "past sems" },
            ]}
            active={view}
            onTab={onView}
          />
        </div>
        <div aria-label="Bar key" className={`flex flex-wrap gap-4 font-mono text-[11px] text-mut ${view === "now" ? "" : "invisible"}`}>
          <span className="inline-flex items-center gap-1.5"><i className="inline-block h-2.5 w-2.5 bg-ink" />got</span>
          <span className="inline-flex items-center gap-1.5"><i className="inline-block h-2.5 w-2.5 shadow-[inset_0_0_0_1.5px_var(--color-dead)]" />lost</span>
          <span className="inline-flex items-center gap-1.5"><i className="inline-block h-2.5 w-2.5 bg-hair-2" />still up for grabs</span>
        </div>
      </div>

      {view === "past" ? (
        <PastSems exams={data.exams} loading={loading} />
      ) : (
      <>
      <p className="mt-4 font-mono text-xs text-mut">cie so far, out of 50 each · see marks come later</p>
      {rows.map(({ c, v }) => {
        const min = seeMin(c, data.student.usn);
        return (

        <div
          key={c.code}
          className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 gap-y-3.5 border-b border-hair py-[22px] md:grid-cols-[minmax(0,1.25fr)_150px_minmax(0,1.25fr)] md:gap-x-8 md:py-[26px]"
        >
          <div>
            <div className="text-lg font-semibold leading-tight">{c.name}</div>
            <div className="mt-1 font-mono text-xs text-mut">{c.code.toLowerCase()}</div>
          </div>
          <div className={`numeral text-[60px] leading-[.8] tracking-[-.02em] md:text-[72px] ${v.zone ? text[v.zone] : "text-mut"}`}>
            {v.got}
            <small className="ml-1 text-[.3em] font-semibold tracking-normal text-mut">/{v.max}</small>
          </div>
          <div className="col-span-full md:col-span-1">
            <b className="block font-display text-[22px] font-extrabold leading-[1.15] tracking-[-.01em]">{v.head}</b>
            <span className="mt-1 block text-sm text-ink-2">{v.sub}</span>
            {min && (
              <span className="mt-2 block font-mono text-xs text-mut">
                to sit the see: {seeLine("cie", min.cie)} · {seeLine("activities", min.act)}
              </span>
            )}
          </div>

          <div className="col-span-full">
            {/* got / lost / still up for grabs, to scale out of the CIE max */}
            <div className="flex h-2.5" role="img" aria-label={`${v.got} got, ${v.lost} lost, ${v.left} still up for grabs, out of ${v.max}`}>
              <i className={v.zone ? fill[v.zone] : "bg-ink"} style={{ width: `${(v.got / v.max) * 100}%` }} />
              <i className="shadow-[inset_0_0_0_1.5px_var(--color-dead)] opacity-70" style={{ width: `${(v.lost / v.max) * 100}%` }} />
              <i className="bg-hair-2" style={{ width: `${(v.left / v.max) * 100}%` }} />
            </div>
            {(c.cie ?? []).length > 0 && (
              <ul className="mt-3 flex flex-wrap gap-x-6 gap-y-1.5 font-mono text-xs">
                {(c.cie ?? []).map((p) => (
                  <li key={p.name} className="text-ink-2">
                    <span className="text-ink">{partLabel(p.name)}</span>{" "}
                    {p.absent ? (
                      <span className="text-dead">ghosted</span>
                    ) : p.pending ? (
                      <span className="text-mut">not marked yet</span>
                    ) : (
                      <span>
                        {p.obtained}/{p.max}
                      </span>
                    )}
                    {p.entered && <span className="text-mut"> · {shortDate(p.entered)}</span>}
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
        );
      })}
      </>
      )}
    </section>
  );
}
