import type { TransferJob, TransferJobDetail } from "../services/transfer.service";
import { json, notebookJSON, problem, type Answer } from "./fakes";
import { ada, pageServer } from "./page-server";

/** addressOf is the address the server signed for job's archive, which expires at expires. */
export function addressOf(job: TransferJob, expires = "2100-01-01T00:00:00Z"): NonNullable<TransferJob["download"]> {
  return { url: `/api/v0/transfer-jobs/${job.id}/download?e=${Date.parse(expires) / 1000}&s=sig`, expires_at: expires };
}

/**
 * jobJSON is the job n of Plans: Ada's export of the whole notebook, which
 * succeeded, its four pages written, created, started and ended at hours
 * apart; more changes what it says. As the server answers it (v0.1 design
 * 13.4, item 7), a succeeded export has its size and an address (expiring
 * far off unless more gives one), an expired one its size alone, any other
 * job neither.
 */
export function jobJSON(n: number, more: Partial<TransferJob> = {}): TransferJob {
  const job: TransferJob = {
    id: `0199a2b4-0000-7000-8000-0000000007${n.toString().padStart(2, "0")}`,
    notebook_id: notebookJSON.id,
    root_id: null,
    name: notebookJSON.name,
    kind: "export",
    state: "succeeded",
    client: "web",
    created_by: ada,
    created_at: "2026-10-05T08:00:00Z",
    started_at: "2026-10-05T09:10:00Z",
    cancel_requested_at: null,
    finished_at: "2026-10-05T10:20:00Z",
    progress: { done: 4, total: 4 },
    result_bytes: null,
    report: { failure: null, counts: { pages: 4, attachments: 0, renamed: 0, missing: 0, skipped: 0 } },
    download: null,
    ...more,
  };
  const exported = job.kind === "export" && (job.state === "succeeded" || job.state === "expired");
  return {
    ...job,
    result_bytes: "result_bytes" in more ? job.result_bytes : exported ? 2048 : null,
    download: "download" in more || !exported || job.state !== "succeeded" ? job.download : addressOf(job),
  };
}

/** underWay is job as it runs, done of total written; queued, as it waits, when total is 0. */
export function underWay(job: TransferJob, done: number, total: number): TransferJob {
  return {
    ...job,
    state: total === 0 ? "queued" : "running",
    started_at: total === 0 ? null : job.started_at,
    finished_at: null,
    progress: { done, total },
    result_bytes: null,
    report: null,
    download: null,
  };
}

type JobsServerOptions = NonNullable<Parameters<typeof pageServer>[0]> & {
  /** Plans' jobs that Ada sees, the newest first. */
  jobs?: TransferJob[];
  /** How many jobs a page of the list has. */
  pageSize?: number;
};

/**
 * jobsServer is pageServer with Plans' imports and exports as Ada sees
 * them (M7/P5 design 4.2): its jobs listed the newest first, pageSize of
 * them a page whatever limit asks (the store asks 50), the cursor the
 * index of the page's first, not the server's key of the last job before
 * it: a job started before the cursor shifts the pages; an export started
 * is queued, first, named after the page exported or the notebook; a
 * queued job cancelled is cancelled, a running one has its cancel asked,
 * an ended one is 409 transfer.not_cancellable; a job read is its detail,
 * with the problems problems has for it. While listDown is set, the list
 * cannot be read. It checks no permission and no
 * limit, which a test answers through answers. The test changes the jobs
 * as the server's work would; what went out is in asked.
 */
export function jobsServer({ jobs = [], pageSize = 50, answers = {}, ...options }: JobsServerOptions = {}) {
  const state = {
    jobs,
    asked: [] as string[],
    /** While set, the list cannot be read. */
    listDown: false,
    /** Each job's problems and whether more were left out, by its id. */
    problems: new Map<string, Pick<TransferJobDetail, "problems" | "problems_truncated">>(),
  };
  let started = 50;
  const plans = `/api/v0/notebooks/${notebookJSON.id}`;
  const jobOf = (request: Request) => {
    const id = new URL(request.url).pathname.split("/")[4];
    return server.jobs.find((job) => job.id === id);
  };
  const replace = (job: TransferJob) => {
    server.jobs = server.jobs.map((held) => (held.id === job.id ? job : held));
    return job;
  };
  const routes: Record<string, Answer> = {
    [`GET ${plans}/transfer-jobs`]: (request) => {
      const query = new URL(request.url).searchParams;
      const cursor = query.get("cursor");
      server.asked.push(`GET jobs ${query.get("limit") ?? ""}${cursor === null ? "" : ` after ${cursor}`}`);
      if (server.listDown) {
        return Promise.reject(new TypeError("offline"));
      }
      const at = cursor === null ? 0 : Number(cursor);
      const next = at + pageSize;
      return json({
        data: server.jobs.slice(at, next),
        next_cursor: next < server.jobs.length ? next.toString() : null,
      });
    },
    [`POST ${plans}/exports`]: async (request) => {
      const { root_id: rootId } = (await request.clone().json()) as { root_id: string | null };
      const root = server.nodes.find((node) => node.id === rootId);
      server.asked.push(`POST export ${root?.name ?? "notebook"}`);
      const job = underWay(
        jobJSON(++started, {
          root_id: rootId,
          name: root?.name ?? notebookJSON.name,
          created_at: "2026-10-06T08:00:00Z",
        }),
        0,
        0
      );
      server.jobs = [job, ...server.jobs];
      return json(job, 202);
    },
    "GET /api/v0/transfer-jobs/*": (request) => {
      const job = jobOf(request);
      server.asked.push(`GET job ${job?.name ?? ""}`);
      return job === undefined
        ? problem(404, "transfer.not_found")
        : json({ ...job, ...(server.problems.get(job.id) ?? { problems: [], problems_truncated: false }) });
    },
    "POST /api/v0/transfer-jobs/*/cancel": (request) => {
      const job = jobOf(request);
      server.asked.push(`CANCEL ${job?.name ?? ""}`);
      if (job === undefined) {
        return problem(404, "transfer.not_found");
      }
      if (job.state === "queued") {
        const counts = { pages: 0, attachments: 0, renamed: 0, missing: 0, skipped: 0 };
        return json(
          replace({
            ...job,
            state: "cancelled",
            finished_at: "2026-10-06T08:00:02Z",
            report: { failure: null, counts },
          })
        );
      }
      return job.state === "running"
        ? json(replace({ ...job, cancel_requested_at: job.cancel_requested_at ?? "2026-10-06T08:00:02Z" }))
        : problem(409, "transfer.not_cancellable");
    },
  };
  const server = Object.assign(pageServer({ ...options, answers: { ...routes, ...answers } }), state);
  return server;
}
