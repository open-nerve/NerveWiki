import type { ApiClient, Asset, AssetPage } from "@nervewiki/api-client";

import { unwrap } from "./api";
import { uploadFetch, type Transfer, type UploadOptions } from "./upload-fetch";

export type { Asset, AssetPage };
export type { UploadOptions };

/** AssetUpload is a file to upload as an attachment: under parent (null: the notebook's root), named name. */
export type AssetUpload = { parent: string | null; name: string; file: Blob };

/**
 * AssetService reads a notebook's attachments and uploads them (M7/P4
 * design 3.2). An attachment is renamed, moved and deleted as a node is,
 * through the page tree's writes.
 */
export class AssetService {
  constructor(
    private readonly api: ApiClient,
    /** transfer is what an upload goes out on: the browser's XMLHttpRequest unless a test says otherwise. */
    private readonly transfer?: () => Transfer
  ) {}

  /** list answers a page of the attachments under parent (null: the root), by name, after cursor. */
  async list(notebookId: string, parent: string | null, cursor?: string): Promise<AssetPage> {
    return unwrap(
      await this.api.GET("/api/v0/notebooks/{notebook_id}/assets", {
        params: {
          path: { notebook_id: notebookId },
          query: { ...(parent === null ? {} : { parent_id: parent }), ...(cursor === undefined ? {} : { cursor }) },
        },
      })
    );
  }

  /** get answers the attachment id, its addresses signed anew. */
  async get(id: string): Promise<Asset> {
    return unwrap(await this.api.GET("/api/v0/assets/{node_id}", { params: { path: { node_id: id } } }));
  }

  /**
   * upload uploads upload into the notebook and answers the attachment.
   * It goes through the session's client, the token on it, renewed and
   * sent again on a 401, on an XMLHttpRequest, which tells its progress
   * (uploadFetch); the form's parts in the contract's order: parent_id,
   * name, file. The request the client builds has no body, which its
   * middleware would copy: the transfer sends the form.
   */
  async upload(notebookId: string, upload: AssetUpload, options: UploadOptions = {}): Promise<Asset> {
    const form = new FormData();
    if (upload.parent !== null) {
      form.append("parent_id", upload.parent);
    }
    form.append("name", upload.name);
    form.append("file", upload.file, upload.name);
    return unwrap(
      await this.api.POST("/api/v0/notebooks/{notebook_id}/assets", {
        params: { path: { notebook_id: notebookId } },
        body: { file: upload.name },
        bodySerializer: () => undefined,
        fetch: uploadFetch(form, options, this.transfer),
      })
    );
  }
}
