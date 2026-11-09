-- notebook_upgrades: a notebook's upgrades to an LLM Wiki (M11 design 4.2, 4.3), the wiki module's first table: who
-- started it and when, the source root it proposes, the pages its start wrote, its baseline's counts, its submission
-- and the report kept, its rollback's job, its state, ending and whether its rollback was done. A notebook has at
-- most one under way. The ones that ended stay, soft-deleted with their notebook and purged after it.

-- +goose Up
CREATE TABLE notebook_upgrades (
    id uuid NOT NULL,
    notebook_id uuid NOT NULL REFERENCES notebooks ON DELETE RESTRICT,
    state text NOT NULL,
    ending text,
    source_root_id uuid NOT NULL,
    seeded_root boolean NOT NULL,
    seeded_agents_id uuid,
    locale text NOT NULL,
    baseline_nodes integer NOT NULL,
    baseline_pages integer NOT NULL,
    baseline_wanted integer NOT NULL,
    baseline_wanted_capped boolean NOT NULL,
    submission_id uuid,
    submitted_by_id uuid REFERENCES users,
    submitted_at timestamptz,
    report jsonb,
    rollback_job_id uuid,
    rollback_complete boolean,
    rollback_left_nodes integer,
    rollback_left_pages integer,
    rollback_left_created integer,
    created_by_id uuid NOT NULL REFERENCES users,
    updated_by_id uuid NOT NULL REFERENCES users,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    deleted_at timestamptz,
    CONSTRAINT notebook_upgrades_pkey PRIMARY KEY (id),
    CONSTRAINT notebook_upgrades_state_check
        CHECK (state IN ('upgrading', 'reviewing', 'rolling_back', 'approved', 'rolled_back')),
    CONSTRAINT notebook_upgrades_ending_check CHECK (ending IN ('abandoned', 'rejected')),
    CONSTRAINT notebook_upgrades_locale_check CHECK (locale IN ('zh-CN', 'en')),
    -- baseline_nodes >= 0 follows from 0 <= baseline_pages <= baseline_nodes, kept to be read.
    CONSTRAINT notebook_upgrades_baseline_check CHECK (baseline_nodes >= 0 AND baseline_pages BETWEEN 0 AND baseline_nodes
        AND baseline_wanted BETWEEN 0 AND 10000 AND baseline_wanted_capped = (baseline_wanted = 10000)),
    CONSTRAINT notebook_upgrades_ending_state_check
        CHECK ((state IN ('rolling_back', 'rolled_back')) = (ending IS NOT NULL)),
    CONSTRAINT notebook_upgrades_submission_check CHECK ((submission_id IS NULL) = (submitted_at IS NULL)
        AND (submitted_at IS NULL) = (submitted_by_id IS NULL)),
    -- 'approved' in the second conjunct follows from approved_check with report_check, kept to be read.
    CONSTRAINT notebook_upgrades_submission_state_check CHECK ((state <> 'upgrading' OR submission_id IS NULL)
        AND (state NOT IN ('reviewing', 'approved') OR submission_id IS NOT NULL)),
    -- state <> 'upgrading' follows from submission_id IS NOT NULL with submission_state_check, kept to be read.
    CONSTRAINT notebook_upgrades_report_check CHECK (report IS NULL OR submission_id IS NOT NULL AND state <> 'upgrading'
        AND jsonb_typeof(report) = 'object' AND octet_length(report::text) <= 65536),
    CONSTRAINT notebook_upgrades_approved_check CHECK (state <> 'approved' OR report IS NOT NULL),
    CONSTRAINT notebook_upgrades_rejected_check CHECK (ending IS DISTINCT FROM 'rejected' OR submission_id IS NOT NULL),
    CONSTRAINT notebook_upgrades_rollback_job_check
        CHECK ((rollback_job_id IS NOT NULL) = (state IN ('rolling_back', 'rolled_back'))),
    CONSTRAINT notebook_upgrades_rollback_end_check CHECK ((rollback_complete IS NOT NULL) = (state = 'rolled_back')
        AND (rollback_complete IS DISTINCT FROM false) = (rollback_left_nodes IS NULL)
        AND (rollback_left_nodes IS NULL) = (rollback_left_pages IS NULL)
        AND (rollback_left_nodes IS NULL) = (rollback_left_created IS NULL))
);
-- One upgrade under way a notebook; the foreign key's check as a notebook is purged.
CREATE UNIQUE INDEX notebook_upgrades_notebook_id_key ON notebook_upgrades (notebook_id)
    WHERE state IN ('upgrading', 'reviewing', 'rolling_back') AND deleted_at IS NULL;
CREATE INDEX notebook_upgrades_notebook_id_idx ON notebook_upgrades (notebook_id);

-- +goose Down
DROP TABLE notebook_upgrades;
