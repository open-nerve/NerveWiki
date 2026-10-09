import { expect, test } from "vitest";

import { fakeApi, json, problem } from "../test/fakes";
import { jobJSON } from "../test/jobs-server";
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
  const { service, asked } = serviceOf(() => json(job, 201));

  await expect(service.startExport("n1", "p1")).resolves.toMatchObject({ id: job.id });
  await service.startExport("n1", null);

  expect(asked).toEqual([
    'POST /api/v0/notebooks/n1/exports {"root_id":"p1"}',
    'POST /api/v0/notebooks/n1/exports {"root_id":null}',
  ]);
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
