import { describe, expect, test } from "vitest";

import { assetJSON, assetNode } from "../test/page-server";
import { fakeApi, json, problem, signedInApp, tokensJSON } from "../test/fakes";
import { FakeTransfer, formOf, transferTo } from "../test/transfer";
import { ApiError } from "./api";
import { AssetService } from "./asset.service";

const picture = assetNode(70, "a.png");

describe("AssetService.list", () => {
  test("reads a hundred attachments under a page, after a cursor; at the root, with neither", async () => {
    const asked: string[] = [];
    const service = new AssetService(
      fakeApi((request) => {
        const url = new URL(request.url);
        asked.push(`${url.pathname}${url.search}`);
        return json({ data: [assetJSON(picture)], next_cursor: null });
      })
    );

    await service.list("n1", "p1", "c1");
    const page = await service.list("n1", null);

    expect(asked).toEqual([
      "/api/v0/notebooks/n1/assets?parent_id=p1&limit=100&cursor=c1",
      "/api/v0/notebooks/n1/assets?limit=100",
    ]);
    expect(page.data.map((asset) => asset.name)).toEqual(["a.png"]);
  });
});

describe("AssetService.upload", () => {
  test("sends parent_id, name and file, in the contract's order, the file named as it is sent", async () => {
    const transfers = transferTo(() => json(assetJSON(picture), 201));
    const service = new AssetService(
      fakeApi(() => problem(500, "internal")),
      transfers
    );

    await service.upload("n1", { parent: "p1", name: "a.png", file: new Blob(["x"]) });
    await service.upload("n1", { parent: null, name: "b.png", file: new Blob(["y"]) });

    const [under, root] = transfers.made.map((transfer) => transfer.body as FormData);
    expect(under && [...under.keys()]).toEqual(["parent_id", "name", "file"]);
    expect(under?.get("parent_id")).toBe("p1");
    expect(under?.get("name")).toBe("a.png");
    expect(under?.get("file")).toMatchObject({ name: "a.png" });
    expect(root && [...root.keys()]).toEqual(["name", "file"]);
  });

  test("answers the attachment; a refusal throws the server's problem", async () => {
    let answer = json(assetJSON(picture), 201);
    const service = new AssetService(
      fakeApi(() => problem(500, "internal")),
      transferTo(() => answer)
    );

    await expect(service.upload("n1", { parent: null, name: "a.png", file: new Blob([]) })).resolves.toMatchObject({
      id: picture.id,
    });
    answer = problem(409, "page.title_taken");
    const refused = service.upload("n1", { parent: null, name: "a.png", file: new Blob([]) });
    await expect(refused).rejects.toBeInstanceOf(ApiError);
    await expect(refused).rejects.toMatchObject({ code: "page.title_taken" });
  });

  test("goes through the session's client: its token, renewed after a 401 and sent again with the whole form", async () => {
    const tokens: string[] = [];
    let refreshes = 0;
    const app = signedInApp({
      "POST /api/v0/auth/refresh": () => json({ ...tokensJSON, access_token: `at-${(++refreshes).toString()}` }),
      "POST /api/v0/notebooks/n1/assets": (request) => {
        tokens.push(`${request.headers.get("Authorization")} ${String(formOf(request)?.get("name"))}`);
        return tokens.length === 1 ? problem(401, "unauthorized") : json(assetJSON(picture), 201);
      },
    });
    await app.session.start();
    const service = new AssetService(app.session.clientFor("login-0"), app.transfer);

    await expect(service.upload("n1", { parent: null, name: "a.png", file: new Blob(["x"]) })).resolves.toMatchObject({
      id: picture.id,
    });

    expect(tokens).toEqual(["Bearer at-1 a.png", "Bearer at-2 a.png"]);
  });

  test("tells its progress, and stops as its signal aborts", async () => {
    const transfer = new FakeTransfer();
    const service = new AssetService(
      fakeApi(() => problem(500, "internal")),
      () => transfer
    );
    const told: number[] = [];
    const stop = new AbortController();

    const upload = service.upload(
      "n1",
      { parent: null, name: "a.png", file: new Blob(["x"]) },
      { progress: (sent) => told.push(sent), signal: stop.signal }
    );
    await expectSent(transfer);
    transfer.progress(1, 2);
    stop.abort();

    await expect(upload).rejects.toMatchObject({ name: "AbortError" });
    expect(told).toEqual([1]);
  });
});

/** expectSent waits for transfer to be sent: the middleware's turn goes first. */
async function expectSent(transfer: FakeTransfer): Promise<void> {
  for (let i = 0; i < 20 && transfer.body === undefined; i++) {
    // oxlint-disable-next-line no-await-in-loop -- a turn at a time
    await Promise.resolve();
  }
  expect(transfer.body).toBeInstanceOf(FormData);
}
