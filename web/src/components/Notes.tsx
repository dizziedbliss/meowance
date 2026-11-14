import { useEffect, useState } from "react";
import {
  type Note,
  listNotes,
  noteFileUrl,
  normCode,
  NotesError,
  notesErrorCopy,
  rememberVote,
  report,
  sizeLabel,
  validCode,
  vote,
  votedIds,
} from "../lib/notes";
import type { Attendance } from "../lib/types";
import { AddNote } from "./AddNote";
import { field } from "./field";
import { SubTabs } from "./SubTabs";

const kind = (mime: string) => (mime === "application/pdf" ? "pdf" : mime === "image/png" ? "png" : "jpg");
const day = (iso: string) => new Date(iso).toLocaleDateString("en-GB", { day: "numeric", month: "short" }).toLowerCase();
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
  const [reporting, setReporting] = useState<string | null>(null);
  const [reason, setReason] = useState("");

  const code = normCode(tab === "other" ? other : tab);
  const ok = validCode(code);

  useEffect(() => {
    let live = true;
    setList(null);
    setLoadErr(null);
    setAdding(false);
    setReporting(null);
    if (ok) {
      listNotes(code)
        .then((n) => live && setList(n))
        .catch((x) => live && setLoadErr(failure(x)));
    }
    return () => {
      live = false;
    };
  }, [code, ok]);

  async function upvote(n: Note) {
    if (voted.has(n.id)) return;
    setMsg(null);
    setVoted((v) => new Set(v).add(n.id));
    setList((l) => l && l.map((x) => (x.id === n.id ? { ...x, votes: x.votes + 1 } : x)));
    try {
      await vote(n.id);
      rememberVote(n.id);
    } catch (x) {
      setVoted((v) => {
        const c = new Set(v);
        c.delete(n.id);
        return c;
      });
      setList((l) => l && l.map((y) => (y.id === n.id ? { ...y, votes: y.votes - 1 } : y)));
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
            setAdding(false);
            setList((l) => [n, ...(l ?? [])]);
            setMsg({ text: "posted!! ur basically a topper now heh", bad: false });
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
                disabled={voted.has(n.id)}
                aria-label={`upvote, ${n.votes} so far`}
                className={voted.has(n.id) ? "text-orange" : "text-ink-2 hover:text-orange"}
              >
                ▲ {n.votes}
              </button>
              <button type="button" onClick={() => setReporting(reporting === n.id ? null : n.id)} className="text-mut hover:text-ink" aria-expanded={reporting === n.id}>
                report
              </button>
            </div>
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
