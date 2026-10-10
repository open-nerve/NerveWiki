import type {
  ApiClient,
  TransferFailure,
  TransferJob,
  TransferJobDetail,
  TransferJobPage,
  TransferProblem,
} from "@nervewiki/api-client";

import { unwrap } from "./api";
import { uploadFetch, type Transfer, type UploadOptions } from "./upload-fetch";

export type { TransferFailure, TransferJob, TransferJobDetail, TransferJobPage, TransferProblem };
export type { UploadOptions };

/** How many jobs a page of a list has (M7/P5 design 4.2). */
const jobPage = 50;

/**
 * TransferService starts a notebook's imports and exports and reads, lists
 * and cancels its jobs (M7/P5 design 4.2, P6 design 4.1). An export's
 * archive downloads at the address the server signed, without the client:
 * a link does.
 */
export class TransferService {
  constructor(
    private readonly api: ApiClient,
    /** transfer is what an import's upload goes out on: the browser's XMLHttpRequest unless a test says otherwise. */
    private readonly transfer?: () => Transfer
  ) {}

  /** startExport starts the export of the notebook, or of the page rootId and its subtree, and answers its job. */
  async startExport(notebookId: string, rootId: string | null): Promise<TransferJob> {
    return unwrap(
      await this.api.POST("/api/v0/notebooks/{notebook_id}/exports", {
        params: { path: { notebook_id: notebookId } },
        body: { root_id: rootId },
      })
    );
  }

  /**
   * startImport uploads file, a zip, to import into the notebook under the
   * page parent (null: at its root), and answers the import's job. It goes
   * through the session's client, as an attachment's upload does, on an
   * XMLHttpRequest, which tells its progress (uploadFetch); the form's
   * parts in the contract's order: parent_id, file.
   */
  async startImport(
    notebookId: string,
    parent: string | null,
    file: File,
    options: UploadOptions = {}
  ): Promise<TransferJob> {
    const form = new FormData();
    if (parent !== null) {
      form.append("parent_id", parent);
    }
    form.append("file", file, file.name);
    return unwrap(
      await this.api.POST("/api/v0/notebooks/{notebook_id}/imports", {
        params: { path: { notebook_id: notebookId } },
        body: { file: file.name },
        bodySerializer: () => undefined,
        fetch: uploadFetch(form, options, this.transfer),
      })
    );
  }

  /** list answers a page of the notebook's jobs the caller sees, the newest first, jobPage of them, after cursor. */
  async list(notebookId: string, cursor?: string): Promise<TransferJobPage> {
    return unwrap(
      await this.api.GET("/api/v0/notebooks/{notebook_id}/transfer-jobs", {
        params: {
          path: { notebook_id: notebookId },
          query: { limit: jobPage, ...(cursor === undefined ? {} : { cursor }) },
        },
      })
    );
  }

  /** get answers the job id with its report's problems, its address signed anew. */
  async get(id: string): Promise<TransferJobDetail> {
    return unwrap(await this.api.GET("/api/v0/transfer-jobs/{job_id}", { params: { path: { job_id: id } } }));
  }

  /** cancel cancels the job id and answers it: a queued one cancelled, a running one with its cancel asked. */
  async cancel(id: string): Promise<TransferJob> {
    return unwrap(await this.api.POST("/api/v0/transfer-jobs/{job_id}/cancel", { params: { path: { job_id: id } } }));
  }
}
