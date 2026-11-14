import { useRef, useState } from "react";
import { checkFile, NotesError, notesErrorCopy, type Note, uploadNote } from "../lib/notes";
import { field } from "./field";

type Props = { code: string; defaultSem: string; onDone: (n: Note) => void };

/** The "add a note" form: a title, a semester, one file. The file goes straight to storage. */
export function AddNote({ code, defaultSem, onDone }: Props) {
  const [title, setTitle] = useState("");
  const [sem, setSem] = useState(defaultSem || "5");
  const [file, setFile] = useState<File | null>(null);
  const [fileErr, setFileErr] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const picker = useRef<HTMLInputElement>(null);

  function pick(f: File | null) {
    setFile(f);
    setFileErr(f ? checkFile(f) : null);
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!file || fileErr || !title.trim() || busy) return;
    setErr(null);
    setBusy("checking it's you…");
    try {
      const note = await uploadNote(file, { title: title.trim(), code, sem }, (f) => setBusy(`sending… ${Math.round(f * 100)}%`));
      onDone(note);
    } catch (x) {
      setErr(x instanceof NotesError ? notesErrorCopy[x.code] : notesErrorCopy.internal);
      setBusy(null);
    }
  }

  return (
    <form onSubmit={submit} className="grid gap-4 border-b border-hair py-5 md:max-w-[560px]" aria-label="Add a note">
      <div>
        <label htmlFor="note-title" className="mb-1.5 block font-mono text-xs text-ink-2">
          title · what is it?
        </label>
        <input
          id="note-title"
          value={title}
          maxLength={80}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="unit 3 handwritten, neat i swear"
          className={field}
          disabled={!!busy}
        />
      </div>
      <div className="flex flex-wrap gap-4">
        <div>
          <label htmlFor="note-sem" className="mb-1.5 block font-mono text-xs text-ink-2">
            sem
          </label>
          <select id="note-sem" value={sem} onChange={(e) => setSem(e.target.value)} className={`${field} w-24`} disabled={!!busy}>
            {[1, 2, 3, 4, 5, 6, 7, 8].map((n) => (
              <option key={n} value={String(n)}>
                {n}
              </option>
            ))}
          </select>
        </div>
        <div className="min-w-0 flex-1">
          <span className="mb-1.5 block font-mono text-xs text-ink-2">file · pdf, png or jpg, up to 25 mb</span>
          <input
            ref={picker}
            id="note-file"
            type="file"
            accept="application/pdf,image/png,image/jpeg"
            className="sr-only"
            onChange={(e) => pick(e.target.files?.[0] ?? null)}
            disabled={!!busy}
          />
          <button
            type="button"
            onClick={() => picker.current?.click()}
            disabled={!!busy}
            className={`${field} truncate text-left ${file ? "text-ink" : "text-[#8a6a4a]"}`}
            aria-describedby="note-file-help"
          >
            {file ? file.name : "choose a file…"}
          </button>
          <p id="note-file-help" role={fileErr ? "alert" : undefined} className={`mt-1.5 font-mono text-xs ${fileErr ? "text-dead" : "text-mut"}`}>
            {fileErr ?? "labels, not names: nobody sees who posted it."}
          </p>
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
        <button
          type="submit"
          disabled={!file || !!fileErr || !title.trim() || !!busy}
          className="border border-orange px-5 py-2.5 font-mono text-sm text-orange hover:bg-orange hover:text-on-slab disabled:cursor-not-allowed disabled:border-hair-2 disabled:text-mut disabled:hover:bg-transparent"
        >
          {busy ?? "post it"}
        </button>
        {err && (
          <p role="alert" className="font-mono text-xs text-dead">
            {err}
          </p>
        )}
      </div>
    </form>
  );
}
