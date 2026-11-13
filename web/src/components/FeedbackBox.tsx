import { useState } from "react";
import { sendFeedback } from "../lib/api";

const MAX = 1000;

/** "tell max": a message to Max's inbox, sent with the student's usn (and nothing else). */
export function FeedbackBox({ usn }: { usn: string }) {
  const [text, setText] = useState("");
  const [state, setState] = useState<"idle" | "sending" | "sent">("idle");
  const [err, setErr] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!text.trim() || state === "sending") return;
    setState("sending");
    setErr(null);
    try {
      await sendFeedback(usn, text.trim());
      setText("");
      setState("sent");
    } catch (x) {
      setErr(x instanceof Error ? x.message : "couldn't send it. try again in a bit.");
      setState("idle");
    }
  }

  return (
    <section aria-label="Tell max" className="mt-8 grid gap-1.5 md:grid-cols-[140px_minmax(0,1fr)] md:gap-5">
      <span className="font-mono text-xs text-mut">tell max</span>
      <form onSubmit={submit} className="max-w-[68ch]">
        <label htmlFor="feedback" className="sr-only">
          your feedback
        </label>
        <textarea
          id="feedback"
          value={text}
          maxLength={MAX}
          rows={3}
          onChange={(e) => {
            setText(e.target.value);
            if (state === "sent") setState("idle");
          }}
          placeholder="bugs, ideas, roasts. max reads all of it."
          className="block w-full resize-y rounded-none border border-brown-line bg-brown px-4 py-3 text-[14px] text-ink outline-none placeholder:text-[#8a6a4a] focus:border-orange focus:bg-brown-focus"
        />
        <div className="mt-2 flex flex-wrap items-center justify-between gap-3">
          <span className="font-mono text-[11px] text-mut">
            goes to max's inbox with your usn ({usn}). nothing else. · {MAX - text.length} left
          </span>
          <button
            type="submit"
            disabled={!text.trim() || state === "sending"}
            className="bg-orange px-4 py-2 font-display text-sm font-extrabold text-on-slab hover:bg-orange-hi disabled:cursor-not-allowed disabled:opacity-40"
          >
            {state === "sending" ? "sending…" : "send it"}
          </button>
        </div>
        {state === "sent" && <p className="mt-2 text-sm text-safe">sent. max will read it, probably at 2am.</p>}
        {err && (
          <p role="alert" className="mt-2 text-sm text-dead">
            {err}
          </p>
        )}
      </form>
    </section>
  );
}
