package drh

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// corrColumns is the CSV header of the correspondence table, as produced by the
// export and by the manual work done with the MSHP.
var corrColumns = []string{"libelle_drh", "structure_iss", "uid_dhis2", "district", "type", "statut"}

// statutAliases accepte les libellés saisis à la main dans le fichier de
// travail ("NON RATTACHE") aussi bien que les codes internes.
var statutAliases = map[string]string{
	"ok":                 CorrOK,
	"bureau de district": CorrBureau,
	"bureau_district":    CorrBureau,
	"non rattache":       CorrNonRattache,
	"non_rattache":       CorrNonRattache,
	"a trancher":         CorrATrancher,
	"a_trancher":         CorrATrancher,
}

// ParseCorrespondancesCSV reads the DRH → ISS correspondence table. Columns are
// matched by name, so the file can be edited in a spreadsheet and re-imported.
func ParseCorrespondancesCSV(r io.Reader) ([]Correspondance, []LineError) {
	cr := csv.NewReader(r)
	cr.Comma = ';'
	// Les lignes « # » sont des commentaires : la table s'édite à la main, et
	// pouvoir désactiver une règle sans la perdre — ou proposer deux cibles
	// pour un même libellé en n'en gardant qu'une — évite de raisonner de tête.
	cr.Comment = '#'
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	cr.LazyQuotes = true

	header, err := cr.Read()
	if err != nil {
		return nil, []LineError{{Line: 1, Message: "fichier vide ou illisible"}}
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))] = i
	}
	if _, ok := idx["libelle_drh"]; !ok {
		return nil, []LineError{{Line: 1, Message: `colonne "libelle_drh" absente`}}
	}

	var out []Correspondance
	var errs []LineError
	seen := map[string]int{} // libellé normalisé → index dans out
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
		libelle := get("libelle_drh")
		if libelle == "" {
			errs = append(errs, LineError{Line: line, Message: "libelle_drh vide"})
			continue
		}
		statut, ok := statutAliases[strings.ToLower(get("statut"))]
		if !ok {
			errs = append(errs, LineError{Line: line, Message: fmt.Sprintf("statut inconnu %q (ok, bureau de district, non rattache, a trancher)", get("statut"))})
			continue
		}
		uid := get("uid_dhis2")
		if statut == CorrOK && uid == "" {
			errs = append(errs, LineError{Line: line, Message: `statut "ok" sans uid_dhis2`})
			continue
		}
		// Deux lignes pour le même libellé : la **dernière** gagne. La table
		// s'édite en ajoutant à la fin — une correction écrite après coup doit
		// l'emporter sur la règle qu'elle corrige, pas être ignorée en silence.
		c := Correspondance{
			LibelleNorm: Norm(libelle),
			LibelleDRH:  libelle,
			OrgUnitUID:  uid,
			Statut:      statut,
			District:    get("district"),
		}
		if i, deja := seen[c.LibelleNorm]; deja {
			out[i] = c
			continue
		}
		seen[c.LibelleNorm] = len(out)
		out = append(out, c)
	}
	return out, errs
}

// WriteCorrespondancesCSV writes the table back in the same format.
func WriteCorrespondancesCSV(w io.Writer, corr []Correspondance, nameOf func(uid string) string) error {
	cw := csv.NewWriter(w)
	cw.Comma = ';'
	if err := cw.Write(corrColumns); err != nil {
		return err
	}
	for _, c := range corr {
		name := ""
		if c.OrgUnitUID != "" && nameOf != nil {
			name = nameOf(c.OrgUnitUID)
		}
		if err := cw.Write([]string{c.LibelleDRH, name, c.OrgUnitUID, c.District, "", c.Statut}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
