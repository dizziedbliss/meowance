export type Tab = "timetable" | "scores" | "events";

const tabs: Tab[] = ["timetable", "scores", "events"];

/** Floating over the page on its own solid bar, so it never blends into what's under it. */
export function BottomNav({ active, onTab, onLogout }: { active: Tab | null; onTab: (t: Tab) => void; onLogout: () => void }) {
  const btn = "whitespace-nowrap px-3.5 py-3 text-[15px] font-medium focus-visible:outline-night md:px-6 md:text-base";
  return (
    <nav
      aria-label="Sections"
      className="fixed left-1/2 z-30 flex -translate-x-1/2 items-center bg-orange p-1 shadow-[0_10px_30px_rgba(0,0,0,.55)]"
      style={{ bottom: "calc(22px + env(safe-area-inset-bottom, 0px))" }}
    >
      {tabs.map((t) => (
        <span key={t} className="flex items-center">
          <button
            type="button"
            aria-current={active === t ? "page" : undefined}
            onClick={() => onTab(t)}
            className={`${btn} ${active === t ? "bg-night text-orange" : "text-on-slab hover:bg-on-slab/10"}`}
          >
            {t}
          </button>
          <span className="h-4 w-px flex-none bg-on-slab/25" />
        </span>
      ))}
      <button type="button" onClick={onLogout} className={`${btn} text-on-slab/70 hover:bg-on-slab/10 hover:text-on-slab`}>
        logout
      </button>
    </nav>
  );
}
