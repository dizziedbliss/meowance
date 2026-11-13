import { useState } from "react";
import { posts } from "../data/board";
import { ApiError, dobOk, errorCopy, fetchAttendance, usnOk } from "../lib/api";
import { portalAsleep } from "../lib/dates";
import type { Attendance } from "../lib/types";
import { saveCreds, saveData } from "../lib/vault";
import { Logo } from "./Logo";

export function Login({ onIn }: { onIn: (a: Attendance) => void }) {
  const [usn, setUsn] = useState("");
  const [dob, setDob] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!usnOk(usn)) return setErr("that's not an mce usn. it looks like 4MC23CI000.");
    if (!dobOk(dob)) return setErr("birthday goes ddmmyyyy, like 15082004.");
    setErr(null);
    setBusy(true);
    try {
      const a = await fetchAttendance(usn.trim().toUpperCase(), dob.trim());
      await saveCreds({ usn: usn.trim().toUpperCase(), dob: dob.trim() });
      saveData(a);
      onIn(a);
    } catch (x) {
      setErr(errorCopy[x instanceof ApiError ? x.code : "internal"]);
    } finally {
      setBusy(false);
    }
  }

  const asleep = portalAsleep();

  const field =
    "w-full rounded-none border border-brown-line bg-brown px-4 py-[15px] font-mono text-[17px] tracking-[.04em] text-ink outline-none placeholder:text-[#8a6a4a] focus:border-orange focus:bg-brown-focus";

  return (
    <main>
      <div className="flex flex-wrap items-center justify-between gap-4">
        <Logo className="h-[50px] md:h-16" />
        <span className="flex items-center gap-2 font-mono text-xs text-ink-2">
          <i className="h-2 w-2 bg-orange" />
          mce kids only. no outsiders.
        </span>
      </div>

      <div className="mt-8 grid gap-12 md:grid-cols-[minmax(0,1.35fr)_minmax(0,.75fr)] md:gap-0">
        <div className="md:pr-14">
          <h1 className="numeral -ml-[.03em] mb-[22px] whitespace-nowrap text-[clamp(120px,24vw,300px)] leading-[.76] tracking-[-.03em] [font-stretch:64%]">
            bunk
            <span className="cat" aria-hidden="true">
              <span className="lid" />
            </span>
          </h1>
          <p className="mb-[34px] max-w-[26ch] font-display text-[clamp(22px,2.4vw,28px)] font-bold leading-[1.15] tracking-[-.01em]">
            how many classes you can <em className="not-italic text-orange">skip</em> before mce sends the <em className="not-italic text-orange">fine</em>? we did
            the math. you do the bunking.
          </p>

          {asleep && (
            <div className="mt-4 mb-6 max-w-[66ch] space-y-2 border border-orange px-5 py-2 text-[13.5px]">
              <em className="not-italic text-orange">NOTICE:</em>
              <p>
                Sources (or past experience) say that <em className="not-italic text-orange">after 12AM till somewhere around 6AM</em> MCE
                needs to sleep, so you can't access your data, nor can <em className="not-italic text-orange">we</em> do, uhmm u can test it
                tho, ur luck <span className="text-[10px]">(or maybe I'm too lazy to code smth to hide login at this hour)</span>.
              </p>
            </div>
          )}

          <form onSubmit={submit} noValidate className="max-w-[560px]">
            <div className="mb-3 grid gap-3 md:grid-cols-[1.2fr_1fr]">
              <div>
                <label htmlFor="usn" className="mb-2 block font-mono text-xs text-ink-2">usn</label>
                <input
                  id="usn"
                  value={usn}
                  onChange={(e) => setUsn(e.target.value.toUpperCase())}
                  placeholder="4MC23CI000"
                  autoComplete="username"
                  spellCheck={false}
                  maxLength={10}
                  className={field}
                />
              </div>
              <div>
                <label htmlFor="dob" className="mb-2 block font-mono text-xs text-ink-2">birthday · ddmmyyyy</label>
                <input
                  id="dob"
                  value={dob}
                  onChange={(e) => setDob(e.target.value.replace(/\D/g, ""))}
                  placeholder="15082004"
                  inputMode="numeric"
                  autoComplete="bday"
                  maxLength={8}
                  className={field}
                />
              </div>
            </div>
            <button
              type="submit"
              disabled={busy}
              className="group flex w-full items-center justify-between gap-3 bg-orange px-5 py-[17px] font-display text-xl font-extrabold tracking-[-.01em] text-[#140b00] transition-colors hover:bg-orange-hi disabled:cursor-wait disabled:opacity-80"
            >
              <span>{busy ? "pulling your receipts…" : "show me the damage"}</span>
              <span className="font-mono font-medium transition-transform group-hover:translate-x-1.5">→</span>
            </button>
            <p role="alert" className="mt-3 min-h-5 text-sm text-dead">{err}</p>
          </form>

          <div className="mt-4 max-w-[56ch] space-y-2 text-[13.5px] text-mut">
            <p>Your usn and birthday pass through our server once, then we forget you exist. no database, no receipts.</p>
            <p>Still scared to try? don't log in. </p>
            <p>Trying to use someone else's usn? weird. please don't stalk.</p>
          </div>
        </div>

        <aside aria-label="Community board" className="border-t border-hair-2 pt-[34px] md:border-t-0 md:border-l md:pt-0 md:pl-10">
          <h2 className="font-display text-[40px] font-black leading-[.9] tracking-[-.02em] [font-stretch:80%]">
            Changelogs
          </h2>
          <div className="mt-2.5 mb-7 font-mono text-xs text-mut">What happened??</div>
          {posts.map((p, i) => (
            <article key={p.title} className={`post ${i === 0 ? "new" : ""}`}>
              <div className="flex gap-2.5 font-mono text-[11px] text-mut">
                <span>{p.when}</span>
                {i === 0 && <b className="font-medium text-orange">new</b>}
              </div>
              <h3 className="mt-1 font-display text-[21px] font-extrabold leading-[1.15] tracking-[-.01em]">{p.title}</h3>
              <p className="mt-1 text-sm text-ink-2">{p.body}</p>
              <div className="mt-1 font-sign text-[21px] leading-none text-orange">{p.sign}</div>
            </article>
          ))}
        </aside>
      </div>
    </main>
  );
}
