import { createHash } from "node:crypto";

import type { ApiTokenCreated } from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import type { Database } from "../db";

// Database assertions of the identity stories, by table. The page version and
// the API version of a story call the same function (v0.1 design 12.5).

const refreshTokenPrefix = "nwk_rt_";
const dayMs = 24 * 60 * 60 * 1000;

/** A refresh token's content (M1/P1 design 3.4): nwk_rt_, then session id, generation, secret and tag in base64url. */
interface RefreshTokenParts {
  sessionId: string;
  generation: number;
  secret: Buffer;
}

function parseRefreshToken(token: string): RefreshTokenParts {
  expect(token.startsWith(refreshTokenPrefix), `${token} starts with ${refreshTokenPrefix}`).toBe(true);
  const raw = Buffer.from(token.slice(refreshTokenPrefix.length), "base64url");
  expect(raw.length, "a refresh token holds 68 bytes").toBe(68);
  const id = raw.subarray(0, 16).toString("hex");
  return {
    sessionId: `${id.slice(0, 8)}-${id.slice(8, 12)}-${id.slice(12, 16)}-${id.slice(16, 20)}-${id.slice(20)}`,
    generation: raw.readUInt32BE(16),
    secret: raw.subarray(20, 52),
  };
}

/** The SHA-256 of a refresh token's secret: what auth_sessions.token_hash holds of its generation. */
function secretHash(refreshToken: string): Buffer {
  return createHash("sha256").update(parseRefreshToken(refreshToken).secret).digest();
}

/**
 * refreshToken's generation with a random secret and tag: what someone who saw only the session id
 * could make of an older generation (A5).
 */
export function forgedFrom(refreshToken: string, generation = 0): string {
  const raw = Buffer.from(refreshToken.slice(refreshTokenPrefix.length), "base64url");
  raw.writeUInt32BE(generation, 16);
  raw.set(Buffer.from(Array.from({ length: 48 }, () => Math.floor(Math.random() * 256))), 20);
  return refreshTokenPrefix + raw.toString("base64url");
}

/** A session's row, as the session assertions read it. */
export interface SessionRow {
  id: string;
  user_id: string;
  token_hash: Buffer;
  generation: number;
  user_agent: string;
  ip: string;
  expires_at: Date;
  created_at: Date;
  last_refreshed_at: Date | null;
  revoked_at: Date | null;
  revoke_reason: string | null;
}

/** The row of the session that refreshToken belongs to. */
export async function sessionOf(db: Database, refreshToken: string): Promise<SessionRow> {
  const rows = await db.query<SessionRow>(
    `SELECT id, user_id, token_hash, generation, user_agent, host(ip) AS ip, expires_at, created_at,
            last_refreshed_at, revoked_at, revoke_reason
       FROM auth_sessions WHERE id = $1`,
    [parseRefreshToken(refreshToken).sessionId]
  );
  expect(rows, "the session of the refresh token").toHaveLength(1);
  return rows[0] as SessionRow;
}

/** What a sign-up or a sign-in sent and got back. */
export interface SignIn {
  /** The address as typed; the account holds it trimmed and lowercased. */
  email: string;
  accessToken: string;
  refreshToken: string;
  userAgent: string;
  ip: string;
}

/**
 * users: one account has the address typed, lowercased, with an argon2id
 * hash, the display name from the address, active, and no onboarding step
 * done. Returns its id.
 */
export async function expectNewAccount(db: Database, typed: string): Promise<string> {
  const email = typed.trim().toLowerCase();
  const users = await db.query<{
    id: string;
    password: string;
    display_name: string;
    is_active: boolean;
    onboarding_steps: string[];
  }>("SELECT id, password, display_name, is_active, onboarding_steps FROM users WHERE email = $1", [email]);
  expect(users).toHaveLength(1);
  const [user] = users;
  if (!user) {
    throw new Error(`no account of ${email}`);
  }
  expect(user.password).toMatch(/^\$argon2id\$/);
  expect(user.display_name).toBe(email.slice(0, email.lastIndexOf("@")));
  expect(user.is_active).toBe(true);
  expect(user.onboarding_steps).toEqual([]);
  return user.id;
}

/** The id of the account of the address typed. */
export async function accountIdOf(db: Database, typed: string): Promise<string> {
  const users = await db.query<{ id: string }>("SELECT id FROM users WHERE email = $1", [typed.trim().toLowerCase()]);
  expect(users, `the account of ${typed}`).toHaveLength(1);
  return (users[0] as { id: string }).id;
}

/**
 * auth_sessions: the session of s.refreshToken, which s.accessToken's claims
 * name too, is new: generation 0 of the account userId, holding the hash of
 * the token's secret, with the caller's User-Agent and IP, never refreshed,
 * live, ending 30 days after it began.
 */
export async function expectNewSession(db: Database, userId: string, s: SignIn): Promise<void> {
  const token = parseRefreshToken(s.refreshToken);
  const session = await sessionOf(db, s.refreshToken);
  expect(session.user_id).toBe(userId);
  // The access token's claims name the account and the session; the
  // signature is the server's to check.
  const claims: unknown = JSON.parse(Buffer.from(s.accessToken.split(".")[1] ?? "", "base64url").toString());
  expect(claims).toMatchObject({ sub: userId, sid: token.sessionId });
  expect(token.generation).toBe(0);
  expect(session.generation).toBe(0);
  expect(session.token_hash.equals(secretHash(s.refreshToken))).toBe(true);
  expect(session.user_agent).toBe(s.userAgent);
  expect(session.ip).toBe(s.ip);
  // auth.session_ttl is 720h: the session ends 30 days after it began.
  expect(session.expires_at.getTime() - session.created_at.getTime()).toBe(30 * dayMs);
  expect(session.last_refreshed_at).toBeNull();
  expect(session.revoked_at).toBeNull();
  expect(session.revoke_reason).toBeNull();
}

/**
 * A4: after `refreshes` refreshes, each with the token the last one
 * returned, the session is live at that generation, holds the hash of the
 * latest token's secret, was refreshed, and still ends at sessionEnd, the
 * refresh_token_expires_at of the sign-in (M1/P2 design 3.5).
 */
export async function expectRefreshed(
  db: Database,
  latest: string,
  refreshes: number,
  sessionEnd: string
): Promise<void> {
  const session = await sessionOf(db, latest);
  expect(parseRefreshToken(latest).generation).toBe(refreshes);
  expect(session.generation).toBe(refreshes);
  expect(session.token_hash.equals(secretHash(latest))).toBe(true);
  expect(session.last_refreshed_at).not.toBeNull();
  expect(session.expires_at.getTime()).toBe(new Date(sessionEnd).getTime());
  expect(session.revoked_at).toBeNull();
}

/** A5, A6: the session of refreshToken is revoked for reason. */
export async function expectRevoked(
  db: Database,
  refreshToken: string,
  reason: "logout" | "reuse_detected"
): Promise<void> {
  const session = await sessionOf(db, refreshToken);
  expect(session.revoked_at).not.toBeNull();
  expect(session.revoke_reason).toBe(reason);
}

/**
 * auth_sessions: the account's sessions, oldest first, each with the
 * reason it was revoked for, or "" while it is live.
 */
async function sessionReasonsOf(db: Database, userId: string): Promise<string[]> {
  const rows = await db.query<{ reason: string }>(
    "SELECT coalesce(revoke_reason, '') AS reason FROM auth_sessions WHERE user_id = $1 ORDER BY created_at, id",
    [userId]
  );
  return rows.map((r) => r.reason);
}

/** The row of an account, as the account assertions read it. */
interface AccountRow {
  display_name: string;
  is_active: boolean;
  onboarding_steps: string[];
  password: string;
}

export async function accountOf(db: Database, userId: string): Promise<AccountRow> {
  const rows = await db.query<AccountRow>(
    "SELECT display_name, is_active, onboarding_steps, password FROM users WHERE id = $1",
    [userId]
  );
  expect(rows, `the account ${userId}`).toHaveLength(1);
  return rows[0] as AccountRow;
}

/** A8: the account's display name is name. */
export async function expectDisplayName(db: Database, userId: string, name: string): Promise<void> {
  expect((await accountOf(db, userId)).display_name).toBe(name);
}

/** A9: the account has recorded steps, in that order, each once. */
export async function expectOnboardingSteps(db: Database, userId: string, steps: string[]): Promise<void> {
  expect((await accountOf(db, userId)).onboarding_steps).toEqual(steps);
}

/**
 * A7: the password changed (another argon2id hash than before) and every
 * session but kept is revoked for password_changed; kept, when given, is
 * the caller's own session and stays live.
 */
export async function expectPasswordChanged(
  db: Database,
  userId: string,
  hashBefore: string,
  kept?: string
): Promise<void> {
  const account = await accountOf(db, userId);
  expect(account.password).toMatch(/^\$argon2id\$/);
  expect(account.password).not.toBe(hashBefore);
  const keptId = kept === undefined ? undefined : parseRefreshToken(kept).sessionId;
  const sessions = await db.query<{ id: string; reason: string | null }>(
    "SELECT id, revoke_reason AS reason FROM auth_sessions WHERE user_id = $1",
    [userId]
  );
  for (const session of sessions) {
    expect(session.reason, `session ${session.id}`).toBe(session.id === keptId ? null : "password_changed");
  }
}

/** A11: the account is inactive and every session of it is revoked for deactivated. */
export async function expectDeactivated(db: Database, userId: string): Promise<void> {
  expect((await accountOf(db, userId)).is_active).toBe(false);
  for (const reason of await sessionReasonsOf(db, userId)) {
    expect(reason).toBe("deactivated");
  }
}

/** A personal access token's row. */
interface TokenRow {
  user_id: string;
  token_hash: Buffer;
  name: string;
  expires_at: Date | null;
  last_used_at: Date | null;
  revoked_at: Date | null;
  created_at: Date;
}

async function tokenOf(db: Database, id: string): Promise<TokenRow> {
  const rows = await db.query<TokenRow>(
    "SELECT user_id, token_hash, name, expires_at, last_used_at, revoked_at, created_at FROM api_tokens WHERE id = $1",
    [id]
  );
  expect(rows, `the token ${id}`).toHaveLength(1);
  return rows[0] as TokenRow;
}

/**
 * A10: api_tokens has the token answered, of the account userId: the
 * SHA-256 of the whole token, never the token, with the name and expiry
 * answered, created when the answer says, never used, live.
 */
export async function expectNewToken(db: Database, userId: string, created: ApiTokenCreated): Promise<void> {
  expect(created.token).toMatch(/^nwk_pat_[A-Za-z0-9_-]{43}$/);
  expect(created.last_used_at).toBeNull();
  const row = await tokenOf(db, created.id);
  expect(row.user_id).toBe(userId);
  expect(row.token_hash.equals(createHash("sha256").update(created.token).digest())).toBe(true);
  expect(row.name).toBe(created.name);
  expect(row.expires_at?.toISOString() ?? null).toBe(created.expires_at && new Date(created.expires_at).toISOString());
  expect(row.created_at.getTime()).toBe(new Date(created.created_at).getTime());
  expect(row.last_used_at).toBeNull();
  expect(row.revoked_at).toBeNull();
}

/** A10: the token has authenticated a request: last_used_at is set. */
export async function expectTokenUsed(db: Database, id: string): Promise<void> {
  expect((await tokenOf(db, id)).last_used_at).not.toBeNull();
}

/** A10: the token is revoked (a soft delete: the row stays). */
export async function expectTokenRevoked(db: Database, id: string): Promise<void> {
  expect((await tokenOf(db, id)).revoked_at).not.toBeNull();
}

/** The rows of the identity tables. */
export interface IdentityCounts {
  users: number;
  sessions: number;
  tokens: number;
}

export async function countIdentity(db: Database): Promise<IdentityCounts> {
  const [counts] = await db.query<IdentityCounts>(
    `SELECT (SELECT count(*)::int FROM users) AS users, (SELECT count(*)::int FROM auth_sessions) AS sessions,
      (SELECT count(*)::int FROM api_tokens) AS tokens`
  );
  if (!counts) {
    throw new Error("the counts query returned no row");
  }
  return counts;
}

/** A refused sign-up, sign-in or token creation added no account, session or token. */
export async function expectNothingAdded(db: Database, before: IdentityCounts): Promise<void> {
  expect(await countIdentity(db)).toEqual(before);
}
