package drh

import (
	"sort"
	"strings"
)

// Familles d'affectation : où travaille l'agent.
const (
	AffStructure      = "structure"       // structure de soins
	AffBureau         = "bureau"          // bureau de district (DPS, DCS)
	AffBureauRegional = "bureau_regional" // inspection régionale, DSV Conakry
	AffCentrale       = "centrale"        // administration centrale, programme national
	AffNonRattache    = "non_rattache"    // aucun identifiant exploitable
	KeyNational       = "national"
)

// Sources de rattachement. Il n'en reste qu'une : le fichier.
const (
	SrcFichier = "fichier"
	SrcInconnu = "inconnu"
)

// UniteOrg is one DHIS2 organisation unit, whatever its level: the root, a
// region, a district, a facility. The file points at one by its identifier and
// the level decides what kind of posting it is.
type UniteOrg struct {
	UID               string
	Name              string
	Level             int
	District          string
	Region            string
	SousPrefecture    string
	SousPrefectureUID string
	TypeCode          string
	// HorsRecensement : connue de DHIS2, jamais visitée par le recensement ISS.
	HorsRecensement bool
}

// Affectation is where one agent was posted.
type Affectation struct {
	Kind     string // AffStructure | AffBureau | AffBureauRegional | AffCentrale | AffNonRattache
	Key      string // uid de structure | district | région | entité centrale | "national"
	Label    string
	District string
	Region   string
	Source   string
}

// Inconnu is one unresolved label, with the headcount behind it: the list to
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
	NBureauRegional int            `json:"n_bureau_regional"`
	NCentrale       int            `json:"n_centrale"`
	NNonRattache    int            `json:"n_non_rattache"`
	NStructuresVues int            `json:"n_structures_couvertes"`
	ParSource       map[string]int `json:"par_source"`
	Inconnus        []Inconnu      `json:"inconnus"`
}

// PctCategorise is the share of agents the file could place.
func (r Report) PctCategorise() float64 {
	if r.NAgents == 0 {
		return 0
	}
	return 100 * float64(r.NAgents-r.NNonRattache) / float64(r.NAgents)
}

// Resolver places an agent from the identifier carried by the file.
//
// Il n'y a plus d'appariement : ni nom approché, ni sigle, ni déduction. Le
// fichier porte l'identifiant de l'unité d'organisation, et le niveau de cette
// unité dans la hiérarchie DHIS2 dit de quel genre d'affectation il s'agit.
// Ce qui n'a pas d'identifiant reste non rattaché et remonte dans le rapport,
// pour être tranché à la source plutôt que deviné ici.
type Resolver struct {
	parUID map[string]UniteOrg
}

// NewResolver indexes the organisation units by identifier.
func NewResolver(unites []UniteOrg) *Resolver {
	r := &Resolver{parUID: make(map[string]UniteOrg, len(unites))}
	for _, u := range unites {
		r.parUID[u.UID] = u
	}
	return r
}

// Resolve places one agent.
func (r *Resolver) Resolve(a AgentRow) Affectation {
	uid := strings.TrimSpace(a.UIDDhis2)
	if uid == "" {
		return r.nonRattache(a)
	}
	u, ok := r.parUID[uid]
	if !ok {
		return r.nonRattache(a)
	}

	switch {
	case u.Level <= 1:
		// L'unité racine porte les directions, instituts et programmes
		// nationaux : ils ne relèvent d'aucune zone, et le fichier leur donne
		// à tous le même identifiant. C'est donc le libellé de la ligne qui
		// sépare les entités — sans quoi 818 agents formeraient un bloc
		// « Guinée » indistinct, alors qu'ils se répartissent sur 45 entités.
		entite := centraleLabel(a, u)
		return Affectation{Kind: AffCentrale, Key: Norm(entite), Label: entite, Source: SrcFichier}
	case u.Level == 2:
		// Les cadres d'une inspection régionale restent au niveau région : les
		// verser dans un district en gonflerait un au hasard.
		return Affectation{Kind: AffBureauRegional, Key: u.Name, Label: u.Name, Region: u.Name, Source: SrcFichier}
	case u.Level == 3:
		return Affectation{Kind: AffBureau, Key: u.Name, Label: "Bureau de district — " + u.Name,
			District: u.Name, Region: u.Region, Source: SrcFichier}
	default:
		return Affectation{Kind: AffStructure, Key: u.UID, Label: u.Name,
			District: u.District, Region: u.Region, Source: SrcFichier}
	}
}

// centraleLabel names the directorate, institute or national programme the
// agent belongs to. The file's own label carries it; the root unit's name is
// the last resort, for the few lines that leave both columns empty.
func centraleLabel(a AgentRow, u UniteOrg) string {
	if l := a.Libelle(); l != "" {
		return l
	}
	return u.Name
}

// nonRattache place l'agent nulle part : sans identifiant, sa zone est
// inconnue. La préfecture et la région écrites par la DRH ne sont pas celles
// d'ISS — les reprendre fabriquerait des districts et des régions fantômes à
// côté des vrais. L'agent compte au national et ressort dans le rapport, avec
// son libellé et sa préfecture, pour être tranché à la source.
func (r *Resolver) nonRattache(a AgentRow) Affectation {
	return Affectation{Kind: AffNonRattache, Key: KeyNational, Label: a.Libelle(), Source: SrcInconnu}
}

// ResolveAll places every agent and builds the import report.
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
		case AffBureauRegional:
			rep.NBureauRegional++
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
