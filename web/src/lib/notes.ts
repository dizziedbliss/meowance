import type { ApiErrorCode } from "./types";
import { loadCreds, NOTES_TOKEN, NOTES_VOTED } from "./vault";

export type Note = { id: string; title: string; label: string; mime: string; size: number; votes: number; created: string };

/** The login and portal failures are the attendance ones; the rest are notes-only. */
export type NotesErrorCode =
  | ApiErrorCode
  | "bad_token"
  | "not_found"
  | "forbidden"
  | "rejected"
  | "too_many_uploads"
  | "notes_off"
  | "storage_error";

export class NotesError extends Error {
  constructor(public code: NotesErrorCode) {
    super(code);
  }
}

/** What to tell the student for each failure. Written like Max talks: u, ur, n, rn. */
export const notesErrorCopy: Record<NotesErrorCode, string> = {
  bad_input: "something's off with the title or the file. check it?",
  bad_login: "portal said nope. log out n back in, then try again.",
  too_many_tries: "too many wrong tries for that usn. chill for 15 min.",
  portal_down: "portal's asleep again, can't check it's u rn. try in a bit.",
  portal_changed: "portal changed its look n we can't read it rn. fixing it.",
  portal_blocked: "the portal's blocking us rn. try later.",
  internal: "we broke something. try again.",
  network: "ur offline, or we are. try again.",
  bad_token: "lost ur login. log out n back in, then try again.",
  not_found: "that note's gone. someone took it down, or it never made it.",
  forbidden: "not urs to touch.",
  rejected: "the upload didn't match what u said, so it got binned. try again.",
  too_many_uploads: "5 notes a day is the limit. go touch grass, come back tmrw.",
  notes_off: "notes are off rn. check back soon.",
  storage_error: "file storage hiccuped. try again in a min.",
};

export const MAX_FILE = 25 * 1024 * 1024;
const TYPES = ["application/pdf", "image/png", "image/jpeg"];

/** Why a file can't be uploaded, or null when it can. */
export function checkFile(f: { type: string; size: number }): string | null {
  if (!TYPES.includes(f.type)) return "pdf, png or jpg only bro.";
  if (f.size <= 0) return "that file's empty lol.";
  if (f.size > MAX_FILE) return "25 mb max. squish it or split it.";
  return null;
}

export function sizeLabel(bytes: number): string {
  if (bytes < 1024) return `${bytes} b`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} kb`;
  return `${Math.round((bytes / 1024 / 1024) * 10) / 10} mb`;
}

export const normCode = (s: string) => s.trim().toUpperCase();
export const validCode = (s: string) => /^[A-Z0-9]{3,12}$/.test(normCode(s));

// ---- the 7-day token: kept in this browser, wiped on logout ----

type Storage = Pick<globalThis.Storage, "getItem" | "setItem" | "removeItem">;
const HOUR = 3600 * 1000;

export function saveToken(s: Storage, token: string, expiresInSec: number, now: number): void {
  try {
    s.setItem(NOTES_TOKEN, JSON.stringify({ token, exp: now + expiresInSec * 1000 }));
  } catch {
    /* storage blocked: they'll just be asked to log in each time */
  }
}

/** The saved token, unless it ends within the hour (renew early rather than fail mid-upload). */
export function loadToken(s: Storage, now: number): string | null {
  try {
    const raw = s.getItem(NOTES_TOKEN);
    if (!raw) return null;
    const { token, exp } = JSON.parse(raw) as { token: string; exp: number };
    return typeof token === "string" && exp - HOUR > now ? token : null;
  } catch {
    return null;
  }
}

export function clearToken(s: Storage): void {
  try {
    s.removeItem(NOTES_TOKEN);
  } catch {
    /* ignore */
  }
}

const storage = (): Storage | null => {
  try {
    return globalThis.localStorage ?? null;
  } catch {
    return null;
  }
};

// ---- talking to /api/notes ----

async function call<T>(init: RequestInit, query = ""): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`/api/notes${query}`, init);
  } catch {
    throw new NotesError("network");
  }
  const body = (await res.json().catch(() => null)) as { error?: NotesErrorCode } | null;
  if (!res.ok) throw new NotesError(body?.error && body.error in notesErrorCopy ? body.error : "internal");
  return body as T;
}

const post = <T>(body: object, token?: string) =>
  call<T>({ method: "POST", headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) }, body: JSON.stringify(body) });

/** Checks the saved birthday on the portal and returns a fresh 7-day token. */
async function login(): Promise<string> {
  const c = await loadCreds();
  if (!c) throw new NotesError("bad_token");
  const r = await post<{ token: string; expiresIn: number }>({ action: "login", usn: c.usn, dob: c.dob });
  const s = storage();
  if (s) saveToken(s, r.token, r.expiresIn, Date.now());
  return r.token;
}

/** Runs a call that needs the token; if the server refuses the token, logs in again once and retries. */
async function withToken<T>(fn: (token: string) => Promise<T>): Promise<T> {
  const s = storage();
  const saved = s ? loadToken(s, Date.now()) : null;
  try {
    return await fn(saved ?? (await login()));
  } catch (e) {
    if (!(e instanceof NotesError) || e.code !== "bad_token" || !saved) throw e;
    if (s) clearToken(s);
    return fn(await login());
  }
}

export async function listNotes(code: string): Promise<Note[]> {
  return (await call<{ notes: Note[] }>({}, `?code=${encodeURIComponent(normCode(code))}`)).notes;
}

/** Where a note's file opens (the server redirects to wherever it's stored). */
export const noteFileUrl = (id: string) => `/api/notes?file=${encodeURIComponent(id)}`;

/** Upvotes a note. Resolves to whether it counted: false when this student had already voted. */
export function vote(id: string): Promise<boolean> {
  return withToken((t) => post<{ added: boolean }>({ action: "vote", noteId: id }, t)).then((r) => r.added);
}

/** Takes the upvote back. Resolves to whether there was one to take. */
export function unvote(id: string): Promise<boolean> {
  return withToken((t) => post<{ removed: boolean }>({ action: "unvote", noteId: id }, t)).then((r) => r.removed);
}

/** Deletes a note this student uploaded (the server refuses anyone else). */
export function deleteNote(id: string): Promise<void> {
  return withToken((t) => post<unknown>({ action: "delete", noteId: id }, t)).then(() => undefined);
}

/**
 * Which of a course's notes are this student's, and which they upvoted. Only asks when a login is
 * already saved (opening the page mustn't hit the portal); null means "don't know".
 */
export async function myNotes(code: string): Promise<{ mine: string[]; voted: string[] } | null> {
  const s = storage();
  const token = s ? loadToken(s, Date.now()) : null;
  if (!token) return null;
  try {
    return await post<{ mine: string[]; voted: string[] }>({ action: "mine", code: normCode(code) }, token);
  } catch {
    return null;
  }
}

export function report(id: string, reason: string): Promise<void> {
  return withToken((t) => post<unknown>({ action: "report", noteId: id, reason }, t)).then(() => undefined);
}

/** Sends the file straight to the store (it never passes through our server) and returns its key. */
function putFile(session: string, file: File, onProgress: (frac: number) => void): Promise<string> {
  return new Promise((resolve, reject) => {
    const x = new XMLHttpRequest();
    x.open("PUT", session);
    x.setRequestHeader("Content-Type", file.type);
    x.upload.onprogress = (e) => e.lengthComputable && onProgress(e.loaded / e.total);
    x.onerror = () => reject(new NotesError("network"));
    x.onload = () => {
      try {
        const id = (JSON.parse(x.responseText) as { id?: string }).id;
        if (x.status >= 200 && x.status < 300 && id) return resolve(id);
      } catch {
        /* fall through */
      }
      reject(new NotesError("storage_error"));
    };
    x.send(file);
  });
}

export type NewNote = { title: string; code: string };

/** Where an upload is at, for the post button. */
export type UploadStage = { stage: "check" } | { stage: "send"; frac: number } | { stage: "publish" };

const CHECK_LINES = ["checking it's u…", "asking the portal nicely…", "portal's being slow, hold on…"];

/** What the post button says: lines rotate while the portal checks it's you (the last one sticks), then the percent. */
export function uploadLine(s: UploadStage, tick: number): string {
  if (s.stage === "send") return `uploading… ${Math.round(s.frac * 100)}%`;
  if (s.stage === "publish") return "making it official…";
  return CHECK_LINES[Math.min(tick, CHECK_LINES.length - 1)];
}

/** The whole upload: announce it, send the file, then ask the server to check and publish it. */
export async function uploadNote(file: File, meta: NewNote, onStage: (s: UploadStage) => void = () => {}): Promise<Note> {
  if (checkFile(file)) throw new NotesError("bad_input");
  onStage({ stage: "check" });
  const started = await withToken((t) =>
    post<{ noteId: string; session: string }>({ action: "init", title: meta.title, code: normCode(meta.code), mime: file.type, size: file.size }, t),
  );
  onStage({ stage: "send", frac: 0 });
  const key = await putFile(started.session, file, (frac) => onStage({ stage: "send", frac }));
  onStage({ stage: "publish" });
  const done = await withToken((t) => post<{ note: Note }>({ action: "commit", noteId: started.noteId, key }, t));
  return done.note;
}

// ---- which notes this student already upvoted (the server counts once anyway) ----

export function votedIds(): string[] {
  try {
    const raw = storage()?.getItem(NOTES_VOTED);
    const list = raw ? (JSON.parse(raw) as unknown) : [];
    return Array.isArray(list) ? list.filter((x): x is string => typeof x === "string") : [];
  } catch {
    return [];
  }
}

export function rememberVote(id: string): void {
  try {
    storage()?.setItem(NOTES_VOTED, JSON.stringify([...new Set([...votedIds(), id])]));
  } catch {
    /* ignore */
  }
}

export function forgetVote(id: string): void {
  try {
    storage()?.setItem(NOTES_VOTED, JSON.stringify(votedIds().filter((x) => x !== id)));
  } catch {
    /* ignore */
  }
}
