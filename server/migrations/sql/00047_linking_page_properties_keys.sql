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
--     more is compared with value_start first, its own first 257 characters, and with the whole value only where they
--     are equal: only a page whose type starts as the filter does reads its value.
-- No index INCLUDEs value_start: with its rows all-visible an index-only probe took 9% less (the guard's 102,000
-- pages, 167 against 185 ms), and none without; a heap fetch a page reads value_start alone, value's TOAST untouched.
-- Adding the column rewrites the table under its exclusive lock, each type's and summary's value read once.

-- +goose Up
ALTER TABLE page_properties ADD COLUMN value_start text GENERATED ALWAYS AS (
    CASE WHEN translate(key, 'TYPE', 'type') = 'type' OR translate(key, 'SUMARY', 'sumary') = 'summary' THEN
        CASE WHEN jsonb_typeof(value) = 'string' THEN left(value #>> '{}', 257) END
    END
) STORED;
CREATE INDEX page_properties_type_idx ON page_properties (source_id, position)
    WHERE translate(key, 'TYPE', 'type') = 'type';
CREATE INDEX page_properties_summary_idx ON page_properties (source_id, position)
    WHERE translate(key, 'SUMARY', 'sumary') = 'summary';

-- +goose Down
DROP INDEX page_properties_summary_idx;
DROP INDEX page_properties_type_idx;
ALTER TABLE page_properties DROP COLUMN value_start;
