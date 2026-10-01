import { createClient } from "@nervewiki/api-client";
import type { BrowserContext, Page, Request } from "@playwright/test";

import { expectRefreshed, generationOf } from "../../fixtures/assert/identity";
import { bearer, emailFor, login, recordOf, refresh, register } from "../../fixtures/auth";
import { expectQuietPage, followAccessToken, watchPage } from "../../fixtures/browser";
import { displayNameField, profileStep, saveProfileStep, workspaceStep } from "../../fixtures/onboarding-pages";
import { expect, test } from "../../fixtures/test";

// A4, refresh (M1 design 3). Reusing an old token is A5.

/** A refresh a tab sent: the generation of the refresh token it sent, and nervewiki's answer. */
interface Refresh {
  generation: number;
  status: number;
}

function isRefresh(request: Request): boolean {
  return new URL(request.url()).pathname === "/api/v0/auth/refresh";
}

function sentGeneration(request: Request): number {
  return generationOf((request.postDataJSON() as { refresh_token: string }).refresh_token);
}

/** What the tabs of a context send: every refresh, and the access token each tab sent last. */
class TabRecorder {
  readonly #refreshes: Refresh[] = [];
  readonly #pending: Promise<void>[] = [];
  readonly #accessTokens: (() => string)[] = [];

  record(tab: Page): void {
    this.#accessTokens.push(followAccessToken(tab));
    tab.on("requestfinished", (request) => {
      if (isRefresh(request)) {
        this.#pending.push(this.#add(request));
      }
    });
    tab.on("requestfailed", (request) => {
      if (isRefresh(request)) {
        this.#refreshes.push({ generation: sentGeneration(request), status: 0 });
      }
    });
  }

  /** The access token each tab sent last, in the order the tabs were recorded: "" for a tab that sent none. */
  lastAccessTokens(): string[] {
    return this.#accessTokens.map((last) => last());
  }

  /** The refreshes, once every answer is in. */
  async refreshes(): Promise<Refresh[]> {
    await Promise.all(this.#pending);
    return this.#refreshes;
  }

  async #add(request: Request): Promise<void> {
    const response = await request.response();
    this.#refreshes.push({ generation: sentGeneration(request), status: response?.status() ?? 0 });
  }
}

/**
 * Holds the first refresh the tabs of context send from now on, until another refresh goes out or holdMs
 * pass. With the refresh lock no other refresh can go out meanwhile, and the hold ends at holdMs; without
 * it, the other tab's refresh goes out while this one is held, with the same refresh token.
 */
async function holdFirstRefresh(context: BrowserContext, holdMs: number): Promise<void> {
  let release: (() => void) | undefined;
  await context.route("**/api/v0/auth/refresh", async (route) => {
    if (release === undefined) {
      await new Promise<void>((resolve) => {
        release = resolve;
        setTimeout(resolve, holdMs);
      });
    } else {
      release();
    }
    await route.continue();
  });
}

for (const locks of [true, false]) {
  const how = locks ? "navigator.locks" : "the localStorage lease, without navigator.locks";
  test(`A4 (page): two tabs whose access tokens expired both save at once, refreshing one at a time (${how})`, async ({
    context,
    db,
    nervewikiWith,
    signedInPage,
  }, testInfo) => {
    // Access tokens of 3 s: every request the tabs send refreshes first (30 s before the end, M1/P5 design 3.2).
    const shortLived = await nervewikiWith(db.url, { env: { NWIKI_AUTH__ACCESS_TOKEN_TTL: "3s" } });
    const api = createClient({ baseUrl: shortLived.baseURL });
    const signedUp = await register(api, emailFor(testInfo));
    if (!locks) {
      await context.addInitScript(() => {
        Reflect.deleteProperty(Navigator.prototype, "locks");
      });
    }
    const recorder = new TabRecorder();
    const tabA = await signedInPage(signedUp, shortLived.baseURL);
    const tabB = await context.newPage();
    const watchB = await watchPage(tabB);
    // One tab after the other, so that the second finds the record the first refreshed.
    const openProfileStep = async (tab: Page) => {
      recorder.record(tab);
      await tab.goto(`${shortLived.baseURL}/onboarding`);
      await expect(profileStep(tab)).toBeVisible();
      expect(await tab.evaluate(() => "locks" in navigator)).toBe(locks);
      await displayNameField(tab).fill("Ada");
    };
    await openProfileStep(tabA);
    await openProfileStep(tabB);

    // Wait until nervewiki refuses the last access token each tab sent: every token in the tabs has expired.
    await Promise.all(
      recorder.lastAccessTokens().map(async (token) => {
        expect(token, "the tab sent an access token").not.toBe("");
        await expect
          .poll(async () => (await api.GET("/api/v0/me", { headers: bearer(token) })).response.status, {
            timeout: 10_000,
          })
          .toBe(401);
      })
    );

    // Both tabs save the step at once: both go on to the next step, neither to the sign-in page. The first
    // refresh is held a while, so that the other tab's would overlap it if nothing kept them apart.
    await holdFirstRefresh(context, 1_000);
    expect(await Promise.all([tabA, tabB].map((tab) => saveProfileStep(tab)))).toEqual([200, 200]);
    await Promise.all(
      [tabA, tabB].map(async (tab) => {
        await expect(workspaceStep(tab)).toBeVisible();
        await expect(tab).toHaveURL(`${shortLived.baseURL}/onboarding`);
      })
    );

    // Every refresh succeeded, and no two of them overlapped: each sent the token the one before it got,
    // so the generations sent are 0, 1, 2 … once each. Two at the same time would send the same token, and
    // nervewiki would take the second for a stolen copy (A5). The browser's timings cannot show it: a held
    // request's timing counts from when it was let go.
    const refreshes = await recorder.refreshes();
    expect(refreshes.length).toBeGreaterThanOrEqual(4);
    expect(refreshes.map((r) => r.status)).toEqual(refreshes.map(() => 200));
    expect(refreshes.map((r) => r.generation).toSorted((a, b) => a - b)).toEqual(refreshes.map((_, i) => i));
    expect(watchB.apiFailures).toEqual([]);
    const record = await recordOf(tabA);
    await expectRefreshed(db, record?.refresh_token ?? "", refreshes.length, signedUp.refresh_token_expires_at);
    // Tab A is the test's page, which the page fixture checks as the test ends.
    await expectQuietPage(tabB, watchB);
  });
}

test("A4 (API): each refresh uses the last token; the generation counts up and the session end stays", async ({
  api,
  db,
}, testInfo) => {
  const email = emailFor(testInfo);
  await register(api, email);
  const first = await login(api, email);

  const second = await refresh(api, first.refresh_token);
  const third = await refresh(api, second.refresh_token);
  const fourth = await refresh(api, third.refresh_token);

  // The access token holds only sub, sid and exp in whole seconds: within
  // one second it can come out the same, so only the refresh token must
  // differ.
  for (const [before, after] of [
    [first, second],
    [second, third],
    [third, fourth],
  ] as const) {
    expect(after.refresh_token).not.toBe(before.refresh_token);
    expect(after.refresh_token_expires_at).toBe(first.refresh_token_expires_at);
  }
  await expectRefreshed(db, fourth.refresh_token, 3, first.refresh_token_expires_at);
  const me = await api.GET("/api/v0/me", { headers: bearer(fourth.access_token) });
  expect(me.response.status).toBe(200);
});
