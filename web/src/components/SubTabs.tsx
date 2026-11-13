/** Labels split by hairline dividers, like the bottom nav: one view at a time, no endless scroll. */
export function SubTabs<T extends string>({
  label,
  tabs,
  active,
  onTab,
  className = "",
}: {
  label: string;
  tabs: { id: T; text: string }[];
  active: T;
  onTab: (t: T) => void;
  className?: string;
}) {
  return (
    <div role="tablist" aria-label={label} className={`flex flex-wrap items-center gap-y-1 font-mono text-xs ${className}`}>
      {tabs.map((t, i) => (
        <span key={t.id} className="flex items-center">
          {i > 0 && <span className="h-3 w-px flex-none bg-hair-2" />}
          <button
            type="button"
            role="tab"
            aria-selected={active === t.id}
            onClick={() => onTab(t.id)}
            className={`py-1.5 whitespace-nowrap ${i === 0 ? "pr-3" : "px-3"} ${active === t.id ? "text-orange" : "text-ink-2 hover:text-ink"}`}
          >
            {t.text}
          </button>
        </span>
      ))}
    </div>
  );
}
