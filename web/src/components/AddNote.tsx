import { useEffect, useRef, useState } from "react";
import { checkFile, NotesError, notesErrorCopy, type Note, uploadLine, type UploadStage, uploadNote } from "../lib/notes";
import { field } from "./field";

type Props = { code: string; other: boolean; onDone: (n: Note) => void };

/** The "drop a note" form: a title and one file, posted to the course that's open. The file goes straight to storage. */
export function AddNote({ code, other, onDone }: Props) {
  const [title, setTitle] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [fileErr, setFileErr] = useState<string | null>(null);
  const [stage, setStage] = useState<UploadStage | null>(null); // null = not uploading
  const [tick, setTick] = useState(0);
  const [err, setErr] = useState<string | null>(null);
  const picker = useRef<HTMLInputElement>(null);
  const busy = stage !== null;
  const ready = !!file && !fileErr && !!title.trim();

  // like the login button: while it checks it's you, a new line every 1.6s (the last one stays)
  useEffect(() => {
    if (stage?.stage !== "check") return setTick(0);
    const id = setInterval(() => setTick((t) => t + 1), 1600);
    return () => clearInterval(id);
  }, [stage?.stage]);

  function pick(f: File | null) {
    setFile(f);
    setFileErr(f ? checkFile(f) : null);
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!file || !ready || busy) return;
    setErr(null);
    setStage({ stage: "check" });
    try {
      onDone(await uploadNote(file, { title: title.trim(), code }, setStage));
    } catch (x) {
      setErr(notesErrorCopy[x instanceof NotesError ? x.code : "internal"]);
      setStage(null);
    }
  }

  return (
    <form onSubmit={submit} className="grid gap-4 border-b border-hair py-5 md:max-w-[560px]" aria-label="Drop a note">
      <p className="font-mono text-xs text-ink-2">
        dropping this in <b className="font-medium text-orange">{code.toLowerCase()}</b>
        {other && <span className="text-mut"> · diff sem or branch? totally fine, it goes under that course code</span>}
      </p>
      <div>
        <label htmlFor="note-title" className="mb-1.5 block font-mono text-xs text-ink-2">
          what's it called
        </label>
        <input id="note-title" value={title} maxLength={80} onChange={(e) => setTitle(e.target.value)} placeholder="unit 3 handwritten, neat i swear" className={field} disabled={busy} />
      </div>
      <div>
        <span className="mb-1.5 block font-mono text-xs text-ink-2">the file · pdf, png or jpg, 25 mb max</span>
        <input
          ref={picker}
          id="note-file"
          type="file"
          accept="application/pdf,image/png,image/jpeg"
          className="sr-only"
          onChange={(e) => pick(e.target.files?.[0] ?? null)}
          disabled={busy}
        />
        <button
          type="button"
          onClick={() => picker.current?.click()}
          disabled={busy}
          className={`${field} truncate text-left ${file ? "text-ink" : "text-[#8a6a4a]"}`}
          aria-describedby="note-file-help"
        >
          {file ? file.name : "pick a file…"}
        </button>
        <p id="note-file-help" role={fileErr ? "alert" : undefined} className={`mt-1.5 font-mono text-xs ${fileErr ? "text-dead" : "text-mut"}`}>
          {fileErr ?? "ur name stays hidden, u just show up as a random label lol"}
        </p>
      </div>
      <div>
        <button
          type="submit"
          disabled={!ready || busy}
          aria-busy={busy}
          className={`group relative flex w-full items-center justify-between gap-3 overflow-hidden px-5 py-3.5 font-display text-lg font-extrabold tracking-[-.01em] transition-colors md:w-64 ${
            busy
              ? "busy cursor-wait bg-orange text-[#140b00]"
              : ready
                ? "bg-orange text-[#140b00] hover:bg-orange-hi"
                : "cursor-not-allowed bg-hair text-mut"
          }`}
        >
          <span aria-live="polite">{stage ? uploadLine(stage, tick) : "post it"}</span>
          <span className={`font-mono font-medium transition-transform ${busy ? "nudge" : ready ? "group-hover:translate-x-1.5" : ""}`}>→</span>
          {busy && <i aria-hidden className="sweep" />}
        </button>
        <p role="alert" className="mt-2 min-h-4 font-mono text-xs text-dead">
          {err}
        </p>
      </div>
    </form>
  );
}
