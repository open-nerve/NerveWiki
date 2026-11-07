-- Choose LZ4 for future compressible content; existing values are not rewritten.
-- +goose Up
ALTER TABLE page_contents ALTER COLUMN content SET COMPRESSION lz4;
ALTER TABLE page_revisions ALTER COLUMN content SET COMPRESSION lz4;

-- +goose Down
-- Restore the default policy; already stored LZ4 values still need an LZ4-capable reader.
ALTER TABLE page_contents ALTER COLUMN content SET COMPRESSION default;
ALTER TABLE page_revisions ALTER COLUMN content SET COMPRESSION default;
