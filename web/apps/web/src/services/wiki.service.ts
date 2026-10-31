import type { ApiClient, LintItem, LintRuleSummary, LintSummary } from "@nervewiki/api-client";

import { unwrap } from "./api";

export type { LintItem, LintRuleSummary };
export type LintRule = LintItem["rule"];

/** The notebook's health checks, computed when the reader asks for them (M10/P3). */
export class WikiService {
  constructor(private readonly api: ApiClient) {}

  async summary(notebookId: string): Promise<LintSummary> {
    return unwrap(
      await this.api.GET("/api/v0/notebooks/{notebook_id}/lint", {
        params: { path: { notebook_id: notebookId } },
      })
    );
  }
}
