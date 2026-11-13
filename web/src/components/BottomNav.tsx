export type Tab = "timetable" | "scores" | "events";

const tabs: Tab[] = ["timetable", "scores", "events"];

/** Floating over the page with no bar of its own: labels split by hairlines over the dark fade (body.has-nav). */
export function BottomNav({ active, onTab, onLogout }: { active: Tab | null; onTab: (t: Tab) => void; onLogout: () => void }) {
  const btn =
    "whitespace-nowrap px-3.5 py-2 text-[15px] font-medium underline-offset-[7px] decoration-2 transition-colors md:px-6 md:text-base";
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
            className={`${btn} ${active === t ? "text-ink underline decoration-orange" : "text-ink-2 hover:text-ink"}`}
          >
            {t}
          </button>
          <span className="h-4 w-px flex-none bg-hair-2" />
        </span>
      ))}
      <button type="button" onClick={onLogout} className={`${btn} text-mut hover:text-ink`}>
        logout
      </button>
    </nav>
  );
}
