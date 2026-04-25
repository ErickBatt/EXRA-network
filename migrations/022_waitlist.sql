-- Waitlist table for landing page early-access form
-- Roles: tester | investor | buyer | ghost (easter egg)

create table if not exists public.waitlist (
  id          uuid        primary key default gen_random_uuid(),
  created_at  timestamptz not null    default now(),
  email       text        not null,
  name        text        not null,
  role        text        not null    check (role in ('tester','investor','buyer','ghost')),
  telegram    text,
  device_type text,
  country     text,
  use_case    text,
  is_ghost    boolean     not null    default false,
  metadata    jsonb       not null    default '{}'::jsonb
);

-- One entry per email across all roles
create unique index if not exists waitlist_email_unique
  on public.waitlist (lower(email));

alter table public.waitlist enable row level security;

-- No anonymous access — only service role key bypasses RLS
-- The Next.js server action uses SUPABASE_SERVICE_ROLE_KEY for all writes
