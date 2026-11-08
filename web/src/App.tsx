import { useEffect, useState } from "react";
import { BottomNav, type Tab } from "./components/BottomNav";
import { Dashboard } from "./components/Dashboard";
import { Events } from "./components/Events";
import { Login } from "./components/Login";
import { Logo } from "./components/Logo";
import { Scores } from "./components/Scores";
import { Timetable } from "./components/Timetable";
import { ApiError, errorCopy, fetchAttendance } from "./lib/api";
import { dayMonth } from "./lib/dates";
import type { Attendance } from "./lib/types";
import { loadCreds, loadData, saveData, wipe } from "./lib/vault";

/** The portal prints names in caps: "STUDENT" → "Student". */
const titleCase = (s: string) => s.toLowerCase().replace(/\b\p{L}/gu, (c) => c.toUpperCase());

export default function App() {
  const [data, setData] = useState<Attendance | null>(loadData);
  const [tab, setTab] = useState<Tab | null>(null); // null = the dashboard
  const [syncing, setSyncing] = useState(false);
  const [syncErr, setSyncErr] = useState<string | null>(null);

  useEffect(() => {
    document.body.classList.toggle("has-nav", data !== null);
  }, [data]);

  async function sync() {
    setSyncing(true);
    setSyncErr(null);
    try {
      const c = await loadCreds();
      if (!c) throw new Error("no creds");
      const a = await fetchAttendance(c.usn, c.dob);
      saveData(a);
      setData(a);
    } catch (x) {
      setSyncErr(x instanceof ApiError ? errorCopy[x.code] : "lost your login. log out and back in to sync.");
    } finally {
      setSyncing(false);
    }
  }

  async function logout() {
    await wipe();
    setData(null);
    setTab(null);
    setSyncErr(null);
  }

  return (
    <div className="mx-auto max-w-[1160px] px-5 pt-5 pb-[150px]" style={{ paddingLeft: "max(20px, env(safe-area-inset-left))", paddingRight: "max(20px, env(safe-area-inset-right))" }}>
      {!data ? (
        <Login onIn={(a) => setData(a)} />
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
            <Timetable data={data} />
          ) : tab === "scores" ? (
            <Scores data={data} />
          ) : tab === "events" ? (
            <Events data={data} />
          ) : (
            <Dashboard data={data} />
          )}
          <BottomNav active={tab} onTab={setTab} onLogout={logout} />
        </>
      )}
    </div>
  );
}
