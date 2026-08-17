package ux

import (
	"io"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

const (
	// summaryHeading is the title line of a [Summary] block.
	summaryHeading        = "summary"
	versionsHeaderVersion = "Version"
	versionsHeaderChannel = "Channel"
	versionsHeaderArch    = "Architecture"
	versionsHeaderType    = "Type"
	// fieldSep separates a summary label from its value, and version cells.
	fieldSep = "  "
)

// SummaryRow is one label/value pair in a [Summary] block.
type SummaryRow struct {
	// Label is the left-hand field name.
	Label string
	// Value is the right-hand field value.
	Value string
}

// VersionRow is one release row for [VersionsTable].
type VersionRow struct {
	// Version is the update version name.
	Version string
	// Channel is the release channel, such as stable.
	Channel string
	// Architecture is the image architecture, such as x86_64.
	Architecture string
	// Type is the image type, iso or raw.
	Type string
}

// Summary writes a label/value block to w. Color follows [ColorMode] as
// documented on the package. Nil w is discarded.
func Summary(color ColorMode, w io.Writer, rows []SummaryRow) {
	if w == nil {
		return
	}
	if resolveColor(color, w) {
		writeFancySummary(colorWriter(color, w), rows)
		return
	}
	writePlainSummary(w, rows)
}

// VersionsTable writes a versions listing to w. Color follows [ColorMode]
// as documented on the package. Nil w is discarded.
func VersionsTable(color ColorMode, w io.Writer, rows []VersionRow) {
	if w == nil {
		return
	}
	if resolveColor(color, w) {
		writeFancyVersionsTable(colorWriter(color, w), rows)
		return
	}
	writePlainVersionsTable(w, rows)
}

// writeFancySummary renders a bordered Lip Gloss summary and writes it to w.
func writeFancySummary(w io.Writer, rows []SummaryRow) {
	pal := DefaultPalette()
	lines := make([]string, 0, len(rows)+1)
	lines = append(lines, pal.SummaryTitle.Render(summaryHeading))
	for _, row := range rows {
		key := pal.SummaryKey.Render(row.Label)
		val := pal.SummaryValue.Render(row.Value)
		lines = append(lines, key+fieldSep+val)
	}
	block := pal.SummaryBox.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	writeString(w, block+"\n")
}

// writePlainSummary writes an unstyled summary with fmt-style concatenation.
func writePlainSummary(w io.Writer, rows []SummaryRow) {
	writeString(w, summaryHeading+"\n")
	for _, row := range rows {
		writeString(w, row.Label+fieldSep+row.Value+"\n")
	}
}

// writeFancyVersionsTable renders a Lip Gloss versions table and writes it to w.
func writeFancyVersionsTable(w io.Writer, rows []VersionRow) {
	pal := DefaultPalette()
	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(pal.TableBorder).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return pal.TableHeader
			}
			return pal.TableCell
		}).
		Headers(versionsHeaderVersion, versionsHeaderChannel, versionsHeaderArch, versionsHeaderType)
	for _, row := range rows {
		tbl.Row(row.Version, row.Channel, row.Architecture, row.Type)
	}
	writeString(w, tbl.String())
}

// writePlainVersionsTable writes an unstyled versions listing with plain fmt.
func writePlainVersionsTable(w io.Writer, rows []VersionRow) {
	writeString(w, versionsHeaderVersion+fieldSep+versionsHeaderChannel+fieldSep+
		versionsHeaderArch+fieldSep+versionsHeaderType+"\n")
	for _, row := range rows {
		writeString(w, row.Version+fieldSep+row.Channel+fieldSep+
			row.Architecture+fieldSep+row.Type+"\n")
	}
}
