import type { ApiClient, Notebook } from "@nervewiki/api-client";
import { expect, type Page, type Request } from "@playwright/test";

import { answerTo, countAnswers, eventStreamPath } from "./browser";
import { followStreams, holdStream } from "./events";
import { createNotebook } from "./notebooks";
import { createPage } from "./pages";
import { wikiPagePath } from "./wiki-pages";

/** A small real content write gives 61 independent groups: preview 5, a non-final 50, then 6. */
export async function healthPagination(api: ApiClient, credential: string, slug: string) {
  const notebook = await createNotebook(api, credential, slug, "Health pagination", "none", { mode: "llm_wiki" });
  const targets = Array.from({ length: 61 }, (_, i) => `m${String(i).padStart(3, "0")}`);
  const content = `---\ntype: note\nsummary: Pagination witness\n---\n${targets.map((s) => `[[${s}]]`).join(" ")}\n`;
  const overview = await createPage(api, credential, notebook.id, "Overview", null, content);
  return { notebook, overview, targets };
}

export function healthPath(slug: string, notebook: string): string {
  return `/${slug}/notebooks/${notebook}/health`;
}

export function healthCard(page: Page, rule: string) {
  return page.getByRole("main").locator(`section[id="${rule}"]`);
}

export function healthLink(page: Page, notebook: Notebook) {
  return page.getByRole("navigation", { name: `More of ${notebook.name}`, exact: true }).getByRole("link", {
    name: "Health",
    exact: true,
  });
}

export async function healthShown(page: Page, checks = 8): Promise<void> {
  await expect(page.getByRole("main").getByRole("heading", { level: 1, name: "Health", exact: true })).toBeVisible();
  await expect(page.locator('output[aria-label="Health results"]')).toHaveText(`${checks} checks completed.`);
  await expect(page.getByRole("button", { name: "Recheck", exact: true })).not.toHaveAttribute("aria-busy", "true");
}

/** Count attempts, including transport failures, separately from successful answers and from each card. */
export function healthReads(page: Page, notebook: string) {
  const prefix = `/api/v0/notebooks/${notebook}/lint`;
  const requests: Request[] = [];
  const answers: { path: string; status: number }[] = [];
  page.on("request", (request) => {
    const pathname = new URL(request.url()).pathname;
    if (request.method() === "GET" && (pathname === prefix || pathname.startsWith(`${prefix}/`)))
      requests.push(request);
  });
  page.on("response", (response) => {
    const pathname = new URL(response.url()).pathname;
    if (response.request().method() === "GET" && (pathname === prefix || pathname.startsWith(`${prefix}/`)))
      answers.push({ path: pathname, status: response.status() });
  });
  return {
    requests,
    answers,
    summary: () => requests.filter((request) => new URL(request.url()).pathname === prefix).length,
    rule: (rule: string) =>
      requests.filter((request) => new URL(request.url()).pathname === `${prefix}/${rule}`).length,
    total: () => requests.length,
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((ready) => (resolve = ready));
  return { promise, resolve };
}

/** Pause one actual browser GET before transport; release forwards it to the real server without a fake answer. */
export async function holdHealthGet(page: Page, pathname: string) {
  const entered = deferred<Request>();
  const released = deferred<void>();
  const ended = deferred<void>();
  let started = false;
  await page.route(
    (url) => url.pathname === pathname,
    async (route) => {
      started = true;
      entered.resolve(route.request());
      try {
        await released.promise;
        if (route.request().failure() === null) await route.continue();
      } finally {
        ended.resolve();
      }
    },
    { times: 1 }
  );
  return {
    entered: entered.promise,
    release: () => released.resolve(),
    finish: async () => {
      released.resolve();
      if (started) await ended.promise;
    },
  };
}

/** Observe a clone's real first SSE frame; the application receives the original Response unchanged. */
async function observeHello(page: Page): Promise<void> {
  await page.addInitScript(() => {
    const observed: { hellos: string[]; errors: string[] } = { hellos: [], errors: [] };
    Object.assign(window, { lw8Stream: observed });
    const fetch = window.fetch.bind(window);
    window.fetch = async (...args) => {
      const response = await fetch(...args);
      if (new URL(response.url).pathname === "/api/v0/events" && response.status === 200) {
        const reader = response.clone().body?.getReader();
        if (reader) {
          void (async () => {
            let text = "";
            const decoder = new TextDecoder();
            try {
              while (!text.includes("\n\n")) {
                // oxlint-disable-next-line no-await-in-loop -- observe the first real SSE frame, chunk by chunk
                const chunk = await reader.read();
                if (chunk.done) break;
                text += decoder.decode(chunk.value, { stream: true });
              }
              observed.hellos.push(text.slice(0, text.indexOf("\n\n") + 2));
            } catch (error) {
              observed.errors.push(String(error));
            } finally {
              void reader.cancel().catch(() => undefined);
            }
          })();
        }
      }
      return response;
    };
  });
}

export async function streamHellos(page: Page): Promise<string[]> {
  const observed = await page.evaluate(
    () => (window as unknown as { lw8Stream: { hellos: string[]; errors: string[] } }).lw8Stream
  );
  expect(observed.errors).toEqual([]);
  for (const hello of observed.hellos) expect(hello).toMatch(/^event: hello\ndata: \{"heartbeat_seconds":\d+\}\n\n$/);
  return observed.hellos;
}

/** Mount health, then establish its first real stream and wait for the initial outside-in refresh. */
export async function connectedHealth(page: Page, slug: string, notebook: Notebook, pageId: string) {
  await observeHello(page);
  const streams = followStreams(page.context());
  const ended = new Set<Request>();
  page.on("requestfinished", (request) => ended.add(request));
  page.on("requestfailed", (request) => ended.add(request));
  const notebooks = countAnswers(page, "GET", `/api/v0/workspaces/${slug}/notebooks`);
  const nodes = countAnswers(page, "GET", `/api/v0/notebooks/${notebook.id}/nodes`);
  const release = await holdStream(page);
  let stream: Request;
  try {
    await page.goto(wikiPagePath(slug, notebook.id, pageId));
    await healthLink(page, notebook).click();
    await healthShown(page);
    const before = { notebooks: notebooks(), nodes: nodes() };
    const answer = answerTo(page, "GET", eventStreamPath);
    release();
    const response = await answer;
    expect(response.status()).toBe(200);
    stream = response.request();
    await expect.poll(async () => (await streamHellos(page)).length).toBe(1);
    await expect.poll(notebooks).toBeGreaterThan(before.notebooks);
    await expect.poll(nodes).toBeGreaterThan(before.nodes);
    await expect.poll(streams.count).toBe(1);
    expect(ended.has(stream)).toBe(false);
    await page.evaluate(() => Object.assign(window, { lw8Document: true }));
  } finally {
    release();
  }
  return { streams, stream, ended, notebooks, nodes };
}

export async function healthDocumentStayed(page: Page): Promise<void> {
  expect(await page.evaluate(() => (window as unknown as { lw8Document?: boolean }).lw8Document)).toBe(true);
}
