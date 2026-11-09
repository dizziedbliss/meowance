// Mirrors pkg/api Response (Go). The server does the bunk math.
export type Student = {
  usn: string;
  name: string;
  dept: string; // two-letter branch from the USN: CI, CS, VL…
  sem: string;
  section: string;
  lastUpdated: string; // as printed by the portal, e.g. 08/11/2026
};

/** One internal assessment (SE1, SE2…). */
export type CIEPart = {
  name: string;
  max: number;
  obtained: number;
  absent: boolean; // portal shows "A"
  pending: boolean; // no marks entered yet
  entered: string; // as printed, e.g. "13th Oct 26"
};

export type Class = { start: string; end: string; code: string; name: string; room: string; batch: string };
export type Day = { weekday: string; date: string; classes: Class[] }; // date is dd-mm-yyyy

export type Course = {
  code: string;
  name: string;
  credits: number;
  attended: number;
  conducted: number;
  percent: number; // one decimal
  need75: number; // classes in a row to get back to 75
  need85: number; // classes in a row to get back to 85
  skip85: number; // classes you can miss and stay at or above 85
  cie?: CIEPart[]; // optional: data cached before scores existed won't have it
  cieTotal?: number;
  cieMax?: number;
};

export type CalendarEvent = { from: string; to: string; title: string; kind: "holiday" | "exam" | "lastday" | "" }; // YYYY-MM-DD
export type Calendar = { start: string; end: string; holidays: string[]; examDays: string[]; events: CalendarEvent[] };

/** timetable / calendar: null = couldn't fetch it this time; undefined = cached before they existed. */
export type Attendance = { student: Student; courses: Course[]; timetable?: Day[] | null; calendar?: Calendar | null };

export type ApiErrorCode =
  | "bad_input"
  | "bad_login"
  | "too_many_tries"
  | "portal_down"
  | "portal_changed"
  | "internal"
  | "network"
  | "portal_blocked";
