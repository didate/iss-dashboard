package usage

import (
	"strings"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/quality"
)

// normalizeRHRoot merges irregular root codes that refer to the same profile.
func normalizeRHRoot(root string) string {
	// ISS_AUTRE_PROF_BENEV → root ISS_AUTRE_PROF, but ISS_AUTRE_PROFESSION_CONTR → root ISS_AUTRE_PROFESSION
	// Merge both into ISS_AUTRE_PROFESSION
	if strings.HasPrefix(root, "ISS_AUTRE_PROF") {
		return "ISS_AUTRE_PROFESSION"
	}
	return root
}

// RHDataElement is one RH data element resolved to its profile root and employment status.
type RHDataElement struct {
	UID    string
	Root   string // profile, e.g. ISS_RH_INF
	Statut string // fonc, contr, benev, asc, reco, other
	Label  string
}

// DiscoverRHProfiles lists the RH data elements (sections ISS_RH / ISS_RH_SPE,
// numeric) with their profile root, plus a display label per root. Shared by the
// RH aggregation and the demographic coverage ratios so both count the same DEs.
func DiscoverRHProfiles(ctx *quality.QualityContext) ([]RHDataElement, map[string]string) {
	var rhDEs []RHDataElement
	rootLabels := make(map[string]string)

	for _, de := range ctx.MetadataByUID {
		if de.SectionPrefix != "ISS_RH" && de.SectionPrefix != "ISS_RH_SPE" {
			continue
		}
		if de.ValueType == "BOOLEAN" {
			continue
		}

		code := de.Code
		var root, statut string

		switch {
		// Community health workers have no employment-status suffix; they are NOT
		// fonctionnaires. Give each its own statut so they never fall into the
		// default (fonc) bucket.
		case code == "ISS_RH_AGENT_COMM_DE":
			root = "ISS_RH_AGENT_COMM"
			statut = "asc"
		case code == "ISS_RH_RELAIS_COMM_DE":
			root = "ISS_RH_RELAIS_COMM"
			statut = "reco"
		case strings.HasSuffix(code, "_FN_DE"):
			root = strings.TrimSuffix(code, "_FN_DE")
			statut = "fonc"
		case strings.HasSuffix(code, "_CT_DE"):
			root = strings.TrimSuffix(code, "_CT_DE")
			statut = "contr"
		case strings.HasSuffix(code, "_BN_DE"):
			root = strings.TrimSuffix(code, "_BN_DE")
			statut = "benev"
		case strings.HasSuffix(code, "_FONC"):
			root = strings.TrimSuffix(code, "_FONC")
			statut = "fonc"
		case strings.HasSuffix(code, "_CONTR"):
			root = strings.TrimSuffix(code, "_CONTR")
			statut = "contr"
		case strings.HasSuffix(code, "_BENEV"):
			root = strings.TrimSuffix(code, "_BENEV")
			statut = "benev"
		default:
			root = code
			statut = "other"
		}

		// Normalize known irregular roots
		root = normalizeRHRoot(root)

		rhDEs = append(rhDEs, RHDataElement{UID: de.UID, Root: root, Statut: statut, Label: de.DisplayName()})

		// Build label from DE display name (strip the status suffix)
		if _, ok := rootLabels[root]; !ok {
			label := de.DisplayName()
			for _, suf := range []string{" — Fonctionnaire", " — Contractuel", " — Bénévole"} {
				label = strings.TrimSuffix(label, suf)
			}
			rootLabels[root] = label
		}
	}
	return rhDEs, rootLabels
}

// ComputeRH aggregates human resources by profile and employment status.
func ComputeRH(events []*models.Event, ctx *quality.QualityContext) []models.UsageRH {
	rhDEs, rootLabels := DiscoverRHProfiles(ctx)

	type counter struct {
		fonc, contr, benev, asc, reco int
	}
	accum := make(map[string]*counter) // root|district → counter
	ensure := func(root, district string) *counter {
		k := root + "|" + district
		if accum[k] == nil {
			accum[k] = &counter{}
		}
		return accum[k]
	}

	for _, evt := range events {
		vals := evt.Values()
		for _, rh := range rhDEs {
			v := quality.ParseNum(vals[rh.UID])
			if v == 0 {
				continue
			}
			iv := int(v)

			add := func(c *counter) {
				switch rh.Statut {
				case "fonc":
					c.fonc += iv
				case "contr":
					c.contr += iv
				case "benev":
					c.benev += iv
				case "asc":
					c.asc += iv
				case "reco":
					c.reco += iv
				default:
					c.fonc += iv // lump "other" into fonc
				}
			}

			add(ensure(rh.Root, "all"))
			if evt.District != "" {
				add(ensure(rh.Root, evt.District))
			}
		}
	}

	var result []models.UsageRH
	// Strip "ISS_RH " prefix from labels for cleaner display
	for root, label := range rootLabels {
		for _, prefix := range []string{"ISS_RH_SPE ", "ISS_RH "} {
			if strings.HasPrefix(label, prefix) {
				rootLabels[root] = strings.TrimPrefix(label, prefix)
				break
			}
		}
	}

	for key, c := range accum {
		parts := splitFirst(key, '|')
		root, district := parts[0], parts[1]
		result = append(result, models.UsageRH{
			ProfilCode:    root,
			Label:         rootLabels[root],
			District:      district,
			EffectifFonc:  c.fonc,
			EffectifContr: c.contr,
			EffectifBenev: c.benev,
			EffectifASC:   c.asc,
			EffectifRECO:  c.reco,
			EffectifTotal: c.fonc + c.contr + c.benev + c.asc + c.reco,
		})
	}
	return result
}
