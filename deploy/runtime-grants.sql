-- What the role nervewiki serves with needs when another role, the owner of the tables, runs the migrations
-- (README, "部署"). It grants to the group role nervewiki_runtime; the login role of nervewiki serve is a member of
-- it. Run it as the owner of the tables after every `nervewiki migrate up`: it names each table and sequence, and
-- a new migration may add one. Running it again changes nothing.
--
-- Tables get reads and writes (DML) by table: no TRUNCATE, REFERENCES, TRIGGER or DDL. Functions and types get
-- nothing here: PUBLIC may run every function and use every type by default, and River's river_job_state_in_bitmask
-- and river_job_state rely on that. A migration that adds a view grants SELECT on it here; one that takes EXECUTE
-- on a function from PUBLIC grants it here.
--
-- server/internal/bootstrap/runtime_role_test.go runs nervewiki on a role with exactly these grants (ready, the
-- API, the administrator's commands, River's jobs, River's reindex), and fails when a table, view, sequence or
-- function of the schema public has other privileges than these.

GRANT USAGE ON SCHEMA public TO nervewiki_runtime;

-- The modules' tables.
GRANT SELECT, INSERT, UPDATE, DELETE ON users, auth_sessions, api_tokens, workspaces, workspace_members, workspace_invitations,
    notebooks, notebook_members, notebook_audit_events, nodes, page_contents, changesets, changeset_items,
    page_revisions, edit_sessions TO nervewiki_runtime;

-- River's tables and the sequences of their ids. River rebuilds the indexes of river_job every day with
-- REINDEX INDEX CONCURRENTLY, which takes MAINTAIN on the table (PostgreSQL 17 and later).
GRANT SELECT, INSERT, UPDATE, DELETE ON river_job, river_leader, river_queue, river_notification TO nervewiki_runtime;
GRANT USAGE ON SEQUENCE river_job_id_seq, river_notification_id_seq TO nervewiki_runtime;
GRANT MAINTAIN ON river_job TO nervewiki_runtime;

-- goose's record of the migrations: /readyz reads it to tell whether every migration is applied.
GRANT SELECT ON goose_db_version TO nervewiki_runtime;
