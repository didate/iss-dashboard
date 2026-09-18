package normes

import (
	"strings"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/quality"
	"iss-dashboard-backend/internal/typologie"
	"iss-dashboard-backend/internal/usage"
)

// Statuts d'une exigence évaluée sur une structure.
const (
	StatusOK      = "ok"
	StatusManque  = "manque"
	StatusInconnu = "inconnu" // donnée ISS non renseignée : hors score (principe 5)
)

// Item is one requirement evaluated on one structure.
type Item struct {
	RuleID   int64    `json:"rule_id"`
	TypeCode string   `json:"type_code"` // type de la règle appliquée (peut être "*")
	Kind     string   `json:"kind"`
	Target   string   `json:"target"`
	Label    string   `json:"label"`
	Level    string   `json:"level"`
	Expected float64  `json:"expected"`
	Observed *float64 `json:"observed"` // nil = non renseigné
	Status   string   `json:"status"`
}

// Summary is the per-structure roll-up of its items.
type Summary struct {
	NRules           int      `json:"n_rules"`
	NOk              int      `json:"n_ok"`
	NManque          int      `json:"n_manque"`
	NInconnu         int      `json:"n_inconnu"`
	NManqueEssentiel int      `json:"n_manque_essentiel"`
	Score            *float64 `json:"score"` // nil = non évalué (aucune règle applicable ou tout inconnu)
	Conforme         bool     `json:"conforme"`
}

// Evaluator holds the active rules and the data-element lookups needed to read
// what a structure declares. Build it once per recompute, then Evaluate each event.
type Evaluator struct {
	byType    map[string][]models.NormeRule // type_code → rules (incl. "*")
	codeToUID map[string]string
	rhDEs     []usage.RHDataElement
	equipFonc map[string]string // equipment root → FONC data element uid
}

func NewEvaluator(rules []models.NormeRule, ctx *quality.QualityContext) *Evaluator {
	e := &Evaluator{
		byType:    map[string][]models.NormeRule{},
		codeToUID: ctx.CodeToUID,
		equipFonc: map[string]string{},
	}
	for _, r := range rules {
		e.byType[r.TypeCode] = append(e.byType[r.TypeCode], r)
	}
	e.rhDEs, _ = usage.DiscoverRHProfiles(ctx)
	for _, p := range ctx.EquipPairs {
		e.equipFonc[p.Root] = p.FoncUID
	}
	return e
}

// RulesFor returns the rules applying to a type: the type's own rules plus the
// "*" rules, a specific rule overriding "*" for the same (kind, target).
// INDETERMINE (or empty) types only get the "*" rules.
func (e *Evaluator) RulesFor(typeCode string) []models.NormeRule {
	if typeCode == "" {
		typeCode = typologie.Indetermine
	}
	var out []models.NormeRule
	seen := map[string]bool{}
	if typeCode != typologie.Indetermine {
		for _, r := range e.byType[typeCode] {
			out = append(out, r)
			seen[r.Kind+"|"+r.Target] = true
		}
	}
	for _, r := range e.byType[AllTypes] {
		if !seen[r.Kind+"|"+r.Target] {
			out = append(out, r)
		}
	}
	return out
}

// Evaluate applies the rules of the structure's type to its declared values.
func (e *Evaluator) Evaluate(evt *models.Event) []Item {
	rules := e.RulesFor(evt.TypeCode)
	if len(rules) == 0 {
		return nil
	}
	vals := evt.Values()
	items := make([]Item, 0, len(rules))
	for _, r := range rules {
		it := Item{RuleID: r.ID, TypeCode: r.TypeCode, Kind: r.Kind, Target: r.Target, Label: r.Label, Level: r.Level, Expected: r.MinValue}
		it.Observed = e.observe(r, vals)
		switch {
		case it.Observed == nil:
			it.Status = StatusInconnu
		case *it.Observed >= it.Expected:
			it.Status = StatusOK
		default:
			it.Status = StatusManque
		}
		items = append(items, it)
	}
	return items
}

// observe reads the value a rule compares against; nil when not declared.
func (e *Evaluator) observe(r models.NormeRule, vals map[string]string) *float64 {
	switch r.Kind {
	case KindService:
		v := strings.TrimSpace(vals[e.codeToUID[r.Target]])
		switch v {
		case "":
			return nil
		case "oui":
			return f(1)
		default: // non, oui_pas_fonctionnel : le service n'est pas offert fonctionnellement
			return f(0)
		}
	case KindRH:
		prefix := strings.HasSuffix(r.Target, "_")
		sum, any := 0.0, false
		for _, de := range e.rhDEs {
			if (prefix && strings.HasPrefix(de.Root, r.Target)) || (!prefix && de.Root == r.Target) {
				if v := strings.TrimSpace(vals[de.UID]); v != "" {
					any = true
					sum += quality.ParseNum(v)
				}
			}
		}
		if !any {
			return nil
		}
		return f(sum)
	case KindEquipement:
		uid, ok := e.equipFonc[r.Target]
		if !ok {
			return nil
		}
		return num(vals[uid])
	case KindInfra:
		return num(vals[e.codeToUID[r.Target]])
	}
	return nil
}

func num(v string) *float64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return f(quality.ParseNum(v))
}

func f(v float64) *float64 { return &v }

// Summarize computes the per-structure roll-up. Score = 100 × Σpoids(ok) /
// Σpoids(ok + manque), essentiel = 2, recommande = 1 ; les inconnus sont hors
// dénominateur. Conforme = aucun manque essentiel (et au moins une exigence connue).
func Summarize(items []Item) Summary {
	s := Summary{NRules: len(items)}
	okW, totW := 0.0, 0.0
	for _, it := range items {
		w := 1.0
		if it.Level == LevelEssentiel {
			w = 2
		}
		switch it.Status {
		case StatusOK:
			s.NOk++
			okW += w
			totW += w
		case StatusManque:
			s.NManque++
			totW += w
			if it.Level == LevelEssentiel {
				s.NManqueEssentiel++
			}
		default:
			s.NInconnu++
		}
	}
	if totW > 0 {
		s.Score = f(100 * okW / totW)
		s.Conforme = s.NManqueEssentiel == 0
	}
	return s
}
