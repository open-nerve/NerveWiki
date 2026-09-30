import { describe, expect, test } from "vitest";

import { fakeApi, instanceJSON, json } from "../test/fakes";
import { ApiError } from "./api";
import { InstanceService } from "./instance.service";

describe("InstanceService.get", () => {
  test("asks GET /api/v0/instance and returns the information", async () => {
    let asked = "";
    const service = new InstanceService(
      fakeApi((request) => {
        asked = `${request.method} ${new URL(request.url).pathname}`;
        return json(instanceJSON);
      })
    );

    expect(await service.get()).toEqual(instanceJSON);
    expect(asked).toBe("GET /api/v0/instance");
  });

  test("throws a problem as an ApiError with its status and code", async () => {
    const problem = { status: 503, code: "server_busy", title: "Service Unavailable", detail: "The server is busy." };
    const service = new InstanceService(fakeApi(() => json(problem, 503, "application/problem+json")));

    const error = await service.get().catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: 503, code: "server_busy", message: "The server is busy." });
  });

  test("throws an answer that is not a problem as an ApiError without code", async () => {
    const service = new InstanceService(fakeApi(() => new Response("<html>Bad Gateway</html>", { status: 502 })));

    await expect(service.get()).rejects.toMatchObject({ status: 502, code: undefined, message: "HTTP 502" });
  });

  test("lets a network failure through", async () => {
    const service = new InstanceService(
      fakeApi(() => {
        throw new TypeError("Failed to fetch");
      })
    );

    await expect(service.get()).rejects.toThrow(TypeError);
  });
});
