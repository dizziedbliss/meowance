/** The source code, pinned on the login and home screens: contributions are open. */
export const REPO_URL = "https://github.com/dizziedbliss/meowance";

export function Contribute({ className = "" }: { className?: string }) {
  return (
    <section aria-label="Contribute here" className={`grid gap-1.5 border-t border-hair pt-5 md:grid-cols-[140px_minmax(0,1fr)] md:gap-5 ${className}`}>
      <span className="font-mono text-xs text-mut">contribute here</span>
      <div className="max-w-[68ch]">
        <a
          href={REPO_URL}
          target="_blank"
          rel="noreferrer"
          className="font-display text-xl font-extrabold tracking-[-.01em] text-orange underline-offset-4 hover:underline"
        >
          github.com/dizziedbliss/meowance{"\u00a0"}↗
        </a>
        <p className="mt-1 text-[13.5px] text-ink-2">it's all open source heh. fix a bug, add a thing, open a pr n max reviews it.</p>
      </div>
    </section>
  );
}
