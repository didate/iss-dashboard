package drh

import "sort"

// CategorieToutes is the key of the "all professions" row.
const CategorieToutes = ""

// EffectifRow is one headcount cell: a place (dimension + key) crossed with a
// professional category. This is the finest grain ever persisted — no line
// describes an individual.
type EffectifRow struct {
	Dimension    string `json:"dimension"` // grain fin : structure|bureau|centrale|non_rattache ; rollups : global|region|district|sous_prefecture|type
	Key          string `json:"key"`
	Label        string `json:"label"`
	District     string `json:"district"`
	Region       string `json:"region"`
	Categorie    string `json:"categorie"` // "" = toutes professions
	NAgents      int    `json:"n_agents"`
	NFemmes      int    `json:"n_femmes"`
	NDepart5Ans  int    `json:"n_depart_5ans"`
	NDepart10Ans int    `json:"n_depart_10ans"`
	// NAgeConnu est le dénominateur des taux de départ : les agents sans année
	// de naissance comptent dans l'effectif, mais les inclure diluerait le taux
	// et ferait passer le risque pour plus faible qu'il n'est.
	NAgeConnu int `json:"n_age_connu"`

	// Répartition par lieu d'affectation, renseignée sur les rollups.
	NStructure   int `json:"n_structure"`
	NBureau      int `json:"n_bureau"`
	NCentrale    int `json:"n_centrale"`
	NNonRattache int `json:"n_non_rattache"`

	Population *float64 `json:"population,omitempty"`
	Ratio10k   *float64 `json:"ratio_10k,omitempty"`
}

// PyramideRow is one age-bracket cell of the same place and category.
type PyramideRow struct {
	Dimension string `json:"dimension"`
	Key       string `json:"key"`
	Categorie string `json:"categorie"`
	Tranche   string `json:"tranche"`
	NAgents   int    `json:"n_agents"`
	NFemmes   int    `json:"n_femmes"`
}

type cellKey struct{ dim, key, cat string }
type pyrKey struct{ dim, key, cat, tranche string }

// Aggregate turns the agents into the cells that will be persisted. Every agent
// feeds two cells: its own category, and the "all professions" total.
//
// Only the finest grain is produced here (one place = one facility, one district
// office, or the central administration). The rollups by district, region and
// type, the population ratios and the comparison with ISS are computed later,
// from these cells, because they also depend on the ISS snapshot.
func Aggregate(rows []AgentRow, affs []Affectation, opt Options) ([]EffectifRow, []PyramideRow) {
	opt = opt.Normalize(0)
	eff := map[cellKey]*EffectifRow{}
	pyr := map[pyrKey]*PyramideRow{}

	for i, a := range rows {
		if i >= len(affs) {
			break
		}
		aff := affs[i]
		cat := CategorieDe(a.Profession)
		femme := a.Sexe == "F"
		d5 := a.PartDansMoinsDe(5, opt)
		d10 := a.PartDansMoinsDe(10, opt)
		tranche := Tranche(a.Age(opt.RefYear))

		for _, c := range []string{CategorieToutes, cat} {
			ck := cellKey{aff.Kind, aff.Key, c}
			row, ok := eff[ck]
			if !ok {
				row = &EffectifRow{Dimension: aff.Kind, Key: aff.Key, Label: aff.Label,
					District: aff.District, Region: aff.Region, Categorie: c}
				eff[ck] = row
			}
			row.NAgents++
			if femme {
				row.NFemmes++
			}
			if a.Age(opt.RefYear) >= 0 {
				row.NAgeConnu++
			}
			if d5 {
				row.NDepart5Ans++
			}
			if d10 {
				row.NDepart10Ans++
			}

			pk := pyrKey{aff.Kind, aff.Key, c, tranche}
			p, ok := pyr[pk]
			if !ok {
				p = &PyramideRow{Dimension: aff.Kind, Key: aff.Key, Categorie: c, Tranche: tranche}
				pyr[pk] = p
			}
			p.NAgents++
			if femme {
				p.NFemmes++
			}
		}
	}

	effOut := make([]EffectifRow, 0, len(eff))
	for _, r := range eff {
		effOut = append(effOut, *r)
	}
	sort.Slice(effOut, func(i, j int) bool { return lessEffectif(effOut[i], effOut[j]) })

	pyrOut := make([]PyramideRow, 0, len(pyr))
	for _, r := range pyr {
		pyrOut = append(pyrOut, *r)
	}
	sort.Slice(pyrOut, func(i, j int) bool { return lessPyramide(pyrOut[i], pyrOut[j]) })
	return effOut, pyrOut
}

func lessEffectif(a, b EffectifRow) bool {
	switch {
	case a.Dimension != b.Dimension:
		return a.Dimension < b.Dimension
	case a.Key != b.Key:
		return a.Key < b.Key
	default:
		return a.Categorie < b.Categorie
	}
}

func lessPyramide(a, b PyramideRow) bool {
	switch {
	case a.Dimension != b.Dimension:
		return a.Dimension < b.Dimension
	case a.Key != b.Key:
		return a.Key < b.Key
	case a.Categorie != b.Categorie:
		return a.Categorie < b.Categorie
	default:
		return trancheOrder(a.Tranche) < trancheOrder(b.Tranche)
	}
}

var trancheRank = func() map[string]int {
	m := make(map[string]int, len(Tranches))
	for i, t := range Tranches {
		m[t] = i
	}
	return m
}()

func trancheOrder(t string) int {
	if r, ok := trancheRank[t]; ok {
		return r
	}
	return len(Tranches)
}
