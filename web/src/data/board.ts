// Community board posts. Newest first; the first one gets the "new" marker.
export type Post = {
  when: string;
  title: string;
  body: string;
  sign: string;
};

export const posts: Post[] = [
  {
    when: "11 nov (midnight ffs)",
    title: "Midnight Struggles",
    body: "Bro was trying so hard to block us, let them try blocking whole internet now, oh also found out some sneaky lil bugs- smashed em heh-",
    sign: "Max!!",
  },
  {
    when: "9 nov",
    title: "Contineo was trying to block ussss",
    body: "Bro was trying to block us, but we're too cool for that.",
    sign: "Max!!",
  },
  {
    when: "8 nov",
    title: "timetable + scores",
    body: "donneeee.",
    sign: "Max!!",
  },

  {
    when: "8 nov",
    title: "new ui just dropped",
    body: "darker, meaner, more cat. the old one is in a better place now.",
    sign: "Max!!",
  },
  {
    when: "8 nov",
    title: "85 is the real line",
    body: "below it you're paying the fine. we tell you exactly how many classes it takes to dodge it.",
    sign: "Max!!",
  },
  {
    when: "7 nov",
    title: "the contineo's back",
    body: "came back from the dead. so did we, no cap.",
    sign: "Max!!",
  },
];
