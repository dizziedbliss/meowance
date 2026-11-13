import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, dobOk, errorCopy, fetchAttendance, sendFeedback, usnOk } from "./api";

afterEach(() => vi.unstubAllGlobals());

const reply = (status: number, body: unknown) =>
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify(body), { status })));

const codeOf = (p: Promise<unknown>) => p.then(() => "ok", (e: unknown) => (e instanceof ApiError ? e.code : `not ApiError: ${e}`));

describe("fetchAttendance", () => {
  it("posts usn + dob as json and returns the body", async () => {
    const f = vi.fn(async () => new Response(JSON.stringify({ student: { usn: "4MC00AA000" } }), { status: 200 }));
    vi.stubGlobal("fetch", f);
    const a = await fetchAttendance("4MC00AA000", "15082004");
    expect(a.student.usn).toBe("4MC00AA000");
    expect(f).toHaveBeenCalledWith("/api/attendance", expect.objectContaining({ method: "POST", body: '{"usn":"4MC00AA000","dob":"15082004"}' }));
  });
  it("maps every server error code to its copy", async () => {
    for (const code of ["bad_login", "too_many_tries", "portal_down", "portal_blocked", "portal_changed"] as const) {
      reply(502, { error: code });
      expect(await codeOf(fetchAttendance("4MC00AA000", "15082004"))).toBe(code);
      expect(errorCopy[code]).toBeTruthy();
    }
  });
  it("treats unknown codes and non-json errors as internal", async () => {
    reply(500, { error: "something_new" });
    expect(await codeOf(fetchAttendance("4MC00AA000", "15082004"))).toBe("internal");
    vi.stubGlobal("fetch", vi.fn(async () => new Response("<html>504</html>", { status: 504 })));
    expect(await codeOf(fetchAttendance("4MC00AA000", "15082004"))).toBe("internal");
  });
  it("reports a failed request as network", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => { throw new TypeError("offline"); }));
    expect(await codeOf(fetchAttendance("4MC00AA000", "15082004"))).toBe("network");
  });
});

describe("input checks", () => {
  it("accepts only mce usns, any case, trimmed", () => {
    expect(usnOk(" 4mc00aa000 ")).toBe(true);
    expect(usnOk("1RV22CS001")).toBe(false);
    expect(usnOk("4MC00AA00")).toBe(false);
  });
  it("accepts only real past ddmmyyyy dates from 1970", () => {
    expect(dobOk("15082004")).toBe(true);
    expect(dobOk("29022004")).toBe(true); // leap year
    expect(dobOk("29022005")).toBe(false);
    expect(dobOk("31042004")).toBe(false);
    expect(dobOk("01011969")).toBe(false);
    expect(dobOk("01012999")).toBe(false);
    expect(dobOk("1508200")).toBe(false);
  });
});

describe("sendFeedback", () => {
  it("posts usn + message and resolves on ok", async () => {
    const f = vi.fn(async () => new Response('{"ok":true}', { status: 200 }));
    vi.stubGlobal("fetch", f);
    await expect(sendFeedback("4MC00AA000", "hi")).resolves.toBeUndefined();
    expect(f).toHaveBeenCalledWith("/api/feedback", expect.objectContaining({ method: "POST", body: '{"usn":"4MC00AA000","message":"hi"}' }));
  });
  it("turns failures into copy", async () => {
    for (const [status, body, want] of [
      [429, { error: "too_many_tries" }, "that's a lot of feedback"],
      [503, { error: "feedback_off" }, "napping"],
      [502, { error: "feedback_failed" }, "couldn't send it"],
    ] as const) {
      reply(status, body);
      await expect(sendFeedback("4MC00AA000", "hi")).rejects.toThrow(want);
    }
    vi.stubGlobal("fetch", vi.fn(async () => { throw new TypeError("offline"); }));
    await expect(sendFeedback("4MC00AA000", "hi")).rejects.toThrow("offline");
  });
});
