import type { Head } from "../lib/verdict";

/** Renders a verdict headline with its one number highlighted. */
export function Headline({ head, numberClass }: { head: Head; numberClass: string }) {
  return (
    <>
      {head.before}
      {head.n !== null && <em className={`not-italic ${numberClass}`}>{head.n}</em>}
      {head.after}
    </>
  );
}
