import type {
  ApiClient,
  LintPage,
  LintRule,
  Upgrade,
  UpgradeCheck,
  UpgradeCounts,
  UpgradeGates,
  UpgradeReport,
  UpgradeReportItem,
} from "@nervewiki/api-client";

import type { Locale } from "../i18n/locale";
import { unwrap } from "./api";

export type { Upgrade, UpgradeCheck, UpgradeCounts, UpgradeGates, UpgradeReport, UpgradeReportItem };

/**
 * UpgradeService starts a notebook's upgrade to an LLM Wiki, reads it,
 * abandons it and ends its stopped rollback (M11/P1 design 4.4), checks it,
 * reads its rules' items and submits it for review (M11/P2 design 4.3),
 * approves it, rejects it or sends it back (M11/P3 design 4.3), for its
 * admins. Each change answers the upgrade.
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

  /**
   * checkItems reads the page of rule's items of the upgrade's check after
   * cursor, a check's or a page's; without one, its first page, as a
   * stored report's More does (M11/P3 design 4.3).
   */
  async checkItems(
    notebookId: string,
    rule: LintRule,
    cursor: string | undefined,
    signal?: AbortSignal
  ): Promise<LintPage> {
    return unwrap(
      await this.api.GET("/api/v0/notebooks/{notebook_id}/upgrade/check/{rule}", {
        params: { path: { notebook_id: notebookId, rule }, query: cursor === undefined ? {} : { cursor } },
        signal,
      })
    );
  }

  /**
   * submit submits the notebook's upgrade, upgrading, for review: it is
   * reviewing at once, checked, and answered reviewing with its report
   * once every gate passes; else upgrading again, refused
   * (wiki.upgrade_not_ready, wiki.upgrade_stale_index, the check's error).
   */
  async submit(notebookId: string): Promise<Upgrade> {
    return unwrap(
      await this.api.POST("/api/v0/notebooks/{notebook_id}/upgrade/submit", {
        params: { path: { notebook_id: notebookId } },
      })
    );
  }

  /**
   * approve approves the notebook's upgrade in review, its report kept: the
   * notebook is an LLM Wiki, and the upgrade, approved, is no longer under
   * way (wiki.upgrade_state without a report; wiki.upgrade_not_ready, a
   * gate checked again not passing, still in review; server_busy past its
   * time, nothing written).
   */
  async approve(notebookId: string): Promise<Upgrade> {
    return unwrap(
      await this.api.POST("/api/v0/notebooks/{notebook_id}/upgrade/approve", {
        params: { path: { notebook_id: notebookId } },
      })
    );
  }

  /** reject rolls the notebook's upgrade in review back, rejected, in a background job, as an abandonment does. */
  async reject(notebookId: string): Promise<Upgrade> {
    return unwrap(
      await this.api.POST("/api/v0/notebooks/{notebook_id}/upgrade/reject", {
        params: { path: { notebook_id: notebookId } },
      })
    );
  }

  /**
   * sendBack sends the notebook's upgrade in review back, upgrading, its
   * submission and report cleared; one whose submission is still checked is
   * wiki.upgrade_state.
   */
  async sendBack(notebookId: string): Promise<Upgrade> {
    return unwrap(
      await this.api.POST("/api/v0/notebooks/{notebook_id}/upgrade/send-back", {
        params: { path: { notebook_id: notebookId } },
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
