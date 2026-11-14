-- meowance notes. Idempotent: safe to run on every deploy or by hand.
create table if not exists notes (
  id         text primary key,
  code       text   not null,
  dept       text   not null,
  sem        text   not null,
  title      text   not null,
  storage    text   not null,
  file_key   text   not null default '',
  mime       text   not null,
  size       bigint not null,
  uploader   text   not null,                -- the USN: never leaves the server
  status     text   not null check (status in ('pending', 'live', 'rejected')),
  created_at timestamptz not null
);

create index if not exists notes_course   on notes (code, created_at desc) where status = 'live';
create index if not exists notes_uploader on notes (uploader, created_at);

create table if not exists votes (
  note_id text not null references notes (id) on delete cascade,
  usn     text not null,
  primary key (note_id, usn)
);

create table if not exists reports (
  note_id    text not null references notes (id) on delete cascade,
  usn        text not null,
  reason     text not null,
  created_at timestamptz not null default now(),
  primary key (note_id, usn)
);
