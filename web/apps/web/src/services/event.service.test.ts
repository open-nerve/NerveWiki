import { describe, expect, test } from "vitest";

import { fakeApi, json, problem } from "../test/fakes";
import { ApiError } from "./api";
import { EventService } from "./event.service";
import { PageService } from "./page.service";

describe("EventService.open", () => {
  test("asks GET /api/v0/events with the signal and answers the stream's body unread", async () => {
    let asked: Request | undefined;
    const service = new EventService(
      fakeApi((request) => {
        asked = request;
        return new Response("event: hello\ndata: {}\n\n", { headers: { "Content-Type": "text/event-stream" } });
      })
    );
    const stop = new AbortController();

    const body = await service.open(stop.signal);

    expect(`${asked?.method} ${new URL(asked?.url ?? "").pathname}`).toBe("GET /api/v0/events");
    stop.abort();
    expect(asked?.signal.aborted).toBe(true);
    expect(await new Response(body).text()).toBe("event: hello\ndata: {}\n\n");
  });

  test("throws a 503 not_ready as an ApiError with its Retry-After", async () => {
    const service = new EventService(fakeApi(() => problem(503, "not_ready", {}, { "Retry-After": "1" })));

    const error = await service.open(new AbortController().signal).catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: 503, code: "not_ready", retryAfter: 1 });
  });
});

describe("PageService's edit lock", () => {
  test("lock reads GET edit-lock, releaseLock sends DELETE edit-lock", async () => {
    const asked: string[] = [];
    const service = new PageService(
      fakeApi((request) => {
        asked.push(`${request.method} ${new URL(request.url).pathname}`);
        return request.method === "GET"
          ? json({ holder: { user_id: "u", display_name: "Ada" }, expires_in: 90 })
          : new Response(null, { status: 204 });
      })
    );

    expect(await service.lock("p")).toEqual({ holder: { user_id: "u", display_name: "Ada" }, expires_in: 90 });
    await service.releaseLock("p");

    expect(asked).toEqual(["GET /api/v0/pages/p/edit-lock", "DELETE /api/v0/pages/p/edit-lock"]);
  });
});
