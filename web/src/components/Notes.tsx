import { useEffect, useRef, useState } from "react";
import { isoDayMonth, todayISO } from "../lib/dates";
import {
  type Note,
  deleteNote,
  forgetVote,
  listNotes,
  myNotes,
  noteFileUrl,
  normCode,
  NotesError,
  notesErrorCopy,
  rememberVote,
  report,
  sizeLabel,
  unvote,
  validCode,
  vote,
  votedIds,
} from "../lib/notes";
import type { Attendance } from "../lib/types";
import { AddNote } from "./AddNote";
import { field } from "./field";
import { SubTabs } from "./SubTabs";

/** The day a note went up, in the viewer's own timezone (a UTC date would show 00:00-05:30 IST as yesterday). */
const day = (iso: string) => isoDayMonth(todayISO(new Date(iso))).toLowerCase();
const kind = (mime: string) => (mime === "application/pdf" ? "pdf" : mime === "image/png" ? "png" : "jpg");
const failure = (x: unknown) => notesErrorCopy[x instanceof NotesError ? x.code : "internal"];

/** Notes other students shared, one course at a time. Posters show as funny labels, never names. */
export function Notes({ data }: { data: Attendance }) {
  const [tab, setTab] = useState(data.courses[0]?.code ?? "other");
  const [other, setOther] = useState("");
  const [list, setList] = useState<Note[] | null>(null);
  const [loadErr, setLoadErr] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [msg, setMsg] = useState<{ text: string; bad: boolean } | null>(null);
  const [voted, setVoted] = useState(() => new Set(votedIds()));
  const [mine, setMine] = useState<Set<string>>(new Set());
  const [deleting, setDeleting] = useState<string | null>(null);
  const [reporting, setReporting] = useState<string | null>(null);
  const [reason, setReason] = useState("");

  const code = normCode(tab === "other" ? other : tab);
  const ok = validCode(code);
  const shownCode = useRef(code); // the course on screen now, for uploads that finish after a switch
  shownCode.current = code;

  useEffect(() => {
    let live = true;
    setList(null);
    setLoadErr(null);
    setAdding(false);
    setReporting(null);
    setDeleting(null);
    setMine(new Set());
    if (ok) {
      listNotes(code)
        .then((n) => live && setList(n))
        .catch((x) => live && setLoadErr(failure(x)));
      // which are mine / which i upvoted: the server knows best (the browser's own list can be stale)
      myNotes(code).then((r) => {
        if (!live || !r) return;
        setMine(new Set(r.mine));
        setVoted(new Set(r.voted));
      });
    }
    return () => {
      live = false;
    };
  }, [code, ok]);

  async function upvote(n: Note) {
    setMsg(null);
    const had = voted.has(n.id);
    const bump = (by: number) => setList((l) => l && l.map((x) => (x.id === n.id ? { ...x, votes: x.votes + by } : x)));
    const flip = (on: boolean) =>
      setVoted((v) => {
        const c = new Set(v);
        if (on) c.add(n.id);
        else c.delete(n.id);
        return c;
      });
    // show it right away; undo if the server says no
    flip(!had);
    bump(had ? -1 : 1);
    try {
      if (had) {
        const removed = await unvote(n.id);
        forgetVote(n.id);
        if (!removed) bump(1); // there was no vote of ours to take back: the number was right all along
      } else {
        const counted = await vote(n.id);
        rememberVote(n.id);
        // a vote from an earlier login doesn't count twice: undo our +1
        if (!counted) bump(-1);
      }
    } catch (x) {
      flip(had);
      bump(had ? 1 : -1);
      setMsg({ text: failure(x), bad: true });
    }
  }

  async function remove(n: Note) {
    setMsg(null);
    try {
      await deleteNote(n.id);
      setList((l) => l && l.filter((x) => x.id !== n.id));
      setDeleting(null);
      setMsg({ text: "gone. poof.", bad: false });
    } catch (x) {
      setMsg({ text: failure(x), bad: true });
    }
  }

  async function sendReport(e: React.FormEvent, id: string) {
    e.preventDefault();
    if (!reason.trim()) return;
    try {
      await report(id, reason.trim());
      setMsg({ text: "sent. max will check it.", bad: false });
      setReporting(null);
      setReason("");
    } catch (x) {
      setMsg({ text: failure(x), bad: true });
    }
  }

  return (
    <section className="pt-4 md:pt-8">
      <div className="border-b border-hair-2 pb-5">
        <h2 className="numeral text-[clamp(64px,11vw,128px)] leading-[.8] tracking-[-.03em]">
          notes<span className="text-orange">.</span>
        </h2>
        <p className="mt-4 font-mono text-xs text-mut">Hello padakhu bacchon :")</p>
        <SubTabs
          label="Course"
          className="mt-4"
          tabs={[...data.courses.map((c) => ({ id: c.code, text: c.code.toLowerCase() })), { id: "other", text: "other course…" }]}
          active={tab}
          onTab={setTab}
        />
      </div>

      {tab === "other" && (
        <div className="border-b border-hair py-4">
          <p className="mb-3 max-w-[60ch] font-mono text-xs text-mut">looking for a diff sem or branch? type the course code. got notes for it? drop em, helps everyone heh</p>
          <label htmlFor="other-code" className="mb-1.5 block font-mono text-xs text-ink-2">
            course code · like 23ai501
          </label>
          <input id="other-code" value={other} maxLength={12} onChange={(e) => setOther(e.target.value)} placeholder="23ai501" className={`${field} w-48`} autoCapitalize="characters" />
        </div>
      )}

      {ok && (
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-hair py-3.5">
          <span className="font-mono text-xs text-mut">
            {list === null ? (loadErr ? "—" : "digging up notes…") : `${list.length} ${list.length === 1 ? "note" : "notes"}`} · {code.toLowerCase()}
          </span>
          <button type="button" onClick={() => setAdding((a) => !a)} className="font-mono text-xs text-orange underline-offset-4 hover:underline" aria-expanded={adding}>
            {adding ? "never mind" : "+ drop a note"}
          </button>
        </div>
      )}

      <p role="status" className={`font-mono text-xs ${msg ? `pt-3 ${msg.bad ? "text-dead" : "text-safe"}` : ""}`}>
        {msg?.text}
       </p>

      {adding && ok && (
        <AddNote
          code={code}
          other={tab === "other"}
          onDone={(n) => {
            setMsg({ text: "posted!! ur basically a topper now heh", bad: false });
            if (shownCode.current !== code) return; // switched courses mid-upload: it's in its own course, not this list
            setAdding(false);
            setList((l) => [n, ...(l ?? [])]);
            setMine((m) => new Set(m).add(n.id));
          }}
        />
      )}

      {!ok && tab === "other" && <p className="mt-6 font-display text-2xl font-bold text-ink-2">type the course code up there ↑</p>}
      {loadErr && (
        <p role="alert" className="mt-6 font-display text-2xl font-bold text-ink-2">
          {loadErr}
        </p>
      )}
      {list && list.length === 0 && !adding && <p className="mt-6 max-w-[44ch] font-display text-2xl font-bold leading-tight text-ink-2">nothing here yet. be the hero.</p>}

      <ul>
        {(list ?? []).map((n) => (
          <li key={n.id} className="grid grid-cols-[minmax(0,1fr)_auto] gap-x-4 gap-y-2 border-b border-hair py-4">
            <div className="min-w-0">
              <a href={noteFileUrl(n.id)} target="_blank" rel="noopener noreferrer" className="block break-words font-semibold leading-snug hover:text-orange">
                {n.title}
              </a>
              <div className="mt-1 font-mono text-xs text-mut">
                {n.label} · {kind(n.mime)} {sizeLabel(n.size)} · {day(n.created)}
              </div>
            </div>
            <div className="flex items-start gap-4 font-mono text-xs">
              <button
                type="button"
                onClick={() => upvote(n)}
                aria-pressed={voted.has(n.id)}
                aria-label={voted.has(n.id) ? `upvoted, ${n.votes} so far. tap to unvote` : `upvote, ${n.votes} so far`}
                className={voted.has(n.id) ? "text-orange" : "text-ink-2 hover:text-orange"}
              >
                ▲ {n.votes}
              </button>
              {mine.has(n.id) ? (
                <button type="button" onClick={() => setDeleting(deleting === n.id ? null : n.id)} className="text-mut hover:text-dead" aria-expanded={deleting === n.id}>
                  delete
                </button>
              ) : (
                <button type="button" onClick={() => setReporting(reporting === n.id ? null : n.id)} className="text-mut hover:text-ink" aria-expanded={reporting === n.id}>
                  report
                </button>
              )}
            </div>
            {deleting === n.id && (
              <div className="col-span-full flex flex-wrap items-center gap-4 font-mono text-xs">
                <span className="text-ink-2">delete it for good? no undo.</span>
                <button type="button" onClick={() => remove(n)} className="text-dead underline-offset-4 hover:underline">
                  yes, bin it
                </button>
                <button type="button" onClick={() => setDeleting(null)} className="text-mut hover:text-ink">
                  nah
                </button>
              </div>
            )}
            {reporting === n.id && (
              <form onSubmit={(e) => sendReport(e, n.id)} className="col-span-full flex flex-wrap gap-3">
                <label htmlFor={`why-${n.id}`} className="sr-only">
                  why are you reporting this?
                </label>
                <input id={`why-${n.id}`} value={reason} maxLength={300} onChange={(e) => setReason(e.target.value)} placeholder="what's wrong w it?" className={`${field} min-w-0 flex-1`} />
                <button type="submit" disabled={!reason.trim()} className="border border-hair-2 px-4 py-2.5 font-mono text-xs text-ink-2 hover:text-ink disabled:text-mut">
                  send
                </button>
              </form>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}
