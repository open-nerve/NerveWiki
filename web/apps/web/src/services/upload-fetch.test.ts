import { describe, expect, test } from "vitest";

import { FakeTransfer } from "../test/transfer";
import { uploadFetch } from "./upload-fetch";

/** A request as openapi-fetch hands it over: the token on it, and the Content-Type of its own body. */
function request(): Request {
  return new Request("http://nervewiki.test/api/v0/notebooks/n1/assets", {
    method: "POST",
    headers: { Authorization: "Bearer at-1", "Content-Type": "text/plain;charset=UTF-8", "X-Other": "1" },
  });
}

/** sending sends form by uploadFetch with options, on a FakeTransfer it answers. */
function sending(form: FormData, options: Parameters<typeof uploadFetch>[1] = {}) {
  let transfer: FakeTransfer | undefined;
  const made = () => {
    transfer = new FakeTransfer();
    return transfer;
  };
  const answer = uploadFetch(form, options, made)(request());
  const sent = () => {
    if (transfer === undefined) {
      throw new Error("nothing was sent");
    }
    return transfer;
  };
  return { answer, sent, made: () => transfer !== undefined };
}

describe("uploadFetch", () => {
  test("sends the form, with the request's method, address and headers but its Content-Type, which the form's is", () => {
    const form = new FormData();
    form.append("name", "a.png");
    const { sent } = sending(form);

    expect(sent().method).toBe("POST");
    expect(sent().url).toBe("http://nervewiki.test/api/v0/notebooks/n1/assets");
    expect(sent().body).toBe(form);
    expect([...sent().headers]).toEqual([
      ["authorization", "Bearer at-1"],
      ["x-other", "1"],
    ]);
  });

  test("answers as fetch does: the status, the headers and the body", async () => {
    const { answer, sent } = sending(new FormData());

    await sent().answer(
      new Response(JSON.stringify({ id: "a1" }), {
        status: 201,
        statusText: "Created",
        headers: { "Content-Type": "application/json", "X-Request-Id": "r1" },
      })
    );

    const response = await answer;
    expect(response.status).toBe(201);
    expect(response.statusText).toBe("Created");
    expect(response.headers.get("content-type")).toBe("application/json");
    expect(response.headers.get("x-request-id")).toBe("r1");
    await expect(response.json()).resolves.toEqual({ id: "a1" });
  });

  test("answers a status without a body with none", async () => {
    const { answer, sent } = sending(new FormData());

    await sent().answer(new Response(null, { status: 204 }));

    expect((await answer).body).toBeNull();
  });

  test("tells the progress of the body going out", () => {
    const told: [number, number][] = [];
    const { sent } = sending(new FormData(), { progress: (loaded, total) => told.push([loaded, total]) });

    sent().progress(10, 100);
    sent().progress(100, 100);

    expect(told).toEqual([
      [10, 100],
      [100, 100],
    ]);
  });

  test("a network failure, or a timeout, rejects with a TypeError, as fetch's", async () => {
    const failed = sending(new FormData());
    failed.sent().fail();
    await expect(failed.answer).rejects.toBeInstanceOf(TypeError);

    const late = sending(new FormData());
    late.sent().dispatchEvent(new Event("timeout"));
    await expect(late.answer).rejects.toBeInstanceOf(TypeError);
  });

  test("the signal aborting stops the transfer, which rejects with an AbortError", async () => {
    const stop = new AbortController();
    const { answer, sent } = sending(new FormData(), { signal: stop.signal });

    stop.abort();

    await expect(answer).rejects.toMatchObject({ name: "AbortError" });
    expect(sent().aborted).toBe(true);
  });

  test("a signal aborted already sends nothing", async () => {
    const stop = new AbortController();
    stop.abort();
    const { answer, made } = sending(new FormData(), { signal: stop.signal });

    await expect(answer).rejects.toMatchObject({ name: "AbortError" });
    expect(made()).toBe(false);
  });

  test("once answered, the signal no longer stops it", async () => {
    const stop = new AbortController();
    const { answer, sent } = sending(new FormData(), { signal: stop.signal });

    await sent().answer(new Response("{}", { status: 200 }));
    stop.abort();

    expect((await answer).status).toBe(200);
    expect(sent().aborted).toBe(false);
  });
});
