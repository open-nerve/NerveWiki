import { afterEach, describe, expect, test, vi } from "vitest";

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

/** settling sends a form with options under fake timers; settled() is what its answer settled to, if it has. */
function settling(options: Parameters<typeof uploadFetch>[1] = {}) {
  vi.useFakeTimers();
  const sent = sending(new FormData(), options);
  let settled: unknown = undefined;
  sent.answer.then(
    (response) => (settled = response),
    (error: unknown) => (settled = error)
  );
  return { ...sent, settled: () => settled };
}

describe("uploadFetch", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

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

  test("an answer no Response takes, a status out of 200–599, rejects with a TypeError: it does not hang", async () => {
    const { answer, sent } = sending(new FormData());

    await sent().answer(new Response("{}", { status: 200 }));
    const odd = sending(new FormData());
    Object.defineProperty(odd.sent(), "status", { value: 999 });
    odd.sent().dispatchEvent(new Event("load"));

    expect((await answer).status).toBe(200);
    await expect(odd.answer).rejects.toBeInstanceOf(TypeError);
  });

  test("a network failure rejects with a TypeError, as fetch's", async () => {
    const failed = sending(new FormData());
    failed.sent().fail();
    await expect(failed.answer).rejects.toBeInstanceOf(TypeError);
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

  // A body that goes nowhere, or a server that does not answer it, is given up (M12/P2/S5, 16.3): its result is held,
  // not awaited, so that code that never gives up does not hang the test.
  describe("given up", () => {
    test("a body that goes nowhere for a minute is given up: the transfer is aborted, and it rejects with an UploadTimeout, stalled", async () => {
      const { sent, settled } = settling();

      await vi.advanceTimersByTimeAsync(59_999);
      expect([sent().aborted, settled()]).toEqual([false, undefined]);
      await vi.advanceTimersByTimeAsync(1);

      expect(sent().aborted).toBe(true);
      expect(settled()).toMatchObject({ name: "UploadTimeout", stage: "stalled" });
    });

    test("progress puts the minute off: a body that goes on slowly is not given up", async () => {
      const { sent, settled } = settling();

      await vi.advanceTimersByTimeAsync(50_000);
      sent().progress(10, 100);
      await vi.advanceTimersByTimeAsync(50_000);
      expect([sent().aborted, settled()]).toEqual([false, undefined]);
      await vi.advanceTimersByTimeAsync(10_000);

      expect(sent().aborted).toBe(true);
      expect(settled()).toMatchObject({ name: "UploadTimeout", stage: "stalled" });
    });

    test.each<[string, (transfer: FakeTransfer) => void]>([
      ["its progress all told", (transfer) => transfer.progress(100, 100)],
      ["the upload's load, no progress told", (transfer) => transfer.sentAll()],
    ])(
      "a body all sent waits three minutes for its answer: none comes, it is given up, unanswered (%s)",
      async (_, all) => {
        const { sent, settled } = settling();

        all(sent());
        await vi.advanceTimersByTimeAsync(60_000);
        expect([sent().aborted, settled()]).toEqual([false, undefined]);
        await vi.advanceTimersByTimeAsync(119_999);
        expect([sent().aborted, settled()]).toEqual([false, undefined]);
        await vi.advanceTimersByTimeAsync(1);

        expect(sent().aborted).toBe(true);
        expect(settled()).toMatchObject({ name: "UploadTimeout", stage: "unanswered" });
      }
    );

    test("an answer in time is the answer: nothing is aborted after", async () => {
      const { sent, settled } = settling();

      sent().progress(100, 100);
      await vi.advanceTimersByTimeAsync(179_000);
      await sent().answer(new Response("{}", { status: 201 }));
      // An upload's load later than the answer (Firefox holds it back until the answer) sets no timer again.
      sent().sentAll();
      await vi.advanceTimersByTimeAsync(180_000);

      expect(settled()).toMatchObject({ status: 201 });
      expect(sent().aborted).toBe(false);
    });

    test("the signal stops it as a cancel still: an AbortError, not an UploadTimeout", async () => {
      const stop = new AbortController();
      const { sent, settled } = settling({ signal: stop.signal });

      await vi.advanceTimersByTimeAsync(30_000);
      stop.abort();
      await vi.advanceTimersByTimeAsync(0);

      expect(sent().aborted).toBe(true);
      expect(settled()).toMatchObject({ name: "AbortError" });
    });
  });
});
