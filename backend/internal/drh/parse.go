package drh

import (
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Columns of the normalised CSV, in the expected order (docs/drh-format.md).
var Columns = []string{
	"region", "prefecture", "sous_prefecture",
	"structure_affectation", "structure_rattachement",
	"profession", "profession_oms", "hierarchie", "statut",
	"sexe", "annee_naissance", "zone", "niveau_structure",
}

// requiredColumns : ce sans quoi une ligne ne peut pas être placée. La
// profession et le sexe peuvent manquer (200 agents du millésime 2026 n'ont pas
// de profession renseignée) : l'agent est alors compté, en catégorie « Autre ».
var requiredColumns = []string{"region", "prefecture"}

// LineError is one rejected line of the import file.
type LineError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

var yearRE = regexp.MustCompile(`(19|20)\d{2}`)

// ParseCSV reads the normalised personnel CSV: ";"-separated, one line per
// agent, header naming the columns above.
//
// The header is validated strictly and an unexpected column aborts the import:
// this is what guarantees that a future version of the DRH file cannot
// reintroduce identifying data (matricule, name, exact date of birth) without
// someone noticing.
func ParseCSV(r io.Reader) ([]AgentRow, []LineError) {
	cr := csv.NewReader(r)
	cr.Comma = ';'
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	cr.LazyQuotes = true

	header, err := cr.Read()
	if err != nil {
		return nil, []LineError{{Line: 1, Message: "fichier vide ou illisible"}}
	}
	idx, errs := mapHeader(header)
	if len(errs) > 0 {
		return nil, errs
	}

	var rows []AgentRow
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		line, _ := cr.FieldPos(0)
		if err != nil {
			errs = append(errs, LineError{Line: line, Message: err.Error()})
			continue
		}
		if isBlank(rec) {
			continue
		}
		get := func(col string) string {
			i, ok := idx[col]
			if !ok || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		var missing []string
		for _, col := range requiredColumns {
			if get(col) == "" {
				missing = append(missing, col)
			}
		}
		if len(missing) > 0 {
			errs = append(errs, LineError{Line: line, Message: "colonne obligatoire vide : " + strings.Join(missing, ", ")})
			continue
		}
		rows = append(rows, AgentRow{
			Region:                get("region"),
			Prefecture:            get("prefecture"),
			SousPrefecture:        get("sous_prefecture"),
			StructureAffectation:  get("structure_affectation"),
			StructureRattachement: get("structure_rattachement"),
			Profession:            get("profession"),
			ProfessionOMS:         get("profession_oms"),
			Hierarchie:            strings.ToUpper(get("hierarchie")),
			Statut:                get("statut"),
			Sexe:                  normSexe(get("sexe")),
			AnneeNaissance:        parseYear(get("annee_naissance")),
			Zone:                  strings.ToLower(get("zone")),
			NiveauStructure:       strings.ToLower(get("niveau_structure")),
		})
	}
	return rows, errs
}

// mapHeader matches the header against Columns. Order does not matter, but an
// unknown column or a missing mandatory one rejects the whole file.
func mapHeader(header []string) (map[string]int, []LineError) {
	known := make(map[string]bool, len(Columns))
	for _, c := range Columns {
		known[c] = true
	}
	idx := make(map[string]int, len(header))
	var errs []LineError
	for i, h := range header {
		name := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))
		if name == "" {
			continue
		}
		if !known[name] {
			errs = append(errs, LineError{Line: 1, Message: fmt.Sprintf("colonne inconnue %q — le format attendu est décrit dans docs/drh-format.md", name)})
			continue
		}
		if _, dup := idx[name]; dup {
			errs = append(errs, LineError{Line: 1, Message: fmt.Sprintf("colonne %q en double", name)})
			continue
		}
		idx[name] = i
	}
	for _, col := range requiredColumns {
		if _, ok := idx[col]; !ok {
			errs = append(errs, LineError{Line: 1, Message: fmt.Sprintf("colonne obligatoire %q absente", col)})
		}
	}
	return idx, errs
}

func isBlank(rec []string) bool {
	for _, v := range rec {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

func normSexe(v string) string {
	switch {
	case v == "":
		return ""
	case strings.EqualFold(v[:1], "f"):
		return "F"
	default:
		return "H"
	}
}

// parseYear keeps only the year: the CSV should already hold one, but a file
// converted by hand may still carry a full date.
func parseYear(v string) int {
	m := yearRE.FindString(v)
	if m == "" {
		return 0
	}
	y := 0
	for _, c := range m {
		y = y*10 + int(c-'0')
	}
	return y
}
