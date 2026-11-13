import type { Extra } from "./cache";
import type { ApiErrorCode, Attendance } from "./types";

export class ApiError extends Error {
  constructor(public code: ApiErrorCode) {
    super(code);
  }
}

/** What to tell the student for each failure. */
export const errorCopy: Record<ApiErrorCode, string> = {
  bad_input: "that doesn't look like an mce usn + ddmmyyyy birthday. check it.",
  bad_login: "portal said nope. usn or birthday's wrong.",
  too_many_tries: "too many wrong birthdays for that usn. take 15 minutes, then try again.",
  portal_down: "the portal's down again. classic. try in a bit.",
  portal_changed: "the portal changed its look and we can't read it rn. we're on it.",
  internal: "something broke on our side. try again.",
  network: "you're offline, or we are. try again.",
  portal_blocked: "Oops, they're blocking us, please wait until we fix ts"
};

/** Logs in and returns the dashboard, plus the extras asked for (left out of the reply otherwise). */
export async function fetchAttendance(usn: string, dob: string, extras: Extra[] = []): Promise<Attendance> {
  let res: Response;
  try {
    res = await fetch("/api/attendance", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(extras.length ? { usn, dob, extras } : { usn, dob }),
    });
  } catch {
    throw new ApiError("network");
  }
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: ApiErrorCode };
    throw new ApiError(body.error && body.error in errorCopy ? body.error : "internal");
  }
  return (await res.json()) as Attendance;
}

// Same rules as the server, checked before we bother it.
export const usnOk = (s: string) => /^4MC\d{2}[A-Z]{2}\d{3}$/.test(s.trim().toUpperCase());
export function dobOk(s: string): boolean {
  const m = /^(\d{2})(\d{2})(\d{4})$/.exec(s.trim());
  if (!m) return false;
  const [d, mo, y] = [+m[1], +m[2], +m[3]];
  const date = new Date(Date.UTC(y, mo - 1, d));
  return y >= 1970 && date.getUTCMonth() === mo - 1 && date.getUTCDate() === d && date.getTime() < Date.now();
}

/** What to tell them when feedback doesn't go through. */
const feedbackCopy: Record<string, string> = {
  too_many_tries: "that's a lot of feedback. try again in an hour.",
  feedback_off: "the feedback box is napping. try again later.",
  bad_input: "that message is empty or over 1000 characters.",
};

/** Sends the "tell max" box to Max's inbox, with the student's usn. Throws an Error with the copy to show. */
export async function sendFeedback(usn: string, message: string): Promise<void> {
  let res: Response;
  try {
    res = await fetch("/api/feedback", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ usn, message }),
    });
  } catch {
    throw new Error("you're offline, or we are. try again.");
  }
  if (res.ok) return;
  const body = (await res.json().catch(() => ({}))) as { error?: string };
  throw new Error((body.error && feedbackCopy[body.error]) || "couldn't send it. try again in a bit.");
}
