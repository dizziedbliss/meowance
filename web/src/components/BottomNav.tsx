export type Tab = "timetable" | "scores" | "events";

const tabs: Tab[] = ["timetable", "scores", "events"];

/** Floating over the page on its own solid bar, so it never blends into what's under it. */
export function BottomNav({ active, onTab, onLogout }: { active: Tab | null; onTab: (t: Tab) => void; onLogout: () => void }) {
  const btn = "whitespace-nowrap px-3.5 py-3 text-[15px] font-medium md:px-6 md:text-base";
  return (
    <nav
      aria-label="Sections"
      className="fixed left-1/2 z-30 flex -translate-x-1/2 items-center border border-brown-line bg-brown px-1 shadow-[0_10px_30px_rgba(0,0,0,.55)]"
      style={{ bottom: "calc(22px + env(safe-area-inset-bottom, 0px))" }}
    >
      {tabs.map((t) => (
        <span key={t} className="flex items-center">
          <button
            type="button"
            aria-current={active === t ? "page" : undefined}
            onClick={() => onTab(t)}
            className={`${btn} ${active === t ? "text-orange" : "text-ink hover:text-orange-hi"}`}
          >
            {t}
          </button>
          <span className="h-4 w-px flex-none bg-brown-line" />
        </span>
      ))}
      <button type="button" onClick={onLogout} className={`${btn} text-ink-2 hover:text-ink`}>
        logout
      </button>
    </nav>
  );
}
