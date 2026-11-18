import { isoDayMonth } from "../lib/dates";
import type { ClassRecord, Course } from "../lib/types";
import type { Zone } from "../lib/verdict";

type Props = { course: Course; zone: Zone; classes?: ClassRecord[] | null; left?: number | null; onSlab?: boolean; pop?: boolean; className?: string };

const tip = (c: ClassRecord) => `${c.present ? "pawsent" : "bunked"} · ${isoDayMonth(c.date)} · ${c.start}`;

/** One square per class: filled = attended, hollow = bunked, orange outline = classes still owed. */
export function Tally({ course: c, zone, classes, left = null, onSlab, pop, className = "" }: Props) {
  const sq: React.ReactNode[] = [];
  let k = 0;
  if (classes) {
    // the real order, each box dated (hover on pc; the row's receipts list the bunks on phones)
    for (const cl of classes) sq.push(<i key={cl.date + cl.start} className={cl.present ? "at" : "ms"} data-tip={tip(cl)} />);
  } else {
    for (let i = 0; i < c.attended; i++) sq.push(<i key={`a${i}`} className="at" />);
    for (let i = 0; i < c.conducted - c.attended; i++) sq.push(<i key={`m${i}`} className="ms" />);
  }

  // never more owed boxes than classes left; a flag only when its target can still be hit
  const room = (n: number) => (left === null ? n : Math.min(n, left));
  const owe = (n: number, label: string, prefix: string, flag = true) => {
    for (let i = 0; i < n; i++) sq.push(<i key={`${prefix}${i}`} className="nd" style={{ "--k": k++ } as React.CSSProperties} />);
    if (flag) sq.push(<b key={`${prefix}l`}>{label}</b>);
  };
  const can = (need: number) => left === null || need <= left;

  if (zone === "dead") {
    sq.push(<span key="g" className="gap" />);
    owe(room(c.need75), "75", "n", can(c.need75));
    owe(room(c.need85) - room(c.need75), "85", "f", can(c.need85));
  } else if (zone === "fine") {
    sq.push(<span key="g" className="gap" />);
    owe(room(c.need85), "85", "f", can(c.need85));
  } else if (c.skip85 > 0) {
    sq.push(<span key="g" className="gap" />);
    for (let i = 0; i < c.skip85; i++) sq.push(<i key={`s${i}`} className="sk" />);
    sq.push(<b key="sl">{c.skip85 === 1 ? "life" : "lives"}</b>);
  }

  const label = `${c.attended} attended, ${c.conducted - c.attended} bunked` +
    (zone !== "safe" && !can(zone === "dead" ? c.need75 : c.need85)
      ? `, only ${left} classes left, out of reach`
      : zone === "dead" ? `, ${c.need75} more to reach 75, ${c.need85} to reach 85` : zone === "fine" ? `, ${c.need85} more to reach 85` : "");

  return (
    <div role="img" aria-label={label} className={`tally ${onSlab ? "on-slab" : ""} ${pop ? "pop" : ""} ${className}`}>
      {sq}
    </div>
  );
}
