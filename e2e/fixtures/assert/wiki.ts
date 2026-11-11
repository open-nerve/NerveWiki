import type { Upgrade } from "@nervewiki/api-client";
import { expect } from "@playwright/test";

import type { Database } from "../db";

// Database assertions of the upgrade stories (M11/P1 design 5): the wiki
// module's row of an upgrade and its notebook's columns. The times are
// compared in SQL, to the microsecond.

/**
 * notebook_upgrades, notebooks: upgrade's row is as the API answered it, its state, ending, rollback's job, source
 * root, the pages its start wrote and its baseline's three numbers, started by its initiator at its time; while it is
 * under way its notebook's four columns name it, its initiator and its time. Its submission's submitter and time are
 * the answer's, a submission's id kept exactly when the answer has its time (approved, rejected or abandoned, an upgrade
 * keeps its submission: M11/P2/S2 review C-M3); its report is kept when the answer has one,
 * its passed, gates and counts the answer's (M11/P2 design 5: its time and items are the report's encoding's, which
 * the server's tests hold).
 */
export async function expectUpgrade(db: Database, upgrade: Upgrade): Promise<void> {
  const rows = await db.query(
    `SELECT u.notebook_id, u.state, u.ending, u.rollback_job_id, u.rollback_complete,
            u.submitted_by_id, u.submitted_at IS NOT DISTINCT FROM $3::timestamptz AS submitted_at_answer,
            (u.submission_id IS NOT NULL) = ($3::timestamptz IS NOT NULL) AS submission_answer,
            CASE WHEN u.report IS NOT NULL THEN jsonb_build_object('passed', u.report->'passed', 'gates', u.report->'gates',
              'counts', u.report->'counts') END AS report,
            CASE WHEN u.rollback_left_nodes IS NOT NULL THEN jsonb_build_object('nodes', u.rollback_left_nodes,
              'pages', u.rollback_left_pages, 'created', u.rollback_left_created) END AS rollback_left, u.source_root_id,
            ARRAY(SELECT s.x FROM unnest(ARRAY[CASE WHEN u.seeded_root THEN u.source_root_id END, u.seeded_agents_id])
                    WITH ORDINALITY AS s(x, o) WHERE s.x IS NOT NULL ORDER BY s.o)::text[] AS seeded,
            u.baseline_nodes, u.baseline_pages, u.baseline_wanted, u.created_by_id,
            u.created_at = $2::timestamptz AS at_answer,
            n.upgrade_id IS NOT DISTINCT FROM CASE WHEN u.state IN ('upgrading', 'reviewing', 'rolling_back') THEN u.id END
              AND n.upgrade_state IS NOT DISTINCT FROM CASE WHEN n.upgrade_id = u.id THEN u.state END
              AND n.upgrade_by_id IS NOT DISTINCT FROM CASE WHEN n.upgrade_id = u.id THEN u.created_by_id END
              AND n.upgrade_at IS NOT DISTINCT FROM CASE WHEN n.upgrade_id = u.id THEN u.created_at END AS notebook_names_it
       FROM notebook_upgrades u JOIN notebooks n ON n.id = u.notebook_id WHERE u.id = $1`,
    [upgrade.id, upgrade.started_at, upgrade.submitted_at]
  );
  expect(rows).toEqual([
    {
      notebook_id: upgrade.notebook_id,
      state: upgrade.state,
      ending: upgrade.ending,
      rollback_job_id: upgrade.rollback_job_id,
      rollback_complete: upgrade.rollback_complete,
      submitted_by_id: upgrade.submitted_by?.user_id ?? null,
      submitted_at_answer: true,
      submission_answer: true,
      report:
        upgrade.report === null
          ? null
          : { passed: upgrade.report.passed, gates: upgrade.report.gates, counts: upgrade.report.counts },
      // Rolling back, what is left is counted as it is read, not kept.
      rollback_left: upgrade.state === "rolled_back" ? upgrade.rollback_left : null,
      source_root_id: upgrade.source_root_id,
      seeded: upgrade.seeded_page_ids,
      baseline_nodes: upgrade.baseline.nodes,
      baseline_pages: upgrade.baseline.pages,
      baseline_wanted: upgrade.baseline.wanted,
      created_by_id: upgrade.started_by.user_id,
      at_answer: true,
      notebook_names_it: true,
    },
  ]);
}

/** notebook_upgrades: the upgrades of the notebook id, under way or ended, by state. */
export async function upgradesOf(db: Database, id: string): Promise<string[]> {
  const rows = await db.query<{ state: string }>(
    "SELECT state FROM notebook_upgrades WHERE notebook_id = $1 ORDER BY created_at, id",
    [id]
  );
  return rows.map((row) => row.state);
}
