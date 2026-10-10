import { expect, test } from "vitest";

import { fakeApi, json, problem } from "../test/fakes";
import { jobJSON } from "../test/jobs-server";
import { FakeTransfer, transferTo } from "../test/transfer";
import { ApiError } from "./api";
import { TransferService } from "./transfer.service";

const job = jobJSON(1);

/** A service whose every request is in asked, as "METHOD path?query body", and answered by answer. */
function serviceOf(answer: (request: Request) => Response = () => json(job)) {
  const asked: string[] = [];
  const service = new TransferService(
    fakeApi(async (request) => {
      const url = new URL(request.url);
      const body = request.method === "POST" ? await request.clone().text() : "";
      asked.push(`${request.method} ${url.pathname}${url.search} ${body}`.trim());
      return answer(request);
    })
  );
  return { service, asked };
}

test("startExport posts the root, null for the whole notebook, and answers the job", async () => {
  const { service, asked } = serviceOf(() => json(job, 202));

  await expect(service.startExport("n1", "p1")).resolves.toMatchObject({ id: job.id });
  await service.startExport("n1", null);

  expect(asked).toEqual([
    'POST /api/v0/notebooks/n1/exports {"root_id":"p1"}',
    'POST /api/v0/notebooks/n1/exports {"root_id":null}',
  ]);
});

test("startImport sends parent_id, when it goes under a page, then the file as it is named; it answers the job, a refusal throws the server's problem", async () => {
  let answer = json(jobJSON(2, { kind: "import", name: "Vault.zip" }), 202);
  const transfers = transferTo((request) => {
    expect(`${request.method} ${new URL(request.url).pathname}`).toBe("POST /api/v0/notebooks/n1/imports");
    return answer;
  });
  const service = new TransferService(
    fakeApi(() => problem(500, "internal")),
    transfers
  );

  await expect(service.startImport("n1", "p1", new File(["zip"], "Vault.zip"))).resolves.toMatchObject({
    kind: "import",
  });
  answer = problem(409, "transfer.busy");
  const refused = service.startImport("n1", null, new File(["zip"], "Other.zip"));
  await expect(refused).rejects.toMatchObject({ code: "transfer.busy" });

  const [under, root] = transfers.made.map((transfer) => transfer.body as FormData);
  expect(
    under && [...under.entries()].map(([key, value]) => [key, typeof value === "string" ? value : value.name])
  ).toEqual([
    ["parent_id", "p1"],
    ["file", "Vault.zip"],
  ]);
  expect(root && [...root.keys()]).toEqual(["file"]);
});

test("startImport tells its upload's progress, and stops as its signal aborts", async () => {
  const transfer = new FakeTransfer();
  const service = new TransferService(
    fakeApi(() => problem(500, "internal")),
    () => transfer
  );
  const told: [number, number][] = [];
  const stop = new AbortController();

  const upload = service.startImport("n1", null, new File(["zip"], "Vault.zip"), {
    progress: (sent, total) => told.push([sent, total]),
    signal: stop.signal,
  });
  for (let i = 0; i < 20 && transfer.body === undefined; i++) {
    // oxlint-disable-next-line no-await-in-loop -- the session's middleware goes first
    await Promise.resolve();
  }
  transfer.progress(1, 3);
  stop.abort();

  await expect(upload).rejects.toMatchObject({ name: "AbortError" });
  expect([told, transfer.aborted]).toEqual([[[1, 3]], true]);
});

test("list reads fifty jobs a page, after a cursor when given", async () => {
  const { service, asked } = serviceOf(() => json({ data: [job], next_cursor: "c2" }));

  const first = await service.list("n1");
  await service.list("n1", "c2");

  expect(asked).toEqual([
    "GET /api/v0/notebooks/n1/transfer-jobs?limit=50",
    "GET /api/v0/notebooks/n1/transfer-jobs?limit=50&cursor=c2",
  ]);
  expect(first).toEqual({ data: [job], next_cursor: "c2" });
});

test("get reads the job with its problems; cancel posts to its cancel, without a body", async () => {
  const { service, asked } = serviceOf(() => json({ ...job, problems: [], problems_truncated: false }));

  await expect(service.get("j1")).resolves.toMatchObject({ id: job.id, problems_truncated: false });
  await service.cancel("j1");

  expect(asked).toEqual(["GET /api/v0/transfer-jobs/j1", "POST /api/v0/transfer-jobs/j1/cancel"]);
});

test("a refusal throws the server's problem", async () => {
  const { service } = serviceOf(() => problem(409, "transfer.busy"));

  await expect(service.startExport("n1", null)).rejects.toSatisfy(
    (error) => error instanceof ApiError && error.problem?.code === "transfer.busy"
  );
});
