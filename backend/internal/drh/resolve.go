package drh

import (
	"regexp"
	"sort"
	"strings"
)

// Familles d'affectation : où travaille l'agent.
const (
	AffStructure   = "structure"    // rattaché à une structure de soins ISS
	AffBureau      = "bureau"       // bureau de district (DPS / DCS / IRS / DSP)
	AffCentrale    = "centrale"     // administration centrale, programme national
	AffNonRattache = "non_rattache" // libellé non reconnu — reste visible
	KeyNational    = "national"
)

// Sources de rattachement, de la plus sûre à la plus déduite.
const (
	SrcTable       = "table"            // table de correspondance validée à la main
	SrcExact       = "exact"            // nom normalisé identique à une structure ISS
	SrcApprox      = "approx"           // type + nom propre, dans le district de l'agent
	SrcDeduit      = "deduit"           // seul établissement de ce type dans le district
	SrcPrefixe     = "prefixe"          // sigle de bureau de district ou d'administration centrale
	SrcService     = "service_district" // service hébergé par l'hôpital du district
	SrcNonRecensee = "non_recensee"     // structure connue de DHIS2, absente du recensement ISS
	SrcInconnu     = "inconnu"
)

// Structure is one ISS facility, as the resolver needs it.
type Structure struct {
	UID               string
	Name              string
	District          string
	Region            string
	TypeCode          string
	SousPrefecture    string
	SousPrefectureUID string
	// HorsRecensement marque une unité d'organisation que DHIS2 connaît sans
	// qu'ISS l'ait jamais visitée. Elle reste une structure valide — l'État y
	// affecte du personnel — mais n'entre pas dans l'appariement automatique.
	//
	// La valeur par défaut est donc « recensée » : une structure construite sans
	// ce champ se comporte normalement, au lieu de disparaître en silence de
	// l'appariement.
	HorsRecensement bool
}

// Correspondance is one manually validated DRH label → ISS facility mapping.
type Correspondance struct {
	LibelleNorm string `json:"libelle_norm"`
	LibelleDRH  string `json:"libelle_drh"`
	OrgUnitUID  string `json:"org_unit_uid"`
	Statut      string `json:"statut"` // ok | bureau_district | non_rattache | a_trancher
	District    string `json:"district"`
}

// Statuts d'une correspondance.
const (
	CorrOK          = "ok"
	CorrBureau      = "bureau_district"
	CorrNonRattache = "non_rattache"
	CorrATrancher   = "a_trancher"
)

// Affectation is where one agent was attached.
type Affectation struct {
	Kind     string // AffStructure | AffBureau | AffCentrale | AffNonRattache
	Key      string // uid de structure | district | "national"
	Label    string
	District string
	Region   string
	Source   string
}

// Inconnu is one unrecognised label, with the headcount behind it: the list to
// send back to the DRH.
type Inconnu struct {
	Libelle    string `json:"libelle"`
	Prefecture string `json:"prefecture"`
	NAgents    int    `json:"n_agents"`
}

// Report summarises one resolution pass.
type Report struct {
	NAgents         int            `json:"n_agents"`
	NStructure      int            `json:"n_structure"`
	NBureau         int            `json:"n_bureau"`
	NCentrale       int            `json:"n_centrale"`
	NNonRattache    int            `json:"n_non_rattache"`
	NStructuresVues int            `json:"n_structures_couvertes"`
	ParSource       map[string]int `json:"par_source"`
	Inconnus        []Inconnu      `json:"inconnus"`
}

// PctCategorise is the share of agents placed somewhere — attached, district
// office or central administration. It is the import's quality indicator.
func (r Report) PctCategorise() float64 {
	if r.NAgents == 0 {
		return 0
	}
	return 100 * float64(r.NAgents-r.NNonRattache) / float64(r.NAgents)
}

var (
	bureauPrefixe = regexp.MustCompile(`(?i)^(dps|dcs|drs|irs|dsp)\b`)
	// Les mêmes bureaux écrits en toutes lettres : la DRH alterne entre le
	// sigle et la forme longue d'une ligne à l'autre.
	bureauEnToutesLettres = regexp.MustCompile(`^(direction|inspection) (prefectoral|communal|regional)`)
	centralePrefixe       = regexp.MustCompile(`(?i)^(igs|bsd|drh|daf|dn[a-z]+|sn[a-z]+|pn[a-z-]+|ins[ep]|anss|cnts|pcg|lncqm|smsi|sge|prmp|fbr|sc[frpm]?|shsst|ipps|sp-|cnhd|lnsp|pev|dsvco|crems|mshp)\b`)
	districtPrefixe       = regexp.MustCompile(`(?i)^(dps|dcs|drs|irs|dsp)\s+`)
	// Sigles des sous-préfectures et communes : « CU Kassa » désigne la commune
	// que la DRH écrit simplement « Kassa ».
	sousPrefPrefixe = regexp.MustCompile(`(?i)^(cu|cr|cm)\s+`)
)

// motifs de type dans un libellé DRH, du plus spécifique au plus général.
var typePatterns = []struct {
	re   *regexp.Regexp
	code string
}{
	{regexp.MustCompile(`\bchu\b|centre hospitalier universitaire|hopital national|\bhn\b`), "HN"},
	{regexp.MustCompile(`\bhr\b|hopital regional|centre hospitalier regional`), "HR"},
	{regexp.MustCompile(`\bhp\b|hopital prefectoral|hopital communal|\bhopital\b`), "HP"},
	{regexp.MustCompile(`\bcmc\b|centre medico communal|centre medical communal`), "CMC"},
	{regexp.MustCompile(`\bcsa\b|centre de sante ameliore`), "CSA"},
	{regexp.MustCompile(`\bcsr\b|\bcsu\b|\bcsc\b|\bcs\b|centre de sante`), "CS"},
	{regexp.MustCompile(`\bps\b|poste de sante`), "PS"},
}

// typeHint devine le type de structure visé par un libellé DRH ("" si aucun).
func typeHint(label string) string {
	k := Norm(label)
	for _, p := range typePatterns {
		if p.re.MatchString(k) {
			return p.code
		}
	}
	return ""
}

// servicesDuDistrict liste les libellés qui désignent un service hébergé par
// l'hôpital du district, et non une entité nationale ni une structure à part
// entière. Ils ne peuvent pas passer par la table de correspondance : le même
// libellé existe dans plusieurs districts et doit se résoudre différemment
// dans chacun. Les motifs s'appliquent au libellé normalisé.
//
// Ajouter un service : une ligne ici, et un cas dans TestResolveServiceDuDistrict.
var servicesDuDistrict = []serviceDuDistrict{
	{
		// Centre de traitement des épidémies, hébergé par l'hôpital. Cinq
		// orthographes dans le seul millésime 2026 : CT-EPi, CTEPI, CT-Epi,
		// CTPI, CETPI.
		libelle: regexp.MustCompile(`^(ct|cet|cte) ?e?pi\b`),
	},
	{
		// Centre lèpre-tuberculose-ulcère, qui est lui une structure à part
		// entière : « Centre LTO de <district> », présent dans quatorze
		// districts.
		libelle: regexp.MustCompile(`\blto\b`),
		cible:   regexp.MustCompile(`^centre lto\b`),
	},
}

// serviceDuDistrict rattache un libellé à une structure du district de l'agent,
// designee par son nom et non par un appariement approximatif.
type serviceDuDistrict struct {
	libelle *regexp.Regexp // motif du libellé écrit par la DRH
	cible   *regexp.Regexp // motif du nom de la structure ; nil = l'hôpital du district
}

// typesHospitaliers, du plus spécifique au plus général : le service revient à
// l'hôpital préfectoral, à défaut régional, à défaut national.
var typesHospitaliers = []string{"HP", "HR", "HN"}

var premierToken = regexp.MustCompile(`^[A-Za-z-]+`)

// estSigle distingue un sigle d'administration d'un mot ordinaire : un sigle
// est majoritairement en majuscules (DNELM, PNLP, DSVCo, CT-EPi), pas un nom
// commun. Sans ce garde-fou, le motif des programmes nationaux (`pn[a-z-]+`)
// happait « Pneumologie » — un service hospitalier — et rangeait 30 agents du
// CHU dans les programmes nationaux.
func estSigle(label string) bool {
	tok := premierToken.FindString(strings.TrimSpace(label))
	var lettres, majuscules int
	for _, r := range tok {
		if r == '-' {
			continue
		}
		lettres++
		if r >= 'A' && r <= 'Z' {
			majuscules++
		}
	}
	if lettres < 2 || lettres > 12 {
		return false
	}
	return float64(majuscules)/float64(lettres) >= 0.6
}

// typeWords sont les mots de forme (type, article) : ce qui reste est le nom propre.
var typeWords = map[string]bool{
	"cs": true, "csr": true, "csu": true, "csa": true, "csc": true, "cmc": true, "ps": true,
	"hp": true, "hr": true, "hn": true, "chu": true, "centre": true, "sante": true,
	"hopital": true, "regional": true, "prefectoral": true, "communal": true, "rural": true,
	"urbain": true, "urbaine": true, "poste": true, "medico": true, "universitaire": true,
	"ameliore": true, "national": true, "de": true, "du": true, "des": true, "la": true,
	"le": true, "les": true, "et": true, "d": true,
}

// proper extracts the proper-noun tokens of a label ("CS de Koulé" → {koule}).
func proper(label string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(Norm(label)) {
		if !typeWords[w] && len(w) > 2 {
			out[w] = true
		}
	}
	return out
}

// typeCompatible accepte un type deviné voisin du type ISS : la DRH écrit
// souvent « CMC » pour un CSA, ou « hôpital » pour un HP comme pour un HR.
func typeCompatible(hint, code string) bool {
	if hint == "" || hint == code {
		return true
	}
	hospital := map[string]bool{"HP": true, "HR": true, "HN": true}
	centre := map[string]bool{"CS": true, "CSA": true, "CMC": true}
	return (hospital[hint] && hospital[code]) || (centre[hint] && centre[code])
}

// Resolver attaches DRH labels to ISS facilities.
type Resolver struct {
	corr        map[string]Correspondance // libellé seul, quand il est unique dans la table
	corrZone    map[string]Correspondance // libellé + district, pour les libellés ambigus
	byName      map[string]Structure      // nom normalisé → structure (première gagnante)
	byDistrict  map[string][]Structure    // district normalisé (sans DPS/DCS) → structures
	byUID       map[string]Structure
	districts   map[string]Structure // district normalisé → une structure du district (pour région)
	districtOf  map[string]string    // préfecture normalisée → libellé de district ISS
	viaSousPref map[string]string    // sous-préfecture normalisée → libellé de district ISS
}

// NewResolver indexes the ISS facilities and the correspondence table.
func NewResolver(structures []Structure, corr []Correspondance) *Resolver {
	r := &Resolver{
		corr:        make(map[string]Correspondance, len(corr)),
		corrZone:    make(map[string]Correspondance, len(corr)),
		byName:      make(map[string]Structure, len(structures)),
		byDistrict:  make(map[string][]Structure),
		byUID:       make(map[string]Structure, len(structures)),
		districts:   make(map[string]Structure),
		districtOf:  make(map[string]string),
		viaSousPref: make(map[string]string),
	}
	// Un même libellé peut désigner une structure différente selon le district
	// — « HOPITAL » à Fria n'est pas celui de Boffa. La colonne district de la
	// table dit où la règle s'applique : renseignée, elle limite la règle à ce
	// district ; vide, la règle vaut partout.
	//
	// C'est la seule lecture sûre. Rendre aussi global un libellé qui n'apparaît
	// qu'une fois paraissait commode, mais « HOPITAL », saisi pour Fria, partait
	// alors rattacher les hôpitaux de tous les autres districts.
	for _, c := range corr {
		key := corrKey(c)
		if d := normDistrict(c.District); d != "" {
			r.corrZone[key+"|"+d] = c
			continue
		}
		r.corr[key] = c
	}
	for _, s := range structures {
		// Toute unité connue de DHIS2 peut être la cible d'une correspondance
		// validée à la main. Seules les structures recensées entrent en revanche
		// dans l'appariement automatique : y verser les 4 000 unités du registre
		// multiplierait les candidats et volerait des rattachements aux vraies
		// structures, pour des noms que personne n'a vérifiés.
		r.byUID[s.UID] = s
		if s.HorsRecensement {
			continue
		}
		if k := Norm(s.Name); k != "" {
			if _, seen := r.byName[k]; !seen {
				r.byName[k] = s
			}
		}
		dk := normDistrict(s.District)
		r.byDistrict[dk] = append(r.byDistrict[dk], s)
		if _, seen := r.districts[dk]; !seen {
			r.districts[dk] = s
		}
		r.districtOf[dk] = s.District
	}
	// La DRH écrit parfois une commune là où ISS a un district — « LAMBANYI »
	// pour Ratoma, « Kassa » pour Kaloum. La hiérarchie DHIS2 sait les relier :
	// on indexe les sous-préfectures, en écartant celles dont le nom existe dans
	// deux districts, qui ne désigneraient rien de sûr.
	ambigu := map[string]bool{}
	for _, st := range structures {
		sp := normSousPref(st.SousPrefecture)
		if sp == "" || r.districtOf[sp] != "" {
			continue // déjà un district : le nom de district prime
		}
		if d, seen := r.viaSousPref[sp]; seen && d != st.District {
			ambigu[sp] = true
			continue
		}
		r.viaSousPref[sp] = st.District
	}
	for sp := range ambigu {
		delete(r.viaSousPref, sp)
	}
	return r
}

// normSousPref normalise un nom de sous-préfecture en retirant son sigle.
func normSousPref(sp string) string {
	return Norm(sousPrefPrefixe.ReplaceAllString(strings.TrimSpace(sp), ""))
}

// districtDe résout la préfecture d'un agent en district ISS, en passant au
// besoin par la sous-préfecture. Renvoie "" si rien ne correspond.
func (r *Resolver) districtDe(prefecture string) string {
	k := normDistrict(prefecture)
	if d, ok := r.districtOf[k]; ok {
		return d
	}
	return r.viaSousPref[k]
}

// corrKey is the normalised label a correspondence is indexed by.
func corrKey(c Correspondance) string {
	if c.LibelleNorm != "" {
		return c.LibelleNorm
	}
	return Norm(c.LibelleDRH)
}

// normDistrict strips the "DPS "/"DCS " prefix so that the DRH's "Boké"
// matches the ISS "DPS Boké".
func normDistrict(d string) string {
	return Norm(districtPrefixe.ReplaceAllString(strings.TrimSpace(d), ""))
}

// Resolve attaches one agent. It tries, in order: the correspondence table,
// an exact name match, the district-office and central-administration sigils,
// then type + proper noun within the agent's own district, with a deduction
// when the district holds a single facility of that type.
func (r *Resolver) Resolve(a AgentRow) Affectation {
	affectation := strings.TrimSpace(a.StructureAffectation)
	if affectation != "" {
		if aff, ok := r.resolveLabel(affectation, a); ok {
			return aff
		}
	}

	// Repli sur la structure de rattachement. Quand le libellé d'affectation
	// était renseigné mais non reconnu, ce repli n'est accepté que s'il aboutit
	// à une structure de soins : « CSU SIGUIRIKORO » rattaché à « DPS Siguiri »
	// désigne un centre de santé, pas le bureau du district. L'y ranger gonflait
	// le bureau — 292 agents à Siguiri, quatre fois la moyenne — et faisait
	// disparaître le libellé du rapport, donc de tout arbitrage possible.
	if rattachement := strings.TrimSpace(a.StructureRattachement); rattachement != "" {
		if aff, ok := r.resolveLabel(rattachement, a); ok {
			if affectation == "" || aff.Kind == AffStructure {
				return aff
			}
		}
	}

	return Affectation{Kind: AffNonRattache, Key: r.districtKey(a), Label: a.Libelle(),
		District: r.districtLabel(a), Region: r.regionLabel(a), Source: SrcInconnu}
}

func (r *Resolver) resolveLabel(label string, a AgentRow) (Affectation, bool) {
	k := Norm(label)
	c, ok := r.corrZone[k+"|"+normDistrict(r.districtDe(a.Prefecture))]
	if !ok {
		c, ok = r.corr[k]
	}
	if ok {
		switch c.Statut {
		case CorrOK:
			if s, ok := r.byUID[c.OrgUnitUID]; ok {
				// Une structure absente du recensement reste une structure : on
				// y rattache l'agent, et la source dit que le recensement ISS
				// ne la couvre pas — ce qui est un constat à remonter, pas une
				// raison de perdre l'effectif.
				if s.HorsRecensement {
					return r.structureAff(s, SrcNonRecensee), true
				}
				return r.structureAff(s, SrcTable), true
			}
		case CorrBureau:
			return r.bureauAff(a, SrcTable), true
		case CorrNonRattache, CorrATrancher:
			return Affectation{Kind: AffNonRattache, Key: r.districtKey(a), Label: label,
				District: r.districtLabel(a), Region: r.regionLabel(a), Source: SrcTable}, true
		}
	}
	if s, ok := r.byName[k]; ok {
		return r.structureAff(s, SrcExact), true
	}
	if bureauPrefixe.MatchString(strings.TrimSpace(label)) || bureauEnToutesLettres.MatchString(k) {
		return r.bureauAff(a, SrcPrefixe), true
	}
	if s, ok := r.hopitalDuDistrict(label, a); ok {
		return r.structureAff(s, SrcService), true
	}
	if centralePrefixe.MatchString(strings.TrimSpace(label)) && estSigle(label) {
		// Chaque direction, institut ou programme garde sa propre clé : sans
		// cela, 875 agents se retrouvaient dans un bloc « administration
		// centrale » indistinct, alors qu'ils se répartissent sur 42 entités.
		return Affectation{Kind: AffCentrale, Key: Norm(label), Label: strings.TrimSpace(label), Source: SrcPrefixe}, true
	}

	pool := r.byDistrict[normDistrict(r.districtDe(a.Prefecture))]
	if len(pool) == 0 {
		return Affectation{}, false
	}
	hint, pn := typeHint(label), proper(label)
	if len(pn) == 0 {
		// « Hôpital préfectoral » sans nom : admissible si le district n'en a qu'un.
		if hint != "HP" && hint != "HR" && hint != "HN" {
			return Affectation{}, false
		}
		var hits []Structure
		for _, s := range pool {
			if typeCompatible(hint, s.TypeCode) {
				hits = append(hits, s)
			}
		}
		if len(hits) == 1 {
			return r.structureAff(hits[0], SrcDeduit), true
		}
		return Affectation{}, false
	}
	var hits []Structure
	for _, s := range pool {
		if !typeCompatible(hint, s.TypeCode) {
			continue
		}
		sp := proper(s.Name)
		if shareToken(pn, sp) && (subset(pn, sp) || subset(sp, pn)) {
			hits = append(hits, s)
		}
	}
	switch len(hits) {
	case 0:
		return Affectation{}, false
	case 1:
		return r.structureAff(hits[0], SrcApprox), true
	}
	// Plusieurs candidats : le type exact l'emporte sur le type voisin, puis le
	// nom propre strictement identique. Ce qui reste ambigu n'est pas rattaché
	// au hasard : il part à l'arbitrage humain, via la table de correspondance.
	if hits = narrow(hits, func(s Structure) bool { return s.TypeCode == hint }); len(hits) == 1 {
		return r.structureAff(hits[0], SrcApprox), true
	}
	if hits = narrow(hits, func(s Structure) bool { return equalSet(proper(s.Name), pn) }); len(hits) == 1 {
		return r.structureAff(hits[0], SrcApprox), true
	}
	return Affectation{}, false
}

// hopitalDuDistrict rattache un service du district à son hôpital, quand le
// district n'en compte qu'un seul du type visé. Deux hôpitaux du même type ne
// se départagent pas : l'agent part à l'arbitrage plutôt qu'au hasard.
func (r *Resolver) hopitalDuDistrict(label string, a AgentRow) (Structure, bool) {
	k := Norm(label)
	var regle *serviceDuDistrict
	for i := range servicesDuDistrict {
		if servicesDuDistrict[i].libelle.MatchString(k) {
			regle = &servicesDuDistrict[i]
			break
		}
	}
	if regle == nil {
		return Structure{}, false
	}
	pool := r.byDistrict[normDistrict(r.districtDe(a.Prefecture))]

	// Structure designee par son nom : « LTO » → le Centre LTO du district.
	if regle.cible != nil {
		var hits []Structure
		for _, s := range pool {
			if regle.cible.MatchString(Norm(s.Name)) {
				hits = append(hits, s)
			}
		}
		if len(hits) == 1 {
			return hits[0], true
		}
		return Structure{}, false
	}

	// Sinon l'hôpital du district : préfectoral, à défaut régional, à défaut
	// national. Deux hôpitaux du même type ne se départagent pas.
	for _, typeCode := range typesHospitaliers {
		var hits []Structure
		for _, s := range pool {
			if s.TypeCode == typeCode {
				hits = append(hits, s)
			}
		}
		if len(hits) == 1 {
			return hits[0], true
		}
		if len(hits) > 1 {
			return Structure{}, false
		}
	}
	return Structure{}, false
}

func (r *Resolver) structureAff(s Structure, src string) Affectation {
	return Affectation{Kind: AffStructure, Key: s.UID, Label: s.Name,
		District: s.District, Region: s.Region, Source: src}
}

func (r *Resolver) bureauAff(a AgentRow, src string) Affectation {
	label := r.districtLabel(a)
	if label == "" {
		label = a.Prefecture
	}
	return Affectation{Kind: AffBureau, Key: r.districtKey(a), Label: "Bureau de district — " + label,
		District: label, Region: r.regionLabel(a), Source: src}
}

// districtKey identifies the agent's district; the DRH prefecture is the only
// thing available, so an unknown one falls back to its normalised form.
func (r *Resolver) districtKey(a AgentRow) string {
	if d := r.districtDe(a.Prefecture); d != "" {
		return normDistrict(d)
	}
	if k := normDistrict(a.Prefecture); k != "" {
		return k
	}
	return KeyNational
}

// regionLabel returns the ISS region of the agent's district. The DRH writes
// its own region names ("BOKE") : keeping them would split every regional
// aggregate in two.
//
// A préfecture ISS does not know (the 2026 file has one, "Kassa") yields no
// region at all rather than a phantom one. Those agents stay visible in the
// district table, under their DRH label and without a density, which is the
// signal that the zone needs arbitration.
func (r *Resolver) regionLabel(a AgentRow) string {
	if s, ok := r.districts[normDistrict(r.districtDe(a.Prefecture))]; ok {
		return s.Region
	}
	return ""
}

func (r *Resolver) districtLabel(a AgentRow) string {
	if d := r.districtDe(a.Prefecture); d != "" {
		return d
	}
	return strings.TrimSpace(a.Prefecture)
}

// ResolveAll attaches every agent and builds the import report.
func ResolveAll(rows []AgentRow, r *Resolver) ([]Affectation, Report) {
	affs := make([]Affectation, len(rows))
	rep := Report{NAgents: len(rows), ParSource: map[string]int{}}
	seen := map[string]bool{}
	inconnus := map[Inconnu]int{}
	for i, a := range rows {
		aff := r.Resolve(a)
		affs[i] = aff
		rep.ParSource[aff.Source]++
		switch aff.Kind {
		case AffStructure:
			rep.NStructure++
			seen[aff.Key] = true
		case AffBureau:
			rep.NBureau++
		case AffCentrale:
			rep.NCentrale++
		default:
			rep.NNonRattache++
			inconnus[Inconnu{Libelle: a.Libelle(), Prefecture: a.Prefecture}]++
		}
	}
	rep.NStructuresVues = len(seen)
	for k, n := range inconnus {
		k.NAgents = n
		rep.Inconnus = append(rep.Inconnus, k)
	}
	sort.Slice(rep.Inconnus, func(i, j int) bool {
		if rep.Inconnus[i].NAgents != rep.Inconnus[j].NAgents {
			return rep.Inconnus[i].NAgents > rep.Inconnus[j].NAgents
		}
		return rep.Inconnus[i].Libelle < rep.Inconnus[j].Libelle
	})
	return affs, rep
}

// narrow keeps the candidates matching a finer criterion, unless none does.
func narrow(hits []Structure, keep func(Structure) bool) []Structure {
	var out []Structure
	for _, s := range hits {
		if keep(s) {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return hits
	}
	return out
}

func shareToken(a, b map[string]bool) bool {
	for k := range a {
		if b[k] {
			return true
		}
	}
	return false
}

func subset(a, b map[string]bool) bool {
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func equalSet(a, b map[string]bool) bool { return len(a) == len(b) && subset(a, b) }
