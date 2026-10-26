import type { TransferJob, TransferJobDetail } from "../services/transfer.service";
import { under as underWayNow } from "../stores/transfer.store";
import { json, notebookJSON, problem, type Answer } from "./fakes";
import { orderedConflicts } from "./history-server";
import { ada, pageServer } from "./page-server";
import { formOf } from "./transfer";

/** The kinds of the jobs that write a notebook's tree, one under way at once (M8/P4 design 3.2). */
const writingKinds: ReadonlySet<TransferJob["kind"]> = new Set(["import", "trash_restore", "revert"]);

/** What the server parses as an id: a parent_id not like it is 400. */
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/iu;

type TransferReport = NonNullable<TransferJob["report"]>;

/**
 * reportOf is a report as the server answers it: failure, and counts, none
 * but those given; no place nor lock, an import's or an export's.
 */
export function reportOf(
  failure: TransferReport["failure"],
  counts: Partial<TransferReport["counts"]> = {}
): TransferReport {
  const none = {
    pages: 0,
    attachments: 0,
    renamed: 0,
    missing: 0,
    skipped: 0,
    restored: 0,
    deleted: 0,
    put_back: 0,
    written: 0,
  };
  return { failure, counts: { ...none, ...counts }, placed: null, lock: null, refusal: null };
}

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
 * job neither; a restore and a revert have their changeset from the start
 * (jobChangesetId), and a revert counts nodes, not pages (M8/P4 design
 * 3.2; P4B review B-N2).
 */
export function jobJSON(n: number, more: Partial<TransferJob> = {}): TransferJob {
  const writes = more.kind === "trash_restore" || more.kind === "revert";
  const job: TransferJob = {
    id: `0199a2b4-0000-7000-8000-0000000007${n.toString().padStart(2, "0")}`,
    notebook_id: notebookJSON.id,
    root_id: null,
    name: notebookJSON.name,
    changeset_id: writes ? jobChangesetId(n) : null,
    target_changeset_id: null,
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
    report: reportOf(null, more.kind === "revert" ? { deleted: 4 } : { pages: 4 }),
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

/** cancelled is job, queued, as its cancel leaves it: cancelled at once, its report counting nothing. */
export function cancelled(job: TransferJob): TransferJob {
  return { ...job, state: "cancelled", finished_at: "2026-10-06T08:00:02Z", report: reportOf(null) };
}

/** jobChangesetId is the id of the changeset of the job n the server started: an import's, a restore's or a revert's. */
export function jobChangesetId(n: number): string {
  return `0199a2b4-0000-7000-8000-0000000c${n.toString().padStart(4, "0")}`;
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
 * them a page whatever limit asks (the store asks 50), the cursor the id
 * of the page's last job: the next page is the jobs after it in the list
 * as it is now, so that a job started since, which goes first, moves no
 * page, as the server's keys of time and id have it; an export started
 * is queued, first, named after the page exported or the notebook; an
 * import started, whose form the app's transfer sent, is queued, first,
 * named after its file, with the changeset it writes (jobChangesetId),
 * under its parent_id, as the server takes it: a
 * parent_id that is no id is 400 bad_request, one that is no page of the
 * notebook 404 page.not_found, and only a form without one imports at the
 * root; a
 * queued job cancelled is cancelled, a running one has its cancel asked,
 * an ended one is 409 transfer.not_cancellable; a job read is its detail,
 * with the problems problems has for it, and a revert's conflicts.
 * While listDown is set, the list cannot be read. It checks no permission
 * and no limit, which a test answers through answers. The test changes
 * the jobs as the server's work would; what went out is in asked.
 *
 * A trash item's restore and a changeset's revert start in the background
 * (M8/P4 design 4.4), as the server's starts answer, in its order, of
 * Plans' trash and history as the page server has them:
 *
 * - a restore: Plans gone 404 notebook.not_found; a reader's 403
 *   forbidden; a node neither alive nor in the trash 404 page.not_found;
 *   one alive 409 page.not_in_trash, one deleted with a page above it 409
 *   page.not_trash_root naming its root, one past its time 409
 *   page.restore_expired; a job that writes the tree under way among the
 *   jobs, or another account's (busy), 409 transfer.busy; the queue full
 *   (queueFull) 503 server_busy; else the restore, queued, first, named
 *   after the item, its root_id the item's root;
 * - a revert: a message empty, past 4,096 bytes or with a NUL 422 on
 *   message; then Plans gone 404 notebook.not_found; a reader's 403
 *   forbidden; a changeset not logged 404 page.changeset_not_found; one
 *   trimmed 409 page.changeset_trimmed; one past the jobs' bound of items
 *   (jobTooLarge) 422 page.revert_too_large; busy as a restore's; then its
 *   conflicts 409 page.revert_conflict as the undo answers them; a version
 *   the purge took (purged) 409 page.changeset_trimmed; a page someone
 *   edits (lockedBy) 409 page.locked; the queue full 503; else the
 *   revert, queued, first, named after the changeset's kind, its
 *   target_changeset_id the changeset, with the message.
 *
 * Each started job has the changeset it writes (jobChangesetId) and is
 * made now, by the server's clock, clockBehind the browser's. While
 * startsHeld is set, a start is answered once released;
 * lost, a start's job is written but its answer lost, a 500 or none at
 * all; listWaits holds the list's reads.
 */
export function jobsServer({ jobs = [], pageSize = 50, answers = {}, ...options }: JobsServerOptions = {}) {
  const state = {
    jobs,
    asked: [] as string[],
    /** While set, the list cannot be read. */
    listDown: false,
    /** Each job's problems and whether more were left out, by its id. */
    problems: new Map<string, Pick<TransferJobDetail, "problems" | "problems_truncated">>(),
    /** Each revert's conflicts and their count, by its id. */
    conflicts: new Map<string, Pick<TransferJobDetail, "conflicts" | "conflict_count">>(),
    /** Whether a job that writes the tree is under way that the jobs listed do not have: another account's. */
    busy: false,
    /** Whether the server's queue is full. */
    queueFull: false,
    /** Whether the starts of restores and reverts wait to be released. */
    startsHeld: false,
    /** How a start's answer is lost once its job is written: a 500, or no answer at all; not lost without it. */
    lost: undefined as "500" | "network" | undefined,
    /** How far the server's clock is behind the browser's, in milliseconds: when it says a job started was made. */
    clockBehind: 0,
    /** What the list's reads wait for while a test holds them. */
    listWaits: undefined as Promise<void> | undefined,
    /** The messages the reverts started were sent with, by their jobs' ids. */
    messages: new Map<string, string | undefined>(),
  };
  const role = options.role ?? "admin";
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
  // A job of the tree under way, or none: transfer.busy, then a queue full.
  const refused = () =>
    server.busy || server.jobs.some((job) => writingKinds.has(job.kind) && underWayNow(job))
      ? problem(409, "transfer.busy")
      : server.queueFull
        ? problem(503, "server_busy", {}, { "Retry-After": "2" })
        : undefined;
  // The job a start writes, queued, first; its answer, or lost.
  const queued = (more: Partial<TransferJob>) => {
    const n = ++started;
    const job = underWay(
      jobJSON(n, {
        changeset_id: jobChangesetId(n),
        created_at: new Date(Date.now() - server.clockBehind).toISOString(),
        ...more,
      }),
      0,
      0
    );
    server.jobs = [job, ...server.jobs];
    switch (server.lost) {
      case "500":
        return problem(500, "internal_error");
      case "network":
        return Promise.reject(new TypeError("offline"));
    }
    return json(job, 202);
  };
  const routes: Record<string, Answer> = {
    [`GET ${plans}/transfer-jobs`]: async (request) => {
      const query = new URL(request.url).searchParams;
      const cursor = query.get("cursor");
      server.asked.push(`GET jobs ${query.get("limit") ?? ""}${cursor === null ? "" : ` after ${cursor}`}`);
      await server.listWaits;
      if (server.listDown) {
        return Promise.reject(new TypeError("offline"));
      }
      const at = cursor === null ? 0 : server.jobs.findIndex((job) => job.id === cursor) + 1;
      if (at === 0 && cursor !== null) {
        // The server reads on from a job gone (its cursor is a key of time and id): a test that needs it says so.
        throw new Error(`no job ${cursor} to read on from`);
      }
      const data = server.jobs.slice(at, at + pageSize);
      const last = data.at(-1);
      return json({
        data,
        next_cursor: last !== undefined && at + pageSize < server.jobs.length ? last.id : null,
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
    [`POST ${plans}/imports`]: (request) => {
      const form = formOf(request);
      const parent = form?.get("parent_id") ?? null;
      const file = form?.get("file");
      const name = file instanceof File ? file.name : "";
      if (parent !== null && (typeof parent !== "string" || !uuid.test(parent))) {
        return problem(400, "bad_request");
      }
      const under = server.nodes.find((node) => node.id === parent && node.kind === "page");
      if (parent !== null && under === undefined) {
        return problem(404, "page.not_found");
      }
      server.asked.push(`POST import ${name} under ${under?.name ?? "root"}`);
      const n = ++started;
      const job = underWay(
        jobJSON(n, {
          kind: "import",
          root_id: typeof parent === "string" ? parent : null,
          name,
          changeset_id: jobChangesetId(n),
          created_at: "2026-10-06T08:00:00Z",
        }),
        0,
        0
      );
      server.jobs = [job, ...server.jobs];
      return json(job, 202);
    },
    [`POST ${plans}/trash-restores`]: async (request) => {
      const { node_id: id } = (await request.clone().json()) as { node_id: string };
      const item = server.trash.get(id);
      const member = [...server.trash].find(([, each]) => each.members.some((node) => node.id === id));
      const named = [...server.nodes, ...[...server.trash.values()].flatMap((each) => each.members)];
      server.asked.push(`POST restore ${named.find((node) => node.id === id)?.name ?? id}`);
      if (server.startsHeld) {
        await new Promise<void>((resolve) => server.held.push(resolve));
      }
      if (server.notebookGone) {
        return problem(404, "notebook.not_found");
      }
      if (role === "reader") {
        return problem(403, "forbidden");
      }
      if (server.nodes.some((node) => node.id === id)) {
        return problem(409, "page.not_in_trash");
      }
      if (item === undefined) {
        return member === undefined
          ? problem(404, "page.not_found")
          : problem(409, "page.not_trash_root", { root_id: member[0] });
      }
      if (item.expired) {
        return problem(409, "page.restore_expired");
      }
      return refused() ?? queued({ kind: "trash_restore", root_id: id, name: item.members[0]?.name ?? "" });
    },
    [`POST ${plans}/reverts`]: async (request) => {
      const { changeset_id: id, message } = (await request.clone().json()) as {
        changeset_id: string;
        message?: string;
      };
      const changeset = server.changesets.find((each) => each.id === id);
      server.asked.push(
        `POST revert ${changeset?.label ?? id}${message === undefined ? "" : ` ${JSON.stringify(message)}`}`
      );
      if (server.startsHeld) {
        await new Promise<void>((resolve) => server.held.push(resolve));
      }
      const bytes = message === undefined ? 0 : new TextEncoder().encode(message).length;
      if (message !== undefined && (bytes === 0 || bytes > 4096 || message.includes("\0"))) {
        const code = bytes === 0 ? "too_short" : bytes > 4096 ? "too_long" : "invalid_format";
        return problem(422, "validation_failed", { errors: [{ field: "message", code, message: "" }] });
      }
      if (server.notebookGone) {
        return problem(404, "notebook.not_found");
      }
      if (role === "reader") {
        return problem(403, "forbidden");
      }
      if (changeset === undefined) {
        return problem(404, "page.changeset_not_found");
      }
      if (changeset.trimmed) {
        return problem(409, "page.changeset_trimmed");
      }
      if (changeset.jobTooLarge) {
        return problem(422, "page.revert_too_large");
      }
      const busy = refused();
      if (busy?.status === 409) {
        return busy;
      }
      if (changeset.conflicts.length > 0) {
        return problem(409, "page.revert_conflict", {
          conflicts: orderedConflicts(changeset.conflicts).slice(0, 100),
          conflict_count: changeset.conflicts.length,
        });
      }
      if (changeset.purged) {
        return problem(409, "page.changeset_trimmed");
      }
      if (changeset.lockedBy !== undefined) {
        const { page, holder } = changeset.lockedBy;
        return problem(409, "page.locked", { lock: { page_id: page, ...holder } });
      }
      if (busy !== undefined) {
        return busy;
      }
      const answer = queued({ kind: "revert", root_id: null, name: changeset.kind, target_changeset_id: id });
      server.messages.set(server.jobs[0]?.id ?? "", message);
      return answer;
    },
    "GET /api/v0/transfer-jobs/*": (request) => {
      const job = jobOf(request);
      server.asked.push(`GET job ${job?.name ?? ""}`);
      return job === undefined
        ? problem(404, "transfer.not_found")
        : json({
            ...job,
            ...(server.problems.get(job.id) ?? { problems: [], problems_truncated: false }),
            ...(server.conflicts.get(job.id) ?? { conflicts: [], conflict_count: 0 }),
          });
    },
    "POST /api/v0/transfer-jobs/*/cancel": (request) => {
      const job = jobOf(request);
      server.asked.push(`CANCEL ${job?.name ?? ""}`);
      if (job === undefined) {
        return problem(404, "transfer.not_found");
      }
      if (job.state === "queued") {
        return json(replace(cancelled(job)));
      }
      return job.state === "running"
        ? json(replace({ ...job, cancel_requested_at: job.cancel_requested_at ?? "2026-10-06T08:00:02Z" }))
        : problem(409, "transfer.not_cancellable");
    },
  };
  const server = Object.assign(pageServer({ ...options, answers: { ...routes, ...answers } }), state);
  return server;
}
