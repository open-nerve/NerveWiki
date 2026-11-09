import type { ApiClient, Upgrade } from "@nervewiki/api-client";

import type { Locale } from "../i18n/locale";
import { unwrap } from "./api";

export type { Upgrade };

/**
 * UpgradeService starts a notebook's upgrade to an LLM Wiki and reads it,
 * for its admins (M11/P1 design 4.4). Each change answers the upgrade.
 */
export class UpgradeService {
  constructor(private readonly api: ApiClient) {}

  /** start starts upgrading the standard notebook id, the pages it lacks written in locale. */
  async start(notebookId: string, locale: Locale): Promise<Upgrade> {
    return unwrap(
      await this.api.POST("/api/v0/notebooks/{notebook_id}/upgrade", {
        params: { path: { notebook_id: notebookId } },
        body: { template_locale: locale },
      })
    );
  }

  /** get reads the notebook's upgrade under way: wiki.upgrade_not_found without one. */
  async get(notebookId: string, signal?: AbortSignal): Promise<Upgrade> {
    return unwrap(
      await this.api.GET("/api/v0/notebooks/{notebook_id}/upgrade", {
        params: { path: { notebook_id: notebookId } },
        signal,
      })
    );
  }
}
