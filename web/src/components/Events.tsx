import { isoDayMonth, todayISO } from "../lib/dates";
import { countdown, daysLeftLine, eventTag, rangeLabel, splitEvents, workingDaysLeft } from "../lib/events";
import type { Attendance, CalendarEvent } from "../lib/types";

const tone = { safe: "text-safe", dead: "text-dead", orange: "text-orange" } as const;

/** The semester's calendar of events: what's next, what's coming, what already happened. */
export function Events({ data }: { data: Attendance }) {
  const cal = data.calendar;
  const today = todayISO();

  if (!cal) {
    return (
      <section className="pt-4 md:pt-8">
        <h2 className="numeral text-[clamp(64px,11vw,128px)] leading-[.8] tracking-[-.03em]">
          events<span className="text-orange">.</span>
        </h2>
        <p className="mt-8 max-w-[44ch] font-display text-2xl font-bold leading-tight text-ink-2">
          couldn't grab the calendar this time. the portal's being the portal. hit sync.
        </p>
      </section>
    );
  }

  const left = workingDaysLeft(cal, today < cal.start ? cal.start : today);
  const { upcoming, past } = splitEvents(cal, today);
  const [next, ...rest] = upcoming;

  return (
    <section className="pt-4 md:pt-8">
      <div className="flex flex-wrap items-end justify-between gap-x-6 gap-y-3 border-b border-hair-2 pb-5">
        <div>
          <h2 className="numeral text-[clamp(64px,11vw,128px)] leading-[.8] tracking-[-.03em]">
            events<span className="text-orange">.</span>
          </h2>
          <p className="mt-4 font-mono text-xs text-mut">
            sem runs {isoDayMonth(cal.start)} – {isoDayMonth(cal.end)} · straight from the portal
          </p>
        </div>
        <div className="md:text-right">
          <div className="numeral text-[56px] leading-[.8] tracking-[-.02em] text-orange">{left}</div>
          <div className="mt-1.5 font-mono text-xs text-mut">working days left · {daysLeftLine(left)}</div>
        </div>
      </div>

      {next ? (
        <div className="border-b border-hair py-8">
          <div className="font-mono text-xs text-mut">next up · {countdown(next, today)}</div>
          <p className="mt-2 font-display text-[clamp(34px,5vw,56px)] font-extrabold leading-[.98] tracking-[-.025em] text-balance">
            {next.title.toLowerCase()}
          </p>
          <div className="mt-3 flex flex-wrap gap-x-5 font-mono text-sm">
            <span className="text-ink-2">{rangeLabel(next)}</span>
            <Tag e={next} />
          </div>
        </div>
      ) : (
        <p className="mt-8 font-display text-2xl font-bold text-ink-2">nothing left on the calendar. you made it.</p>
      )}

      {rest.length > 0 && (
        <>
          <div className="pt-8 pb-1 font-mono text-xs text-mut">coming up</div>
          <ul>
            {rest.map((e) => (
              <Row key={`${e.from}-${e.title}`} e={e} today={today} />
            ))}
          </ul>
        </>
      )}

      {past.length > 0 && (
        <details className="group mt-8">
          <summary className="cursor-pointer list-none font-mono text-xs text-mut hover:text-ink-2">
            <span className="group-open:hidden">+ already happened ({past.length})</span>
            <span className="hidden group-open:inline">− already happened ({past.length})</span>
          </summary>
          <ul className="mt-2 opacity-60">
            {past.map((e) => (
              <Row key={`${e.from}-${e.title}`} e={e} />
            ))}
          </ul>
        </details>
      )}
    </section>
  );
}

function Tag({ e }: { e: CalendarEvent }) {
  const t = eventTag(e);
  return t ? <span className={tone[t.tone]}>{t.text}</span> : null;
}

function Row({ e, today }: { e: CalendarEvent; today?: string }) {
  return (
    <li className="grid grid-cols-[minmax(0,1fr)_auto] gap-x-4 gap-y-1 border-b border-hair py-3.5 md:grid-cols-[130px_minmax(0,1fr)_auto] md:items-center">
      <span className="font-mono text-sm text-ink-2">{rangeLabel(e)}</span>
      <span className="col-span-full row-start-2 font-semibold leading-snug md:col-span-1 md:row-start-auto">
        {e.title.toLowerCase()}
      </span>
      <span className="text-right font-mono text-xs">
        <Tag e={e} />
        {today && <span className="ml-3 text-mut">{countdown(e, today)}</span>}
      </span>
    </li>
  );
}
