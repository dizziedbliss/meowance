import type { Course } from "../lib/types";
import type { Zone } from "../lib/verdict";

type Props = { course: Course; zone: Zone; onSlab?: boolean; pop?: boolean; className?: string };

/** One square per class: filled = attended, hollow = bunked, orange outline = classes still owed. */
export function Tally({ course: c, zone, onSlab, pop, className = "" }: Props) {
  const sq: React.ReactNode[] = [];
  let k = 0;
  for (let i = 0; i < c.attended; i++) sq.push(<i key={`a${i}`} className="at" />);
  for (let i = 0; i < c.conducted - c.attended; i++) sq.push(<i key={`m${i}`} className="ms" />);

  const owe = (n: number, label: string, prefix: string) => {
    for (let i = 0; i < n; i++) sq.push(<i key={`${prefix}${i}`} className="nd" style={{ "--k": k++ } as React.CSSProperties} />);
    sq.push(<b key={`${prefix}l`}>{label}</b>);
  };

  if (zone === "dead") {
    sq.push(<span key="g" className="gap" />);
    owe(c.need75, "75", "n");
    owe(c.need85 - c.need75, "85", "f");
  } else if (zone === "fine") {
    sq.push(<span key="g" className="gap" />);
    owe(c.need85, "85", "f");
  } else if (c.skip85 > 0) {
    sq.push(<span key="g" className="gap" />);
    for (let i = 0; i < c.skip85; i++) sq.push(<i key={`s${i}`} className="sk" />);
    sq.push(<b key="sl">{c.skip85 === 1 ? "life" : "lives"}</b>);
  }

  const label = `${c.attended} attended, ${c.conducted - c.attended} bunked` +
    (zone === "dead" ? `, ${c.need75} more to reach 75, ${c.need85} to reach 85` : zone === "fine" ? `, ${c.need85} more to reach 85` : "");

  return (
    <div role="img" aria-label={label} className={`tally ${onSlab ? "on-slab" : ""} ${pop ? "pop" : ""} ${className}`}>
      {sq}
    </div>
  );
}
