package drh

import (
	"bytes"
	_ "embed"
)

// seedCorrespondances is the DRH → ISS correspondence table validated by hand
// with the ministry on the 2026 millésime. It seeds a fresh install so the
// first import starts from the arbitration already done; it is then edited
// through the admin screen, which overrides it entirely.
//
//go:embed seed/correspondances.csv
var seedCorrespondances []byte

// SeedCorrespondances parses the embedded table. Errors there would be a
// packaging bug, so they are returned rather than ignored.
func SeedCorrespondances() ([]Correspondance, []LineError) {
	return ParseCorrespondancesCSV(bytes.NewReader(seedCorrespondances))
}
