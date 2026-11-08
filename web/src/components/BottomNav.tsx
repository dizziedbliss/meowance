export type Tab = "timetable" | "scores" | "events";

const tabs: Tab[] = ["timetable", "scores", "events"];

/** Floating over the page: no background, just labels split by dividers. */
export function BottomNav({ active, onTab, onLogout }: { active: Tab | null; onTab: (t: Tab) => void; onLogout: () => void }) {
  const btn = "whitespace-nowrap px-3 py-1.5 text-[13.5px] font-medium md:px-5 md:text-[14.5px]";
  return (
    <nav
      aria-label="Sections"
      className="fixed left-1/2 z-30 flex -translate-x-1/2 items-center"
      style={{ bottom: "calc(22px + env(safe-area-inset-bottom, 0px))" }}
    >
      {tabs.map((t) => (
        <span key={t} className="flex items-center">
          <button
            type="button"
            aria-current={active === t ? "page" : undefined}
            onClick={() => onTab(t)}
            className={`${btn} ${active === t ? "text-orange" : "text-ink-2 hover:text-ink"}`}
          >
            {t}
          </button>
          <span className="h-3.5 w-px flex-none bg-hair-2" />
        </span>
      ))}
      <button type="button" onClick={onLogout} className={`${btn} text-ink-2 hover:text-ink`}>
        logout
      </button>
    </nav>
  );
}
