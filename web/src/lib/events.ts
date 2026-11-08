import { daysBetween, isoDayMonth } from "./dates";
import type { Calendar, CalendarEvent } from "./types";

export type EventTag = { text: string; tone: "safe" | "dead" | "orange" };

export function eventTag(e: CalendarEvent): EventTag | null {
  switch (e.kind) {
    case "holiday":
      return { text: "free real estate", tone: "safe" };
    case "exam":
      return { text: "lock in", tone: "dead" };
    case "lastday":
      return { text: "last day", tone: "orange" };
    default:
      return null;
  }
}

/** Working days from today to the end of the sem, inclusive: not Sundays, not marked holidays. */
export function workingDaysLeft(cal: Calendar, today: string): number {
  const off = new Set(cal.holidays);
  let n = 0;
  for (let d = today; d <= cal.end; d = addDay(d)) {
    if (!off.has(d) && new Date(`${d}T00:00:00Z`).getUTCDay() !== 0) n++;
  }
  return n;
}

function addDay(d: string): string {
  return new Date(Date.parse(`${d}T00:00:00Z`) + 86_400_000).toISOString().slice(0, 10);
}

/** Upcoming (soonest first, including ones happening now) and past (latest first). */
export function splitEvents(cal: Calendar, today: string): { upcoming: CalendarEvent[]; past: CalendarEvent[] } {
  const upcoming = cal.events.filter((e) => e.to >= today).sort((a, b) => a.from.localeCompare(b.from));
  const past = cal.events.filter((e) => e.to < today).sort((a, b) => b.from.localeCompare(a.from));
  return { upcoming, past };
}

export function countdown(e: CalendarEvent, today: string): string {
  if (e.from <= today && today <= e.to) return "happening now";
  const n = daysBetween(today, e.from);
  return n === 1 ? "tomorrow" : `in ${n} days`;
}

/** "12 nov" or "12–14 nov" or "30 oct – 2 nov". */
export function rangeLabel(e: CalendarEvent): string {
  if (e.from === e.to) return isoDayMonth(e.from);
  const [a, b] = [isoDayMonth(e.from), isoDayMonth(e.to)];
  const [ad, am] = a.split(" ");
  const [bd, bm] = b.split(" ");
  return am === bm ? `${ad}–${bd} ${am}` : `${a} – ${b}`;
}

export function daysLeftLine(n: number): string {
  if (n === 0) return "sem's over. go home.";
  if (n <= 10) return "home stretch. lock in.";
  return "plenty of time to mess up.";
}
