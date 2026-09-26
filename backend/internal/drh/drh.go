// Package drh ingère le fichier annuel du personnel de la fonction publique
// santé (DRH/CNPS) et le transforme en agrégats dépersonnalisés.
//
// Le fichier source est nominatif ; la carte sanitaire n'en a besoin qu'en
// effectifs. L'import lit un CSV normalisé (docs/drh-format.md) qui ne contient
// déjà ni matricule ni nom, en dérive les agrégats, et ne persiste que ceux-ci :
// aucune ligne par agent n'entre en base. Seule l'année de naissance est lue,
// pour la tranche quinquennale et les départs à la retraite.
//
// Le pipeline est : ParseCSV → Resolve (rattachement à une structure ISS) →
// Aggregate (effectifs et pyramide des âges).
package drh

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// AgentRow is one line of the normalised CSV: an agent, without anything that
// could identify them.
type AgentRow struct {
	Region                string
	Prefecture            string
	SousPrefecture        string
	StructureAffectation  string
	StructureRattachement string
	Profession            string
	ProfessionOMS         string
	Hierarchie            string
	Statut                string
	Sexe                  string // "F" | "H" | ""
	AnneeNaissance        int    // 0 = inconnue
	Zone                  string // urbaine | rurale | ""
	NiveauStructure       string // primaire | secondaire | tertiaire | ""
	// UIDDhis2 est l'identifiant de l'unité d'organisation où travaille
	// l'agent. C'est la seule chose qui détermine son rattachement.
	UIDDhis2 string
	NomDhis2 string // informatif, pour la relecture humaine du fichier
}

// Libelle returns the label used to attach the agent to a facility: the posting
// label, falling back to the administrative one.
func (a AgentRow) Libelle() string {
	if s := strings.TrimSpace(a.StructureAffectation); s != "" {
		return s
	}
	return strings.TrimSpace(a.StructureRattachement)
}

// Options carry the ingestion parameters that have no official reference and
// must stay configurable.
type Options struct {
	RefYear       int // année de référence des âges (millésime du fichier)
	RetirementAge int // âge de départ à la retraite (DRH_AGE_RETRAITE, 60 par défaut)
}

// Normalize fills in the defaults.
func (o Options) Normalize(defaultYear int) Options {
	if o.RefYear <= 0 {
		o.RefYear = defaultYear
	}
	if o.RetirementAge <= 0 {
		o.RetirementAge = 60
	}
	return o
}

// TrancheInconnue is the age bracket of agents whose birth year is missing.
// They are counted in the headcounts but excluded from the age pyramid's
// readable brackets and from the retirement projections.
const TrancheInconnue = "inconnu"

// Tranches lists the quinquennial brackets, in display order.
var Tranches = []string{"<25", "25-29", "30-34", "35-39", "40-44", "45-49", "50-54", "55-59", "60+", TrancheInconnue}

// Tranche returns the quinquennial bracket of an age (-1 = unknown).
func Tranche(age int) string {
	switch {
	case age < 0:
		return TrancheInconnue
	case age < 25:
		return "<25"
	case age >= 60:
		return "60+"
	default:
		lo := age / 5 * 5
		return fmt.Sprintf("%d-%d", lo, lo+4)
	}
}

// Age returns the agent's age in the reference year, or -1 when unknown.
func (a AgentRow) Age(refYear int) int {
	if a.AnneeNaissance <= 0 {
		return -1
	}
	return refYear - a.AnneeNaissance
}

// PartDansMoinsDe reports whether the agent reaches the retirement age within
// the next n years. Agents already past it count too: they are, in practice,
// posts about to be vacated.
func (a AgentRow) PartDansMoinsDe(n int, o Options) bool {
	age := a.Age(o.RefYear)
	return age >= 0 && age+n >= o.RetirementAge
}

// --- Normalisation des libellés ---------------------------------------------

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

var deaccent = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// Norm lowercases, strips accents and collapses everything else to single
// spaces. Every comparison of a DRH label to an ISS name goes through it.
func Norm(s string) string {
	out, _, err := transform.String(deaccent, strings.ToLower(s))
	if err != nil {
		out = strings.ToLower(s)
	}
	return strings.TrimSpace(nonAlnum.ReplaceAllString(out, " "))
}
