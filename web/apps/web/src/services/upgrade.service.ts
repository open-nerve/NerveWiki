import type {
  ApiClient,
  LintPage,
  LintRule,
  Upgrade,
  UpgradeCheck,
  UpgradeCounts,
  UpgradeGates,
  UpgradeReportItem,
} from "@nervewiki/api-client";

import type { Locale } from "../i18n/locale";
import { unwrap } from "./api";

export type { Upgrade, UpgradeCheck, UpgradeCounts, UpgradeGates, UpgradeReportItem };

/**
 * UpgradeService starts a notebook's upgrade to an LLM Wiki, reads it,
 * abandons it and ends its stopped rollback (M11/P1 design 4.4), checks it
 * and reads its rules' items (M11/P2 design 4.3), for its admins. Each
 * change answers the upgrade.
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

  /**
   * abandon rolls the notebook's upgrade back, upgrading or reviewing, or
   * continues its stopped rollback, in a background job.
   */
  async abandon(notebookId: string): Promise<Upgrade> {
    return unwrap(
      await this.api.POST("/api/v0/notebooks/{notebook_id}/upgrade/abandon", {
        params: { path: { notebook_id: notebookId } },
      })
    );
  }

  /** endRollback ends the notebook's stopped rollback, keeping it as it is: the upgrade ends, rolled back. */
  async endRollback(notebookId: string): Promise<Upgrade> {
    return unwrap(
      await this.api.POST("/api/v0/notebooks/{notebook_id}/upgrade/end-rollback", {
        params: { path: { notebook_id: notebookId } },
      })
    );
  }

  /** check checks the notebook's upgrade, upgrading or reviewing, against an LLM Wiki's contract: passed or not. */
  async check(notebookId: string, signal?: AbortSignal): Promise<UpgradeCheck> {
    return unwrap(
      await this.api.GET("/api/v0/notebooks/{notebook_id}/upgrade/check", {
        params: { path: { notebook_id: notebookId } },
        signal,
      })
    );
  }

  /** checkItems reads the page of rule's items of the upgrade's check after cursor, a check's or a page's. */
  async checkItems(notebookId: string, rule: LintRule, cursor: string, signal?: AbortSignal): Promise<LintPage> {
    return unwrap(
      await this.api.GET("/api/v0/notebooks/{notebook_id}/upgrade/check/{rule}", {
        params: { path: { notebook_id: notebookId, rule }, query: { cursor } },
        signal,
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
