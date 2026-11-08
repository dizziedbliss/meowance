import { dayMonth } from "../lib/dates";
import type { Attendance } from "../lib/types";
import { classTag, courseName, room, todayName, trimTime } from "../lib/week";

const tone = { dead: "text-dead", fine: "text-fine", safe: "text-safe", mut: "text-mut" } as const;

/** This week, day by day, every class tagged by whether that subject can afford a bunk. */
export function Timetable({ data }: { data: Attendance }) {
  const days = data.timetable;
  const today = todayName();
  const total = days?.reduce((n, d) => n + d.classes.length, 0) ?? 0;

  return (
    <section className="pt-4 md:pt-8">
      <div className="flex flex-wrap items-end justify-between gap-x-6 gap-y-3 border-b border-hair-2 pb-5">
        <div>
          <h2 className="numeral text-[clamp(64px,11vw,128px)] leading-[.8] tracking-[-.03em] [word-spacing:.12em]">
            your week<span className="text-orange">.</span>
          </h2>
          {days && days.length > 0 && (
            <p className="mt-4 font-mono text-xs text-mut">
              {dayMonth(days[0].date)} – {dayMonth(days[days.length - 1].date)} · {total} classes · straight from the portal
            </p>
          )}
        </div>
        <div aria-label="Tag key" className="flex flex-wrap gap-4 font-mono text-[11px]">
          <span className="text-safe">bunkable</span>
          <span className="text-fine">show up</span>
          <span className="text-dead">don't you dare</span>
        </div>
      </div>

      {days === null || days === undefined ? (
        <Empty line="couldn't grab your timetable this time. the portal's being the portal. hit sync." />
      ) : days.length === 0 ? (
        <Empty line="no classes this week. suspicious. enjoy it." />
      ) : (
        <>
          {!days.some((d) => d.weekday === today) && (
            <p className="mt-6 font-display text-xl font-bold text-ink-2">no classes today. go outside, it's free.</p>
          )}
          {days.map((d) => {
            const isToday = d.weekday === today;
            return (
              <div key={d.date} className="grid gap-x-8 border-b border-hair py-6 md:grid-cols-[150px_minmax(0,1fr)]">
                <div className="mb-3 flex items-baseline gap-3 md:mb-0 md:block">
                  <div className={`numeral text-[44px] leading-[.85] tracking-[-.02em] lowercase ${isToday ? "text-orange" : ""}`}>
                    {d.weekday.slice(0, 3)}
                  </div>
                  <div className="font-mono text-xs text-mut md:mt-2">
                    {dayMonth(d.date)}
                    {isToday && <b className="ml-2 font-medium text-orange">today</b>}
                  </div>
                </div>
                <ul>
                  {d.classes.map((c, i) => {
                    const tag = classTag(c, data.courses);
                    return (
                      <li
                        key={`${c.start}-${c.code}-${i}`}
                        className="grid grid-cols-[minmax(0,1fr)_auto] gap-x-4 gap-y-1 border-t border-hair py-3 first:border-t-0 first:pt-0 md:grid-cols-[120px_minmax(0,1fr)_auto] md:items-center"
                      >
                        <span className="font-mono text-sm text-ink-2">
                          {trimTime(c.start)}–{trimTime(c.end)}
                        </span>
                        <span className="col-span-full row-start-2 md:col-span-1 md:row-start-auto">
                          <span className="block font-semibold leading-snug">{courseName(c, data.courses)}</span>
                          <span className="font-mono text-xs text-mut">
                            {c.code.toLowerCase()} · {room(c.room)}
                            {c.batch && ` · ${c.batch.toLowerCase()}`}
                          </span>
                        </span>
                        <span className={`text-right font-mono text-xs ${tag ? tone[tag.tone] : "text-mut"}`}>
                          {tag?.text ?? "—"}
                        </span>
                      </li>
                    );
                  })}
                </ul>
              </div>
            );
          })}
          <p className="mt-6 max-w-[60ch] text-[13.5px] text-mut">
            tags come from each subject's attendance right now. "bunkable" means that subject has lives left, not that
            every one of its classes is free. spend them wisely.
          </p>
        </>
      )}
    </section>
  );
}

function Empty({ line }: { line: string }) {
  return <p className="mt-8 max-w-[44ch] font-display text-2xl font-bold leading-tight text-ink-2">{line}</p>;
}
