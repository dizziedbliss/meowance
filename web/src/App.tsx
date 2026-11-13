import { useEffect, useRef, useState } from "react";
import { BottomNav, type Tab } from "./components/BottomNav";
import { Dashboard } from "./components/Dashboard";
import { Events } from "./components/Events";
import { Login } from "./components/Login";
import { Logo } from "./components/Logo";
import { Scores, type ScoresView } from "./components/Scores";
import { Timetable } from "./components/Timetable";
import { ApiError, errorCopy, fetchAttendance } from "./lib/api";
import { type Extra, merge, stale, type Stored } from "./lib/cache";
import { dayMonth } from "./lib/dates";
import type { Attendance } from "./lib/types";
import { loadCreds, loadData, saveData, wipe } from "./lib/vault";

/** The portal prints names in caps: "STUDENT" → "Student". */
const titleCase = (s: string) => s.toLowerCase().replace(/\b\p{L}/gu, (c) => c.toUpperCase());

/** The page a tab needs besides the dashboard. */
/** The pages the screen on view needs besides the dashboard. */
function extrasFor(data: Attendance | null, tab: Tab | null, scores: ScoresView, open: string | null): Extra[] {
  if (!data) return [];
  const each = (kind: "history" | "cie") => data.courses.map((c) => `${kind}:${c.code}` as Extra);
  if (tab === "timetable") return ["timetable"];
  if (tab === "events") return ["calendar"];
  if (tab === "scores") return scores === "past" ? ["exams"] : each("cie");
  return open ? each("history") : []; // one course opened: fetch them all in one login, so the next is instant
}

export default function App() {
  const [store, setStore] = useState<Stored | null>(loadData);
  const storeRef = useRef(store); // the latest cache, for merges that finish after a re-render
  const data = store?.a ?? null;
  const [tab, setTab] = useState<Tab | null>(null); // null = the dashboard
  const [syncing, setSyncing] = useState(false);
  const [syncErr, setSyncErr] = useState<string | null>(null);
  const [scoresView, setScoresView] = useState<ScoresView>("now");
  const [open, setOpen] = useState<string | null>(null); // the dashboard course showing its receipts
  const tried = useRef(new Set<Extra>()); // auto-fetched on this view already: a failing page isn't retried in a loop

  useEffect(() => {
    document.body.classList.toggle("has-nav", data !== null);
  }, [data]);

  function keep(s: Stored | null) {
    storeRef.current = s;
    setStore(s);
  }

  /** Logs in again and refreshes the dashboard, plus these extras. */
  async function refresh(extras: Extra[]) {
    if (syncing) return;
    setSyncing(true);
    setSyncErr(null);
    try {
      const c = await loadCreds();
      if (!c) throw new Error("no creds");
      const a = await fetchAttendance(c.usn, c.dob, extras);
      const s = merge(storeRef.current, a, extras, Date.now());
      saveData(s);
      keep(s);
    } catch (x) {
      setSyncErr(x instanceof ApiError ? errorCopy[x.code] : "lost your login. log out and back in to sync.");
    } finally {
      setSyncing(false);
    }
  }

  const view = extrasFor(data, tab, scoresView, open);
  const viewKey = `${tab}|${scoresView}|${open}`;
  useEffect(() => {
    tried.current = new Set();
  }, [viewKey]);

  // A screen fetches its pages only when the cached copies are missing or stale,
  // including when a sync just changed what they reflect (new counts, new marks).
  useEffect(() => {
    if (syncing) return;
    const list = stale(storeRef.current, view, Date.now()).filter((e) => !tried.current.has(e));
    if (list.length === 0) return;
    list.forEach((e) => tried.current.add(e));
    void refresh(list);
    // view is derived from these three
  }, [viewKey, store?.at, syncing]);

  // Sync refreshes the dashboard and everything the screen on view shows.
  const sync = () => {
    view.forEach((e) => tried.current.add(e));
    void refresh(view);
  };

  async function logout() {
    await wipe();
    keep(null);
    setTab(null);
    setOpen(null);
    setScoresView("now");
    setSyncErr(null);
  }

  return (
    <div className="mx-auto max-w-[1160px] px-5 pt-5 pb-[150px]" style={{ paddingLeft: "max(20px, env(safe-area-inset-left))", paddingRight: "max(20px, env(safe-area-inset-right))" }}>
      {!data ? (
        <Login onIn={keep} />
      ) : (
        <>
          <header className="mb-[30px] flex flex-wrap items-center justify-between gap-4">
            <button type="button" onClick={() => setTab(null)} aria-label="meowance home">
              <Logo className="h-9 md:h-11" />
            </button>
            <div className="flex items-center gap-3.5 md:gap-5">
              <div className="text-right leading-tight">
                <div className="text-[13.5px] font-semibold md:text-[15px]">{titleCase(data.student.name) || "you"}</div>
                <div className="font-mono text-[11px] text-mut md:text-xs">
                  <b className="font-medium text-orange">{data.student.dept}</b> · {data.student.usn}
                </div>
              </div>
              <button
                type="button"
                onClick={sync}
                disabled={syncing}
                className="flex items-center gap-2 border-l border-hair-2 py-1.5 pl-3.5 font-mono text-xs text-ink-2 hover:text-ink disabled:cursor-wait md:pl-5"
              >
                <i className={`h-1.5 w-1.5 ${syncing ? "animate-pulse bg-orange" : "bg-safe"}`} />
                <span className="hidden sm:inline">
                  {syncing ? "syncing…" : `fresh as of ${dayMonth(data.student.lastUpdated)}`}
                </span>
                <span className="sm:hidden">{syncing ? "…" : "sync"}</span>
              </button>
            </div>
            {syncErr && <p role="alert" className="w-full text-right text-sm text-dead">{syncErr}</p>}
          </header>

          {tab === "timetable" ? (
            <Timetable data={data} loading={syncing} />
          ) : tab === "scores" ? (
            <Scores data={data} view={scoresView} onView={setScoresView} loading={syncing} />
          ) : tab === "events" ? (
            <Events data={data} loading={syncing} />
          ) : (
            <Dashboard data={data} open={open} onOpen={setOpen} loading={syncing} />
          )}
          <BottomNav active={tab} onTab={setTab} onLogout={logout} />
        </>
      )}
    </div>
  );
}
