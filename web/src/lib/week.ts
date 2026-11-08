import type { Class, Course } from "./types";
import { zoneOf } from "./verdict";

export type ClassTag = { text: string; tone: "dead" | "fine" | "safe" | "mut" };

/** Can you skip this class? Decided by that subject's attendance right now. */
export function classTag(c: Class, courses: Course[]): ClassTag | null {
  const course = courses.find((x) => x.code === c.code);
  if (!course || course.conducted === 0) return null;
  switch (zoneOf(course.percent)) {
    case "dead":
      return { text: "don't you dare", tone: "dead" };
    case "fine":
      return { text: "show up", tone: "fine" };
    default:
      return course.skip85 > 0
        ? { text: `bunkable · ${course.skip85} ${course.skip85 === 1 ? "life" : "lives"}`, tone: "safe" }
        : { text: "0 lives, show up", tone: "mut" };
  }
}

const small = new Set(["and", "for", "of", "the", "in", "to", "a", "an", "&"]);
const acronyms = new Set(["ai", "iot", "ml", "dbms", "os", "cn", "vlsi", "ui", "ux"]);

/** "AI FOR PREDICTIVE ANALYTICS AND IOT" → "AI for Predictive Analytics and IoT"-ish. */
export function niceName(raw: string): string {
  return raw
    .toLowerCase()
    .split(/\s+/)
    .map((w, i) => {
      if (w === "iot") return "IoT";
      if (acronyms.has(w)) return w.toUpperCase();
      if (i > 0 && small.has(w)) return w;
      return w.charAt(0).toUpperCase() + w.slice(1);
    })
    .join(" ");
}

/** Prefer the dashboard's properly-cased course name; fall back to tidying the timetable's caps. */
export function courseName(c: Class, courses: Course[]): string {
  return courses.find((x) => x.code === c.code)?.name ?? niceName(c.name);
}

export function room(r: string): string {
  const s = r.trim().toLowerCase();
  if (s.startsWith("lab")) return "lab"; // the portal spells it "LABORATARY"
  return s.replace(/\s+/g, "");
}

export const trimTime = (t: string) => t.replace(/^0/, "");

export const todayName = () => new Date().toLocaleDateString("en-US", { weekday: "long" });
