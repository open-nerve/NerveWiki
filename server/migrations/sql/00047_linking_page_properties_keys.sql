-- page_properties' type and summary, read by index a probe a page (M9/P4 design 3.9.3): a page's first property whose
-- key is "type" (or "summary") but for ASCII case, whatever other properties it has. Without the partial indexes
-- index's lateral reads every property of a page without a type, up to the frontmatter's 10,000 nodes. A key that
-- matches but for ASCII case has 16 (or 128) spellings, so each index holds a few rows a page at most.
--
-- value_start is the start of such a property's value when it is a string, its first 257 characters, NULL for any
-- other row (S2 review B-I1): index reads it and never value, whose whole a writer sets (a string of the content's
-- 5 MiB) and which jsonb's operators detoast whole to take any part of it. 257 is the least that each use needs:
--   - the order's key is a type's first 128 characters (IndexTypeChars);
--   - an answer cuts a type or a summary to 256 bytes at a character's boundary, and says so when it is cut
--     (MaxIndexString). The first 257 characters tell both: a string of at most 257 characters is whole in them; one of
--     more has more than 257 bytes, so it is cut, and its first 256 bytes are among its first 257 characters, each at
--     least a byte. 256 characters would not tell a string of 256 ASCII characters from one of 257;
--   - the type filter (at most 1,024 bytes, MaxIndexType) is equal to a type when it is shorter than 257 characters
--     and value_start is it: value_start shorter than 257 characters is the whole string. A filter of 257 characters or
--     more is compared with value_start first, its own first 257 characters, then with value_chars, and with the whole
--     value only where both are equal.
-- value_chars is such a value's length in characters, for the same rows (S2 fix check B, F-m1): without it a filter of
-- 257 characters or more read whole every type that starts as it does, whatever its length (2,000 types of 5 MB
-- that start as the filter does, 0.76 s a call). With it, the whole value is read only when it has as many characters
-- as the filter, which has at most 1,024 bytes: a value of at most 1,024 characters, 4 KB at most.
-- No index INCLUDEs value_start: with its rows all-visible an index-only probe took 9% less (the guard's 102,000
-- pages, 167 against 185 ms), and none without; a heap fetch a page reads value_start alone, value's TOAST untouched.
-- Adding the columns rewrites the table under its exclusive lock, reads of it waiting as writes do, each type's and
-- summary's value read once; the rewrite needs disk for a second copy of the table while it runs.

-- +goose Up
ALTER TABLE page_properties ADD COLUMN value_start text GENERATED ALWAYS AS (
    CASE WHEN translate(key, 'TYPE', 'type') = 'type' OR translate(key, 'SUMARY', 'sumary') = 'summary' THEN
        CASE WHEN jsonb_typeof(value) = 'string' THEN left(value #>> '{}', 257) END
    END
) STORED, ADD COLUMN value_chars integer GENERATED ALWAYS AS (
    CASE WHEN translate(key, 'TYPE', 'type') = 'type' OR translate(key, 'SUMARY', 'sumary') = 'summary' THEN
        CASE WHEN jsonb_typeof(value) = 'string' THEN char_length(value #>> '{}') END
    END
) STORED;
CREATE INDEX page_properties_type_idx ON page_properties (source_id, position)
    WHERE translate(key, 'TYPE', 'type') = 'type';
CREATE INDEX page_properties_summary_idx ON page_properties (source_id, position)
    WHERE translate(key, 'SUMARY', 'sumary') = 'summary';

-- +goose Down
DROP INDEX page_properties_summary_idx;
DROP INDEX page_properties_type_idx;
ALTER TABLE page_properties DROP COLUMN value_chars, DROP COLUMN value_start;
