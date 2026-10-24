-- notebooks.mode and source_root_id: a notebook's mode, standard or llm_wiki, and an LLM Wiki's source root, the page
-- its template wrote, whose subtree is its source area (M10 design 4.2, 4.3). The source root has no key, as
-- transfer_jobs.root_id has none: a key would have the notebook and page modules reference each other and break the
-- purge's order, leaf to root (bootstrap/registrants.go, purgers); a notebook's deletion deletes the page with it,
-- and a downgrade sets it NULL. The check asks an LLM Wiki for a source root, not a standard notebook for none: M11's
-- upgrade may set one before the mode. M11's upgrade states are its own, never a mode.

-- +goose Up
ALTER TABLE notebooks ADD COLUMN mode text NOT NULL DEFAULT 'standard',
    ADD CONSTRAINT notebooks_mode_check CHECK (mode IN ('standard', 'llm_wiki')),
    ADD COLUMN source_root_id uuid,
    ADD CONSTRAINT notebooks_source_root_check CHECK (mode <> 'llm_wiki' OR source_root_id IS NOT NULL);

-- +goose Down
-- A program before it knows no mode: an LLM Wiki reads as a standard notebook, its pages kept, raw and AGENTS among
-- them, its source root's id dropped with the column.
ALTER TABLE notebooks DROP CONSTRAINT notebooks_source_root_check, DROP COLUMN source_root_id,
    DROP CONSTRAINT notebooks_mode_check, DROP COLUMN mode;
