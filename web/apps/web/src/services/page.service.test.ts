import { describe, expect, test } from "vitest";

import { fakeApi, json } from "../test/fakes";
import { EditLeaveService, PageService } from "./page.service";

describe("PageService.openEditSession", () => {
  test("asks for the lock, taking it over from the account's other sessions when asked to", async () => {
    const bodies: unknown[] = [];
    const service = new PageService(
      fakeApi(async (request) => {
        bodies.push(await request.json());
        return json({ id: "s1", page_id: "p1", expires_at: "2026-10-03T08:02:00Z" }, 201);
      })
    );

    await service.openEditSession("p1");
    await service.openEditSession("p1", true);

    expect(bodies).toEqual([{ take_over: false }, { take_over: true }]);
  });
});

describe("PageService.heartbeatEditSession", () => {
  test("asks POST of the session's heartbeat with the signal: aborted, the beat is given up", async () => {
    let asked: Request | undefined;
    const service = new PageService(
      fakeApi((request) => {
        asked = request;
        // An answer that never comes, but for the abort, as fetch's.
        return new Promise<Response>((_, reject) => {
          request.signal.addEventListener("abort", () => reject(request.signal.reason as Error));
        });
      })
    );
    const stop = new AbortController();

    const beat = service.heartbeatEditSession("s1", stop.signal);
    await Promise.resolve();
    stop.abort();

    await expect(beat).rejects.toThrow();
    expect(`${asked?.method} ${new URL(asked?.url ?? "").pathname}`).toBe("POST /api/v0/edit-sessions/s1/heartbeat");
    expect(asked?.signal.aborted).toBe(true);
  });
});

describe("PageService's trash (M8/P2 design 4.1)", () => {
  test("reads a notebook's trash fifty items a page, after a cursor for the next; an item by its node; a restore by its node, with no body", async () => {
    const asked: string[] = [];
    const service = new PageService(
      fakeApi(async (request) => {
        const url = new URL(request.url);
        asked.push(`${request.method} ${url.pathname}${url.search} ${await request.text()}`.trim());
        return request.method === "POST"
          ? json({
              id: "n1",
              kind: "page",
              name: "A",
              renamed: false,
              placed: { parent_id: null, reason: "original" },
              changeset_id: "c1",
            })
          : json({ data: [], next_cursor: null });
      })
    );

    await service.trash("nb1");
    await service.trash("nb1", "next");
    await service.trashItem("n1");
    expect(await service.restoreNode("n1")).toMatchObject({ id: "n1", renamed: false });

    expect(asked).toEqual([
      "GET /api/v0/notebooks/nb1/trash?limit=50",
      "GET /api/v0/notebooks/nb1/trash?limit=50&cursor=next",
      "GET /api/v0/nodes/n1/trash-item",
      "POST /api/v0/nodes/n1/restore",
    ]);
  });
});

describe("EditLeaveService.endOnLeave", () => {
  test("sends DELETE of the session before it returns, with the token and keepalive, and waits for no answer", () => {
    const asked: Request[] = [];
    const service = new EditLeaveService(
      fakeApi((request) => {
        asked.push(request);
        return new Promise<Response>(() => undefined);
      })
    );

    service.endOnLeave("s1", "at-1");

    expect(asked).toHaveLength(1);
    expect(`${asked[0]?.method} ${new URL(asked[0]?.url ?? "").pathname}`).toBe("DELETE /api/v0/edit-sessions/s1");
    expect(asked[0]?.headers.get("Authorization")).toBe("Bearer at-1");
    expect(asked[0]?.keepalive).toBe(true);
  });

  test("a request that fails is let go", async () => {
    let asked = 0;
    const service = new EditLeaveService(
      fakeApi(() => {
        asked++;
        return Promise.reject(new TypeError("offline"));
      })
    );

    service.endOnLeave("s1", "at-1");
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(asked).toBe(1);
  });
});
