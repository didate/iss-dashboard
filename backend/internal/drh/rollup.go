package drh

import (
	"sort"

	"iss-dashboard-backend/internal/typologie"
)

// Dimensions des rollups, calculées à partir du grain fin.
const (
	DimGlobal         = "global"
	DimRegion         = "region"
	DimDistrict       = "district"
	DimSousPrefecture = "sous_prefecture"
	DimType           = "type"
)

// RollupDimensions lists the dimensions Rollup produces, so the store knows
// exactly what to replace without touching the cells written at import time.
var RollupDimensions = []string{DimGlobal, DimRegion, DimDistrict, DimSousPrefecture, DimType}

// FineDimensions lists the cells written by Aggregate, never recomputed here.
// Le store filtre dessus : cette liste est la source unique, pour qu'ajouter un
// genre d'affectation ne laisse pas des cellules invisibles en base.
var FineDimensions = []string{AffStructure, AffBureau, AffBureauRegional, AffCentrale, AffNonRattache}

var fineDimensions = func() map[string]bool {
	m := make(map[string]bool, len(FineDimensions))
	for _, d := range FineDimensions {
		m[d] = true
	}
	return m
}()

// RollupContext carries what the rollups need beyond the cells themselves.
type RollupContext struct {
	// Unites resolves an org unit UID to its sous-préfecture and type.
	Unites map[string]UniteOrg
	// Population is keyed "dimension|key" (only global, region, district and
	// sous_prefecture carry one). A missing or zero entry leaves the ratio nil.
	Population map[string]float64
}

// PopKey is the key used in RollupContext.Population.
func PopKey(dimension, key string) string { return dimension + "|" + key }

// Rollup aggregates the fine-grained cells into the dimensions the screens
// read: national, region, district, sous-préfecture and structure type.
//
// Who counts where:
//   - a district (and its region) counts the agents of its facilities, of its
//     district office, and those whose posting label could not be attached —
//     they are real agents of that préfecture, and dropping them would
//     understate the district;
//   - the central administration counts nationally only: it is not located in
//     the district whose address it happens to have;
//   - a regional office counts in its region but in no district: it carries no
//     district, and an empty key is skipped, so the rollup excludes it of
//     itself rather than by a special case;
//   - sous-préfecture and type only concern agents attached to a facility, the
//     only ones whose exact location and type are known. District offices are
//     therefore absent from those two dimensions.
func Rollup(eff []EffectifRow, pyr []PyramideRow, ctx RollupContext) ([]EffectifRow, []PyramideRow) {
	effOut := map[cellKey]*EffectifRow{}
	pyrOut := map[pyrKey]*PyramideRow{}

	addEff := func(dim, key, label, district, region string, src EffectifRow) {
		if key == "" {
			return
		}
		ck := cellKey{dim, key, src.Categorie}
		row, ok := effOut[ck]
		if !ok {
			row = &EffectifRow{Dimension: dim, Key: key, Label: label, District: district,
				Region: region, Categorie: src.Categorie}
			effOut[ck] = row
		}
		row.NAgents += src.NAgents
		row.NFemmes += src.NFemmes
		row.NDepart5Ans += src.NDepart5Ans
		row.NDepart10Ans += src.NDepart10Ans
		row.NAgeConnu += src.NAgeConnu
		switch src.Dimension {
		case AffStructure:
			row.NStructure += src.NAgents
		case AffBureau:
			row.NBureau += src.NAgents
		case AffBureauRegional:
			row.NBureauRegional += src.NAgents
		case AffCentrale:
			row.NCentrale += src.NAgents
		case AffNonRattache:
			row.NNonRattache += src.NAgents
		}
	}

	for _, r := range eff {
		if !fineDimensions[r.Dimension] {
			continue // déjà un rollup : on ne cumule jamais un cumul
		}
		addEff(DimGlobal, KeyNational, "Guinée", "", "", r)
		if r.Dimension == AffCentrale {
			continue
		}
		addEff(DimRegion, r.Region, r.Region, "", r.Region, r)
		addEff(DimDistrict, r.District, r.District, r.District, r.Region, r)
		if r.Dimension != AffStructure {
			continue
		}
		s, ok := ctx.Unites[r.Key]
		if !ok {
			continue
		}
		addEff(DimSousPrefecture, s.SousPrefectureUID, s.SousPrefecture, s.District, s.Region, r)
		addEff(DimType, s.TypeCode, typologie.Label(s.TypeCode), "", "", r)
	}

	addPyr := func(dim, key string, src PyramideRow) {
		if key == "" {
			return
		}
		pk := pyrKey{dim, key, src.Categorie, src.Tranche}
		row, ok := pyrOut[pk]
		if !ok {
			row = &PyramideRow{Dimension: dim, Key: key, Categorie: src.Categorie, Tranche: src.Tranche}
			pyrOut[pk] = row
		}
		row.NAgents += src.NAgents
		row.NFemmes += src.NFemmes
	}

	// La pyramide n'est cumulée que là où elle se lit : national, région,
	// district et type. À la sous-préfecture, les effectifs sont trop petits
	// pour qu'une pyramide par tranche veuille dire quoi que ce soit.
	place := map[string]struct{ region, district, typeCode string }{}
	for _, r := range eff {
		if !fineDimensions[r.Dimension] || r.Categorie != CategorieToutes {
			continue
		}
		p := struct{ region, district, typeCode string }{r.Region, r.District, ""}
		if r.Dimension == AffStructure {
			if s, ok := ctx.Unites[r.Key]; ok {
				p.typeCode = s.TypeCode
			}
		}
		place[r.Dimension+"|"+r.Key] = p
	}
	for _, r := range pyr {
		if !fineDimensions[r.Dimension] {
			continue
		}
		addPyr(DimGlobal, KeyNational, r)
		if r.Dimension == AffCentrale {
			continue
		}
		p := place[r.Dimension+"|"+r.Key]
		addPyr(DimRegion, p.region, r)
		addPyr(DimDistrict, p.district, r)
		if p.typeCode != "" {
			addPyr(DimType, p.typeCode, r)
		}
	}

	rows := make([]EffectifRow, 0, len(effOut))
	for _, r := range effOut {
		if pop, ok := ctx.Population[PopKey(r.Dimension, r.Key)]; ok && pop > 0 {
			ratio := float64(r.NAgents) / pop * 10000
			p := pop
			r.Population, r.Ratio10k = &p, &ratio
		}
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool { return lessEffectif(rows[i], rows[j]) })

	pyrRows := make([]PyramideRow, 0, len(pyrOut))
	for _, r := range pyrOut {
		pyrRows = append(pyrRows, *r)
	}
	sort.Slice(pyrRows, func(i, j int) bool { return lessPyramide(pyrRows[i], pyrRows[j]) })
	return rows, pyrRows
}

// --- Comparaison DRH ↔ ISS --------------------------------------------------

// ISSRH is one ISS headcount: what the facilities declare, all statuses mixed.
type ISSRH struct {
	District   string // "all" pour le national
	ProfilCode string
	Effectif   float64
}

// ComparaisonRow confronts the two sources for one place and one category.
type ComparaisonRow struct {
	Dimension string `json:"dimension"` // global | district
	Key       string `json:"key"`
	Label     string `json:"label"`
	Categorie string `json:"categorie"`
	// NDrh ne compte que les agents affectés en structure de soins : ISS
	// n'interroge que les structures, et mettre en face de ses déclarations des
	// agents de bureau comparerait deux périmètres différents.
	NDrh  int      `json:"n_drh"`
	NIss  *float64 `json:"n_iss,omitempty"`
	Ecart *float64 `json:"ecart,omitempty"` // ISS − DRH
	Ratio *float64 `json:"ratio,omitempty"` // ISS / DRH

	// Aligne indique que les deux nomenclatures se recouvrent pour cette
	// catégorie : au national, le fichier DRH n'en compte pas plus que les structures
	// n'en déclarent. Sinon les deux sources ne comptent pas la même chose et
	// leur rapport n'est pas une part.
	Aligne bool `json:"aligne"`
	// PartEtat = 100 × DRH ÷ ISS, la part du personnel déclaré qui figure au fichier DRH.
	// Renseignée pour les seules catégories alignées : ailleurs le mot « part »
	// n'a pas de sens, un rapport supérieur à 100 % n'étant pas une proportion.
	PartEtat *float64 `json:"part_etat,omitempty"`
}

// Compare confronts the DRH/CNPS file with what the facilities declare in ISS.
//
// Les deux sources ne comptent pas la même population : ISS compte tout le
// personnel présent déclaré par la structure, le fichier DRH les seuls agents
// de la fonction publique. Un ratio supérieur à 1 est donc normal et mesure la
// part de personnel hors fonction publique ; un ratio inférieur à 1 est une
// anomalie — défaut de déclaration, ou agents affectés mais absents.
//
// Le périmètre, lui, doit être le même des deux côtés. ISS n'interroge que les
// structures de soins : on ne retient donc du fichier DRH que les agents qui y
// sont affectés, à l'exclusion des bureaux de district, des inspections
// régionales et de l'administration centrale, qu'aucune structure n'a jamais
// déclarés. Seules les catégories ayant un équivalent ISS sont comparées, et la
// ligne « toutes catégories » somme les mêmes des deux côtés.
func Compare(eff []EffectifRow, iss []ISSRH) []ComparaisonRow {
	issByProfil := map[string]map[string]float64{} // district → profil → effectif
	for _, r := range iss {
		d := issByProfil[r.District]
		if d == nil {
			d = map[string]float64{}
			issByProfil[r.District] = d
		}
		d[r.ProfilCode] += r.Effectif
	}
	issProfil := map[string]string{} // catégorie → profil ISS
	for _, c := range Categories {
		if c.ISS != "" {
			issProfil[c.Code] = c.ISS
		}
	}

	// Périmètre comparable : une catégorie n'est retenue que si, au national,
	// le fichier DRH n'en compte pas plus que les structures n'en déclarent.
	// Au-delà de 100 % le rapport n'est plus une part et gonflerait le total
	// sans rien mesurer. Deux causes s'y mêlent : ces métiers sont surtout
	// affectés en bureau, qu'ISS n'interroge pas, et certains intitulés ne
	// recouvrent pas la même chose des deux côtés — « Médecin Spécialiste en
	// Santé Publique » est courant côté DRH, presque jamais coché dans ISS.
	//
	// La décision est prise une fois, au national, et s'applique telle quelle à
	// chaque zone : le périmètre reste identique partout, donc les zones se
	// comparent entre elles. Un district où le fichier DRH dépasse le déclaré reste
	// visible dans ce périmètre — c'est une anomalie, pas un artefact.
	aligne := map[string]bool{}
	for _, r := range eff {
		if r.Dimension != DimGlobal {
			continue
		}
		if profil, ok := issProfil[r.Categorie]; ok {
			n := issByProfil["all"][profil]
			aligne[r.Categorie] = n > 0 && float64(r.NStructure) <= n
		}
	}

	out := map[cellKey]*ComparaisonRow{}
	add := func(dim, key, label, cat string, nDrh int, nIss float64, hasIss bool) {
		ck := cellKey{dim, key, cat}
		row, ok := out[ck]
		if !ok {
			row = &ComparaisonRow{Dimension: dim, Key: key, Label: label, Categorie: cat}
			out[ck] = row
		}
		row.NDrh += nDrh
		if hasIss {
			v := nIss
			if row.NIss != nil {
				v += *row.NIss
			}
			row.NIss = &v
		}
	}

	for _, r := range eff {
		if r.Dimension != DimGlobal && r.Dimension != DimDistrict {
			continue
		}
		profil, comparable := issProfil[r.Categorie]
		if !comparable {
			continue
		}
		issKey := "all"
		if r.Dimension == DimDistrict {
			issKey = r.Key
		}
		n, hasIss := issByProfil[issKey][profil]
		add(r.Dimension, r.Key, r.Label, r.Categorie, r.NStructure, n, hasIss)
		if hasIss && aligne[r.Categorie] {
			// La ligne « toutes catégories » n'agrège que ce qu'ISS a renseigné,
			// sinon le total DRH couvrirait des métiers absents de l'autre côté
			// et l'écart mesurerait ce trou plutôt que la réalité.
			add(r.Dimension, r.Key, r.Label, CategorieToutes, r.NStructure, n, true)
		}
	}

	rows := make([]ComparaisonRow, 0, len(out))
	for _, r := range out {
		r.Aligne = r.Categorie == CategorieToutes || aligne[r.Categorie]
		if r.NIss != nil {
			ecart := *r.NIss - float64(r.NDrh)
			r.Ecart = &ecart
			if r.NDrh > 0 {
				ratio := *r.NIss / float64(r.NDrh)
				r.Ratio = &ratio
			}
			if r.Aligne && *r.NIss > 0 {
				part := 100 * float64(r.NDrh) / *r.NIss
				r.PartEtat = &part
			}
		}
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		switch {
		case a.Dimension != b.Dimension:
			return a.Dimension < b.Dimension
		case a.Key != b.Key:
			return a.Key < b.Key
		default:
			return a.Categorie < b.Categorie
		}
	})
	return rows
}
