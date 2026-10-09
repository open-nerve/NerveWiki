import type { ApiClient, TransferJob, TransferJobDetail } from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import { bearer } from "./auth";
import { unzip, type ZipEntry } from "./zip";

// The exports and imports of the stories, through the API (M7/P5 design
// 3.16, P6 design 3.18): a job started, read until it ends; an export's
// archive downloaded at the address the server signed, without a token;
// an import's sent as multipart/form-data, which the client sends as a
// FormData.

/** credential's start of an export of the notebook notebookId, or of its page rootId and its subtree. */
async function postExport(api: ApiClient, credential: string, notebookId: string, rootId?: string) {
  return api.POST("/api/v0/notebooks/{notebook_id}/exports", {
    params: { path: { notebook_id: notebookId } },
    body: rootId === undefined ? {} : { root_id: rootId },
    headers: bearer(credential),
  });
}

/** Starts credential's export of the notebook notebookId, or of its page rootId, and returns its job, queued. */
export async function startExport(
  api: ApiClient,
  credential: string,
  notebookId: string,
  rootId?: string
): Promise<TransferJob> {
  const { data, error, response } = await postExport(api, credential, notebookId, rootId);
  expect(response.status, `export: ${JSON.stringify(error)}`).toBe(202);
  if (!data) {
    throw new Error("export answered 202 without the job");
  }
  return data;
}

/** credential's import of archive, a zip named name, into the notebook notebookId, under parentId or at its root. */
export async function postImport(
  api: ApiClient,
  credential: string,
  notebookId: string,
  archive: Uint8Array,
  parentId?: string,
  name = "vault.zip"
) {
  const form = new FormData();
  if (parentId !== undefined) {
    form.append("parent_id", parentId);
  }
  form.append("file", new Blob([Buffer.from(archive)], { type: "application/zip" }), name);
  return api.POST("/api/v0/notebooks/{notebook_id}/imports", {
    params: { path: { notebook_id: notebookId } },
    body: { file: name },
    bodySerializer: () => form,
    headers: bearer(credential),
  });
}

/** Starts credential's import of archive into the notebook notebookId, under parentId, and returns its job, queued. */
export async function startImport(
  api: ApiClient,
  credential: string,
  notebookId: string,
  archive: Uint8Array,
  parentId?: string
): Promise<TransferJob> {
  const { data, error, response } = await postImport(api, credential, notebookId, archive, parentId);
  expect(response.status, `import: ${JSON.stringify(error)}`).toBe(202);
  if (!data) {
    throw new Error("import answered 202 without the job");
  }
  return data;
}

/** credential's read of the job id, as the API answers it. */
async function getJob(api: ApiClient, credential: string, id: string) {
  return api.GET("/api/v0/transfer-jobs/{job_id}", { params: { path: { job_id: id } }, headers: bearer(credential) });
}

/** The job id once it has ended, as credential reads it. */
export async function endedJob(api: ApiClient, credential: string, id: string): Promise<TransferJobDetail> {
  let job: TransferJobDetail | undefined;
  await expect
    .poll(
      async () => {
        const { data, response } = await getJob(api, credential, id);
        expect(response.status).toBe(200);
        job = data;
        return job?.state;
      },
      { message: `the job ${id} ends`, timeout: 15_000 }
    )
    .not.toMatch(/^(queued|running)$/);
  if (!job) {
    throw new Error(`no job ${id}`);
  }
  return job;
}

/** The notebook notebookId's jobs, as credential lists them, the newest first. */
export async function listJobs(api: ApiClient, credential: string, notebookId: string): Promise<TransferJob[]> {
  const { data, error, response } = await api.GET("/api/v0/notebooks/{notebook_id}/transfer-jobs", {
    params: { path: { notebook_id: notebookId } },
    headers: bearer(credential),
  });
  expect(response.status, `list the jobs: ${JSON.stringify(error)}`).toBe(200);
  return data?.data ?? [];
}

/** The answer to a GET of address, an archive as the server signed it, at baseURL, without a token. */
export function downloadArchive(baseURL: string, address: string): Promise<Response> {
  return fetch(new URL(address, baseURL));
}

/** The entries of the archive at address, its answer 200 and a zip. */
export async function archiveAt(baseURL: string, address: string): Promise<{ entries: ZipEntry[]; bytes: number }> {
  const answer = await downloadArchive(baseURL, address);
  expect(answer.status, `download ${address}`).toBe(200);
  expect(answer.headers.get("content-type")).toBe("application/zip");
  const buf = Buffer.from(await answer.arrayBuffer());
  return { entries: unzip(buf), bytes: buf.length };
}
