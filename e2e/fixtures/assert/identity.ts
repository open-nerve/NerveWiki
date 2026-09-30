import { createHash } from "node:crypto";

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

/** What a sign-up sent and got back. */
export interface SignIn {
  /** The address as typed; the account holds it trimmed and lowercased. */
  email: string;
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

/**
 * auth_sessions: the account userId has exactly one session, the one of
 * s.refreshToken: generation 0, holding the hash of the token's secret, with
 * the caller's User-Agent and IP, never refreshed, live, ending 30 days after
 * it began.
 */
export async function expectNewSession(db: Database, userId: string, s: SignIn): Promise<void> {
  const token = parseRefreshToken(s.refreshToken);
  const sessions = await db.query<{
    id: string;
    token_hash: Buffer;
    generation: number;
    user_agent: string;
    ip: string;
    expires_at: Date;
    created_at: Date;
    last_refreshed_at: Date | null;
    revoked_at: Date | null;
    revoke_reason: string | null;
  }>(
    `SELECT id, token_hash, generation, user_agent, host(ip) AS ip, expires_at, created_at, last_refreshed_at,
            revoked_at, revoke_reason
       FROM auth_sessions WHERE user_id = $1`,
    [userId]
  );
  expect(sessions).toHaveLength(1);
  const [session] = sessions;
  expect(session?.id).toBe(token.sessionId);
  expect(token.generation).toBe(0);
  expect(session?.generation).toBe(0);
  expect(session?.token_hash.equals(createHash("sha256").update(token.secret).digest())).toBe(true);
  expect(session?.user_agent).toBe(s.userAgent);
  expect(session?.ip).toBe(s.ip);
  // auth.session_ttl is 720h: the session ends 30 days after it began.
  expect((session?.expires_at.getTime() ?? 0) - (session?.created_at.getTime() ?? 0)).toBe(30 * dayMs);
  expect(session?.last_refreshed_at).toBeNull();
  expect(session?.revoked_at).toBeNull();
  expect(session?.revoke_reason).toBeNull();
}

/** The rows of the identity tables. */
export interface IdentityCounts {
  users: number;
  sessions: number;
}

export async function countIdentity(db: Database): Promise<IdentityCounts> {
  const [counts] = await db.query<IdentityCounts>(
    `SELECT (SELECT count(*)::int FROM users) AS users, (SELECT count(*)::int FROM auth_sessions) AS sessions`
  );
  if (!counts) {
    throw new Error("the counts query returned no row");
  }
  return counts;
}

/** A refused sign-up added no account and no session. */
export async function expectNothingAdded(db: Database, before: IdentityCounts): Promise<void> {
  expect(await countIdentity(db)).toEqual(before);
}
