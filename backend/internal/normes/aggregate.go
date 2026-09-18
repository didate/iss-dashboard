package normes

import (
	"sort"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/typologie"
)

// EventResult ties a structure to its evaluation.
type EventResult struct {
	Event   *models.Event
	Items   []Item
	Summary Summary
}

// SummaryRow is the conformity roll-up of one dimension key, optionally per type.
type SummaryRow struct {
	Dimension   string   `json:"dimension"` // global | region | district | sous_prefecture | type
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	TypeCode    string   `json:"type_code"` // "" = tous types
	NStructures int      `json:"n_structures"`
	NEvaluees   int      `json:"n_evaluees"` // avec un score
	AvgScore    *float64 `json:"avg_score"`
	NConformes  int      `json:"n_conformes"`
	PctConforme *float64 `json:"pct_conformes"`
}

// GapRow is « il manque X de Y dans Z » for one requirement.
type GapRow struct {
	Dimension   string  `json:"dimension"`
	Key         string  `json:"key"`
	TypeCode    string  `json:"type_code"`
	Kind        string  `json:"kind"`
	Target      string  `json:"target"`
	Label       string  `json:"label"`
	Level       string  `json:"level"`
	NConcernees int     `json:"n_concernees"`
	NManque     int     `json:"n_manque"`
	NInconnu    int     `json:"n_inconnu"`
	Deficit     float64 `json:"deficit"` // Σ max(0, attendu − observé) ; services : = n_manque
}

// Aggregate builds summary and gap rows from per-structure results.
func Aggregate(results []EventResult) ([]SummaryRow, []GapRow) {
	type sAcc struct {
		n, nEval, nConf int
		sumScore        float64
	}
	type gAcc struct {
		label, level             string
		nConc, nManque, nInconnu int
		deficit                  float64
	}
	sum := map[string]*sAcc{}  // dim|key|type
	gaps := map[string]*gAcc{} // dim|key|type|kind|target

	dimsOf := func(e *models.Event) [][2]string {
		d := [][2]string{{"global", "all"}}
		if e.Region != "" {
			d = append(d, [2]string{"region", e.Region})
		}
		if e.District != "" {
			d = append(d, [2]string{"district", e.District})
		}
		if e.SousPrefecture != "" {
			d = append(d, [2]string{"sous_prefecture", e.SousPrefecture})
		}
		return d
	}

	for _, r := range results {
		tc := r.Event.TypeCode
		if tc == "" {
			tc = typologie.Indetermine
		}
		dims := dimsOf(r.Event)
		dims = append(dims, [2]string{"type", tc})
		for _, d := range dims {
			// two rows per dimension key: all types ("") and this type — except for
			// the "type" dimension, whose key is already the type.
			typeKeys := []string{"", tc}
			if d[0] == "type" {
				typeKeys = []string{tc}
			}
			for _, tk := range typeKeys {
				k := d[0] + "|" + d[1] + "|" + tk
				a := sum[k]
				if a == nil {
					a = &sAcc{}
					sum[k] = a
				}
				a.n++
				if r.Summary.Score != nil {
					a.nEval++
					a.sumScore += *r.Summary.Score
					if r.Summary.Conforme {
						a.nConf++
					}
				}
			}
			if d[0] == "type" {
				continue
			}
			for _, it := range r.Items {
				k := d[0] + "|" + d[1] + "|" + tc + "|" + it.Kind + "|" + it.Target
				g := gaps[k]
				if g == nil {
					g = &gAcc{label: it.Label, level: it.Level}
					gaps[k] = g
				}
				g.nConc++
				switch it.Status {
				case StatusManque:
					g.nManque++
					if it.Kind == KindService {
						g.deficit++
					} else if it.Observed != nil {
						g.deficit += it.Expected - *it.Observed
					}
				case StatusInconnu:
					g.nInconnu++
				}
			}
		}
	}

	var srows []SummaryRow
	for k, a := range sum {
		dim, key, tc := split3(k)
		row := SummaryRow{Dimension: dim, Key: key, Label: key, TypeCode: tc, NStructures: a.n, NEvaluees: a.nEval, NConformes: a.nConf}
		if dim == "global" {
			row.Label = "National"
		}
		if dim == "type" {
			row.Label = typologie.Label(key)
		}
		if a.nEval > 0 {
			avg := a.sumScore / float64(a.nEval)
			pct := 100 * float64(a.nConf) / float64(a.nEval)
			row.AvgScore, row.PctConforme = &avg, &pct
		}
		srows = append(srows, row)
	}
	sort.Slice(srows, func(i, j int) bool {
		if srows[i].Dimension != srows[j].Dimension {
			return srows[i].Dimension < srows[j].Dimension
		}
		if srows[i].Key != srows[j].Key {
			return srows[i].Key < srows[j].Key
		}
		return srows[i].TypeCode < srows[j].TypeCode
	})

	var grows []GapRow
	for k, g := range gaps {
		parts := splitN(k, 5)
		grows = append(grows, GapRow{
			Dimension: parts[0], Key: parts[1], TypeCode: parts[2], Kind: parts[3], Target: parts[4],
			Label: g.label, Level: g.level, NConcernees: g.nConc, NManque: g.nManque, NInconnu: g.nInconnu, Deficit: g.deficit,
		})
	}
	sort.Slice(grows, func(i, j int) bool {
		if grows[i].Dimension != grows[j].Dimension {
			return grows[i].Dimension < grows[j].Dimension
		}
		if grows[i].Key != grows[j].Key {
			return grows[i].Key < grows[j].Key
		}
		if grows[i].Deficit != grows[j].Deficit {
			return grows[i].Deficit > grows[j].Deficit
		}
		return grows[i].Label < grows[j].Label
	})
	return srows, grows
}

func split3(k string) (string, string, string) {
	p := splitN(k, 3)
	return p[0], p[1], p[2]
}

// splitN splits on "|" into exactly n parts (keys never contain "|").
func splitN(k string, n int) []string {
	out := make([]string, 0, n)
	start := 0
	for i := 0; i < len(k) && len(out) < n-1; i++ {
		if k[i] == '|' {
			out = append(out, k[start:i])
			start = i + 1
		}
	}
	out = append(out, k[start:])
	for len(out) < n {
		out = append(out, "")
	}
	return out
}
