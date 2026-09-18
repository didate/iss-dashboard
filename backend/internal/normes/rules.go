// Package normes gère le référentiel de normes (exigences par type de structure)
// et l'évaluation de la conformité des structures à ce référentiel.
//
// Le référentiel est une donnée éditée par l'admin (tables norme_set /
// norme_rule), jamais du code : ce fichier ne contient que le format, la
// validation et le catalogue des cibles possibles.
package normes

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/typologie"
)

// Familles d'exigences.
const (
	KindService     = "service"
	KindRH          = "rh"
	KindEquipement  = "equipement"
	KindInfra       = "infra"
	LevelEssentiel  = "essentiel"
	LevelRecommande = "recommande"
	AllTypes        = "*"
)

var Kinds = []string{KindService, KindRH, KindEquipement, KindInfra}

// Target is one thing a rule can point at (a service, an RH profile, an
// equipment, an infrastructure), with its display label.
type Target struct {
	Kind   string `json:"kind"`
	Code   string `json:"code"`
	Label  string `json:"label"`
	Prefix bool   `json:"prefix,omitempty"` // RH only: matches every profile starting with Code
}

// Catalog lists the admissible targets per kind. Built from the DHIS2 metadata
// already in SQLite, so a rule can only point at something the ISS form measures.
type Catalog struct {
	Targets map[string]map[string]Target // kind → code → target
}

func NewCatalog() *Catalog {
	c := &Catalog{Targets: map[string]map[string]Target{}}
	for _, k := range Kinds {
		c.Targets[k] = map[string]Target{}
	}
	return c
}

func (c *Catalog) Add(t Target) { c.Targets[t.Kind][t.Code] = t }

// Sorted returns the catalog as a flat, sorted list (for the editor's menus).
func (c *Catalog) Sorted() []Target {
	var out []Target
	for _, k := range Kinds {
		for _, t := range c.Targets[k] {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Label < out[j].Label
	})
	return out
}

// Resolve checks that a (kind, target) exists and returns its label. RH targets
// ending with "_" are prefixes and must match at least one profile.
func (c *Catalog) Resolve(kind, target string) (label string, ok bool) {
	byCode, known := c.Targets[kind]
	if !known {
		return "", false
	}
	if t, ok := byCode[target]; ok {
		return t.Label, true
	}
	if kind == KindRH && strings.HasSuffix(target, "_") {
		var matches []string
		for code, t := range byCode {
			if strings.HasPrefix(code, target) {
				matches = append(matches, t.Label)
			}
		}
		if len(matches) > 0 {
			sort.Strings(matches)
			return "Tout profil " + strings.TrimSuffix(strings.TrimPrefix(target, "ISS_RH_"), "_") + " (" + strings.Join(matches, ", ") + ")", true
		}
	}
	return "", false
}

// knownTypeCodes are the type codes a rule may target (plus "*").
func knownTypeCodes() map[string]bool {
	m := map[string]bool{AllTypes: true}
	for code := range typologie.Labels {
		m[code] = true
	}
	return m
}

// LineError is one rejected CSV line.
type LineError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// ParseCSV reads rules from a ";"-separated CSV with header
// type_code;kind;target;label;min_value;level. Lines starting with "#" are
// comments. Labels left empty are filled from the catalog. Every line is
// validated against the catalog; invalid lines are reported, valid ones returned.
func ParseCSV(r io.Reader, catalog *Catalog) ([]models.NormeRule, []LineError) {
	cr := csv.NewReader(r)
	cr.Comma = ';'
	cr.Comment = '#'
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	cr.LazyQuotes = true

	var rules []models.NormeRule
	var errs []LineError
	headerSeen := false
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		line, _ := cr.FieldPos(0) // numéro de ligne réel du fichier (commentaires inclus)
		if err != nil {
			errs = append(errs, LineError{Line: line, Message: err.Error()})
			continue
		}
		if len(rec) == 1 && strings.TrimSpace(rec[0]) == "" {
			continue
		}
		if !headerSeen {
			headerSeen = true
			if strings.EqualFold(strings.TrimSpace(rec[0]), "type_code") {
				continue
			}
		}
		if len(rec) < 3 {
			errs = append(errs, LineError{Line: line, Message: "au moins type_code;kind;target attendus"})
			continue
		}
		get := func(i int) string {
			if i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
			return ""
		}
		rule := models.NormeRule{
			TypeCode: strings.ToUpper(get(0)),
			Kind:     strings.ToLower(get(1)),
			Target:   get(2),
			Label:    get(3),
			MinValue: 1,
			Level:    strings.ToLower(get(5)),
		}
		if v := get(4); v != "" {
			f, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64)
			if err != nil {
				errs = append(errs, LineError{Line: line, Message: fmt.Sprintf("min_value invalide : %q", v)})
				continue
			}
			rule.MinValue = f
		}
		if rule.Level == "" {
			rule.Level = LevelEssentiel
		}
		if msg := Validate(&rule, catalog); msg != "" {
			errs = append(errs, LineError{Line: line, Message: msg})
			continue
		}
		rules = append(rules, rule)
	}
	return rules, errs
}

// Validate normalises a rule and returns an error message ("" when valid).
// The label is filled from the catalog when empty.
func Validate(r *models.NormeRule, catalog *Catalog) string {
	r.TypeCode = strings.ToUpper(strings.TrimSpace(r.TypeCode))
	r.Kind = strings.ToLower(strings.TrimSpace(r.Kind))
	r.Target = strings.TrimSpace(r.Target)
	r.Level = strings.ToLower(strings.TrimSpace(r.Level))
	if r.Level == "" {
		r.Level = LevelEssentiel
	}
	if r.Level == "recommandé" || r.Level == "recommandee" {
		r.Level = LevelRecommande
	}

	if !knownTypeCodes()[r.TypeCode] {
		return fmt.Sprintf("type_code inconnu : %q (attendu PS, CS, CSA, CMC, HP, HR, HN, CABINET, CLINIQUE, AUTRE_PRIVE ou *)", r.TypeCode)
	}
	validKind := false
	for _, k := range Kinds {
		if r.Kind == k {
			validKind = true
		}
	}
	if !validKind {
		return fmt.Sprintf("kind inconnu : %q (attendu service, rh, equipement, infra)", r.Kind)
	}
	if r.Level != LevelEssentiel && r.Level != LevelRecommande {
		return fmt.Sprintf("level inconnu : %q (attendu essentiel ou recommande)", r.Level)
	}
	if r.Kind == KindService {
		r.MinValue = 1 // un service est attendu ou non
	} else if r.MinValue <= 0 {
		return "min_value doit être > 0"
	}
	if catalog != nil {
		label, ok := catalog.Resolve(r.Kind, r.Target)
		if !ok {
			return fmt.Sprintf("cible inconnue pour %s : %q (voir le catalogue des cibles)", r.Kind, r.Target)
		}
		if r.Label == "" {
			r.Label = label
		}
	}
	if r.Label == "" {
		r.Label = r.Target
	}
	return ""
}

// WriteCSV serialises rules in the import format.
func WriteCSV(w io.Writer, rules []models.NormeRule) error {
	cw := csv.NewWriter(w)
	cw.Comma = ';'
	if err := cw.Write([]string{"type_code", "kind", "target", "label", "min_value", "level"}); err != nil {
		return err
	}
	for _, r := range rules {
		if err := cw.Write([]string{r.TypeCode, r.Kind, r.Target, r.Label, strconv.FormatFloat(r.MinValue, 'f', -1, 64), r.Level}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
