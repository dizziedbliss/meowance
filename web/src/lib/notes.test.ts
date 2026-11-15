import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  checkFile,
  clearToken,
  deleteNote,
  listNotes,
  loadToken,
  myNotes,
  NotesError,
  notesErrorCopy,
  normCode,
  saveToken,
  sizeLabel,
  unvote,
  uploadLine,
  vote,
} from "./notes";

vi.mock("./vault", () => ({
  loadCreds: async () => ({ usn: "4MC23AI001", dob: "15082004" }),
  NOTES_TOKEN: "meowance.notes.token",
  NOTES_VOTED: "meowance.notes.voted",
}));

const mem = () => {
  const m = new Map<string, string>();
  return { getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v), removeItem: (k: string) => void m.delete(k) };
};

describe("checkFile", () => {
  const f = (type: string, size: number) => ({ type, size });
  it("takes a pdf, png or jpg under 25 mb", () => {
    for (const t of ["application/pdf", "image/png", "image/jpeg"]) expect(checkFile(f(t, 1000))).toBeNull();
    expect(checkFile(f("application/pdf", 25 * 1024 * 1024))).toBeNull();
  });
  it("says why not, in words", () => {
    expect(checkFile(f("application/pdf", 25 * 1024 * 1024 + 1))).toMatch(/25 mb/);
    expect(checkFile(f("application/zip", 10))).toMatch(/pdf|png|jpg/);
    expect(checkFile(f("", 10))).toMatch(/pdf|png|jpg/);
    expect(checkFile(f("image/png", 0))).toMatch(/empty/);
  });
});

describe("small helpers", () => {
  it("sizeLabel", () => {
    expect(sizeLabel(340)).toBe("340 b");
    expect(sizeLabel(2048)).toBe("2 kb");
    expect(sizeLabel(1.5 * 1024 * 1024)).toBe("1.5 mb");
  });
  it("normCode upper-cases and trims", () => expect(normCode("  23ai501 ")).toBe("23AI501"));
});

describe("uploadLine (what the post button says while it works)", () => {
  it("rotates while it checks it's you, and the last line sticks", () => {
    const lines = [0, 1, 2, 3, 9].map((t) => uploadLine({ stage: "check" }, t));
    expect(new Set(lines).size).toBe(3);
    expect(lines[3]).toBe(lines[2]);
    expect(lines[4]).toBe(lines[2]);
  });
  it("shows the percent while the file goes up", () => {
    expect(uploadLine({ stage: "send", frac: 0.426 }, 0)).toBe("uploading… 43%");
    expect(uploadLine({ stage: "send", frac: 1 }, 5)).toBe("uploading… 100%");
  });
  it("says something while the server checks the file", () => {
    expect(uploadLine({ stage: "publish" }, 0)).toMatch(/official/);
  });
});

describe("token storage", () => {
  it("keeps a token until an hour before it ends", () => {
    const s = mem();
    const now = 1_000_000_000_000;
    saveToken(s, "tok", 7 * 86400, now);
    expect(loadToken(s, now + 6 * 86400 * 1000)).toBe("tok");
    expect(loadToken(s, now + 7 * 86400 * 1000 - 30 * 60 * 1000)).toBeNull(); // inside the last hour: time to renew
  });
  it("forgets on clear, and on garbage", () => {
    const s = mem();
    saveToken(s, "tok", 3600 * 24, 0);
    clearToken(s);
    expect(loadToken(s, 1)).toBeNull();
    s.setItem("meowance.notes.token", "{not json");
    expect(loadToken(s, 1)).toBeNull();
  });
});

describe("every error code has words", () => {
  it("notesErrorCopy is lowercase and non-empty", () => {
    for (const [code, text] of Object.entries(notesErrorCopy)) {
      expect(text.length, code).toBeGreaterThan(8);
      expect(text, code).toBe(text.toLowerCase());
    }
  });
});

describe("api calls", () => {
  const store = mem();
  beforeEach(() => {
    vi.stubGlobal("localStorage", store);
    store.removeItem("meowance.notes.token");
  });
  afterEach(() => vi.unstubAllGlobals());

  const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

  it("listNotes asks for the course in capitals and returns the notes", async () => {
    const fetchMock = vi.fn().mockResolvedValue(json(200, { notes: [{ id: "a", title: "t", label: "just another nerd", mime: "application/pdf", size: 5, votes: 2, created: "2026-11-15T00:00:00Z" }] }));
    vi.stubGlobal("fetch", fetchMock);
    const got = await listNotes("23ai501");
    expect(fetchMock.mock.calls[0][0]).toBe("/api/notes?code=23AI501");
    expect(got).toHaveLength(1);
    expect(got[0].votes).toBe(2);
  });

  it("turns an error body into a NotesError and a dead network into 'network'", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json(503, { error: "notes_off" })));
    await expect(listNotes("23AI501")).rejects.toMatchObject({ code: "notes_off" });
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("offline")));
    await expect(listNotes("23AI501")).rejects.toBeInstanceOf(NotesError);
    await expect(listNotes("23AI501")).rejects.toMatchObject({ code: "network" });
  });

  it("logs in with the saved creds when there is no token", async () => {
    const seen: string[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (_url: string, init?: RequestInit) => {
        const body = JSON.parse(String(init?.body ?? "{}"));
        seen.push(body.action);
        return body.action === "login" ? json(200, { token: "fresh", expiresIn: 604800 }) : json(200, { ok: true });
      }),
    );
    await vote("note1");
    expect(seen).toEqual(["login", "vote"]);
    expect(loadToken(store, Date.now())).toBe("fresh");
  });

  it("vote says whether it counted, so a repeat vote doesn't inflate the number", async () => {
    saveToken(store, "tok", 7 * 86400, Date.now());
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json(200, { ok: true, added: true })).mockResolvedValueOnce(json(200, { ok: true, added: false })));
    expect(await vote("note1")).toBe(true);
    expect(await vote("note1")).toBe(false);
  });

  it("unvote and delete send their own actions", async () => {
    saveToken(store, "tok", 7 * 86400, Date.now());
    const seen: string[] = [];
    vi.stubGlobal("fetch", vi.fn(async (_u: string, init?: RequestInit) => {
      seen.push(JSON.parse(String(init?.body)).action);
      return json(200, { ok: true, removed: true });
    }));
    expect(await unvote("n1")).toBe(true);
    await deleteNote("n1");
    expect(seen).toEqual(["unvote", "delete"]);
  });

  it("myNotes asks only when there's already a login (it never pops one), else says it doesn't know", async () => {
    const f = vi.fn().mockResolvedValue(json(200, { mine: ["a"], voted: ["b"] }));
    vi.stubGlobal("fetch", f);
    expect(await myNotes("23ai501")).toBeNull();
    expect(f).not.toHaveBeenCalled();
    saveToken(store, "tok", 7 * 86400, Date.now());
    expect(await myNotes("23ai501")).toEqual({ mine: ["a"], voted: ["b"] });
    expect(JSON.parse(String(f.mock.calls[0][1].body))).toMatchObject({ action: "mine", code: "23AI501" });
  });

  it("myNotes shrugs off a failure (the list still works without it)", async () => {
    saveToken(store, "tok", 7 * 86400, Date.now());
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json(401, { error: "bad_token" })));
    expect(await myNotes("23ai501")).toBeNull();
  });

  it("a fresh token the server still refuses is an error, not a loop", async () => {
    vi.stubGlobal("fetch", vi.fn(async (_u: string, init?: RequestInit) => (JSON.parse(String(init?.body)).action === "login" ? json(200, { token: "t", expiresIn: 604800 }) : json(401, { error: "bad_token" }))));
    await expect(vote("note1")).rejects.toMatchObject({ code: "bad_token" });
  });

  it("a saved token the server refuses gets one fresh login and a retry", async () => {
    saveToken(store, "stale", 7 * 86400, Date.now());
    const seen: string[] = [];
    let voteCalls = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (_url: string, init?: RequestInit) => {
        const body = JSON.parse(String(init?.body ?? "{}"));
        seen.push(body.action);
        if (body.action === "login") return json(200, { token: `tok${seen.length}`, expiresIn: 604800 });
        voteCalls++;
        return voteCalls === 1 ? json(401, { error: "bad_token" }) : json(200, { ok: true });
      }),
    );
    await vote("note1");
    expect(seen).toEqual(["vote", "login", "vote"]); // refused once, logged in again, then through
  });
});
