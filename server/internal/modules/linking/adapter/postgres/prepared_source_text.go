package postgresadapter

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
)

// A comparison owns only incoming string views and private row state. Native
// text bytes are consumed synchronously by ScanBytes and never stored here.
type preparedSourceTextComparison struct {
	links       *preparedLinkFacts
	matcher     *preparedStartMatcher
	start       *pgtype.Int4
	value       retentionLinkValues
	index       int
	found, same bool
	columns     [7]preparedSourceTextColumn
}

// Only verified native TEXT uses the byte callbacks. Metadata differences
// select the original destinations, leaving native Scan to report its own
// first error. Qualification runs after the first successful Next.
func (c *preparedSourceTextComparison) prepare(rows pgx.Rows, links *preparedLinkFacts, matcher *preparedStartMatcher,
	start *pgtype.Int4, row *gen.InspectPageLinksForPreparationRow, nullable [5]*pgtype.Text) bool {
	fields := rows.FieldDescriptions()
	m := rows.TypeMap()
	conn := rows.Conn()
	if len(fields) != 15 || m == nil || conn == nil || conn.TypeMap() != m {
		return false
	}
	typ, ok := m.TypeForOID(pgtype.TextOID)
	if !ok || typ == nil || typ.OID != pgtype.TextOID {
		return false
	}
	if _, standard := typ.Codec.(pgtype.TextCodec); !standard {
		return false
	}
	for _, field := range fields[4:11] {
		if field.DataTypeOID != pgtype.TextOID ||
			field.Format != pgtype.TextFormatCode && field.Format != pgtype.BinaryFormatCode {
			return false
		}
	}
	kindPlan := m.PlanScan(pgtype.TextOID, fields[4].Format, &row.Kind)
	targetPlan := m.PlanScan(pgtype.TextOID, fields[6].Format, &row.Target)
	if kindPlan == nil || targetPlan == nil {
		return false
	}
	c.links, c.matcher, c.start = links, matcher, start
	c.columns = [7]preparedSourceTextColumn{
		{comparison: c, expected: &c.value.kind, first: true, required: &row.Kind, nullPlan: kindPlan},
		{comparison: c, expected: &c.value.property, nullable: nullable[0]},
		{comparison: c, expected: &c.value.target, required: &row.Target, nullPlan: targetPlan},
		{comparison: c, expected: &c.value.anchor, nullable: nullable[1]},
		{comparison: c, expected: &c.value.display, nullable: nullable[2]},
		{comparison: c, expected: &c.value.key, nullable: nullable[3]},
		{comparison: c, expected: &c.value.alt, nullable: nullable[4]},
	}
	return true
}

func (c *preparedSourceTextComparison) reset() {
	c.value = retentionLinkValues{}
	c.index, c.found, c.same = 0, false, true
	for i := range c.columns {
		if value := c.columns[i].nullable; value != nil {
			*value = pgtype.Text{}
		}
	}
}

// Only an exact non-NULL payload match may supply incoming string views.
// The native NULL flags survive unchanged, so the existing retention
// predicate remains the only implementation of nullable normalization.
func (c *preparedSourceTextComparison) project() {
	if !c.found || !c.same {
		return
	}
	for i := range c.columns {
		column := &c.columns[i]
		if column.nullable != nil {
			column.nullable.String = *column.expected
		} else {
			*column.required = *column.expected
		}
	}
}

// Do not implement or embed TextScanner: its text-format shortcut allocates
// a string before invoking the destination.
type preparedSourceTextColumn struct {
	comparison *preparedSourceTextComparison
	expected   *string
	nullable   *pgtype.Text
	required   *string
	nullPlan   pgtype.ScanPlan
	first      bool
}

func (column *preparedSourceTextColumn) ScanBytes(src []byte) error {
	c := column.comparison
	if src == nil {
		if column.nullable != nil {
			column.nullable.Valid = false
			return nil
		}
		// Preserve the original *string destination, native error text and
		// first-error position, including a later invalid bool in this row.
		return column.nullPlan.Scan(nil, column.required)
	}
	if column.first && c.start.Valid {
		c.index, c.found = c.matcher.find(c.start.Int32)
		if c.found {
			c.value = c.links.at(c.index)
		}
	}
	if column.nullable != nil {
		column.nullable.Valid = true
	}
	if c.found && !preparedSourceTextEqual(src, *column.expected) {
		c.same = false
	}
	return nil
}

func preparedSourceTextEqual(src []byte, expected string) bool {
	if len(src) != len(expected) {
		return false
	}
	for i, value := range src {
		if value != expected[i] {
			return false
		}
	}
	return true
}
