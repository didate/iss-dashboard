package drh

import "strings"

// Familles de professions, pour les lectures de haut niveau.
const (
	FamilleSoignant  = "soignant"
	FamilleTechnique = "technique"
	FamilleSupport   = "support"
)

// Categorie regroupe les intitulés de métier de la DRH (112 libellés, avec
// variantes de casse et coquilles) en catégories stables, alignées sur les
// profils RH du formulaire ISS quand un équivalent existe — c'est ce qui rend
// la comparaison DRH ↔ ISS possible.
type Categorie struct {
	Code    string `json:"code"`
	Label   string `json:"label"`
	Famille string `json:"famille"`
	ISS     string `json:"iss,omitempty"` // profil_code ISS comparable ("" si aucun)
}

// Categories is the ordered catalogue (display order: care first).
var Categories = []Categorie{
	{"MED_GEN", "Médecin généraliste", FamilleSoignant, "ISS_RH_MED_GEN"},
	{"MED_CHIR", "Médecin chirurgien", FamilleSoignant, "ISS_RH_MED_CHIR"},
	{"MED_PED", "Médecin pédiatre", FamilleSoignant, "ISS_RH_MED_PED"},
	{"MED_GYNE", "Médecin gynécologue", FamilleSoignant, "ISS_RH_MED_GYNE"},
	{"MED_ANESTH", "Médecin anesthésiste", FamilleSoignant, "ISS_RH_MED_ANESTH"},
	{"MED_SP_PUB", "Médecin santé publique", FamilleSoignant, "ISS_RH_MED_SP_PUB"},
	{"MED_AUTRE", "Autre médecin spécialiste", FamilleSoignant, "ISS_RH_MED_AUTRE"},
	{"DENT", "Chirurgien-dentiste", FamilleSoignant, "ISS_RH_DENT"},
	{"PHARM", "Pharmacien", FamilleSoignant, "ISS_RH_PHARM"},
	{"BIO", "Biologiste", FamilleSoignant, "ISS_RH_BIO"},
	{"TECH_LAB", "Technicien de laboratoire", FamilleSoignant, "ISS_RH_TECH_LAB"},
	{"INF", "Infirmier", FamilleSoignant, "ISS_RH_INF"},
	{"SAGEF", "Sage-femme", FamilleSoignant, "ISS_RH_SAGEF"},
	{"ATS", "ATS", FamilleSoignant, "ISS_RH_ATS"},
	{"AIDE_SOIN", "Aide-soignant", FamilleSoignant, "ISS_RH_AIDE_SOIN"},
	{"AUTRE_SANTE", "Autre personnel de santé", FamilleSoignant, ""},
	{"BIOMED", "Ingénieur / technicien biomédical", FamilleTechnique, "ISS_RH_BIOMED"},
	{"STAT", "Statisticien", FamilleTechnique, "ISS_RH_STAT"},
	{"INFO", "Informaticien", FamilleTechnique, "ISS_INFORMATICIEN"},
	{"ADMIN", "Personnel administratif", FamilleSupport, "ISS_RH_ADMIN"},
	{"CHAUFFEUR", "Chauffeur", FamilleSupport, "ISS_RH_CHAUFFEUR"},
	{"SECU", "Agent de sécurité", FamilleSupport, "ISS_RH_SECU"},
	{"AUTRE", "Autre / non renseignée", FamilleSupport, ""},
}

var categorieByCode = func() map[string]Categorie {
	m := make(map[string]Categorie, len(Categories))
	for _, c := range Categories {
		m[c.Code] = c
	}
	return m
}()

// CategorieByCode returns a category and whether it exists.
func CategorieByCode(code string) (Categorie, bool) {
	c, ok := categorieByCode[code]
	return c, ok
}

// règle de classement : le premier motif trouvé dans le libellé normalisé gagne.
// L'ordre compte (« chirurgien dentiste » avant « chirurgie »).
type regle struct {
	motifs []string
	code   string
}

var regles = []regle{
	{[]string{"dentiste"}, "DENT"},
	{[]string{"medecin", "medcin"}, ""}, // traité à part : la spécialité décide
	{[]string{"pharmacien"}, "PHARM"},
	{[]string{"biologiste", "biochimiste", "microbiologiste", "biologie moleculaire"}, "BIO"},
	{[]string{"laboratoire", "laborantin"}, "TECH_LAB"},
	{[]string{"infirmier", "infirmiere"}, "INF"},
	{[]string{"sage femme", "sagefemme"}, "SAGEF"},
	{[]string{"ats", "agent technique de sante"}, "ATS"},
	{[]string{"aide sante", "aide soignant", "matronne"}, "AIDE_SOIN"},
	{[]string{"biomedica"}, "BIOMED"},
	{[]string{"statisticien", "biostatistique", "mathematicien"}, "STAT"},
	{[]string{"informatic", "telecom"}, "INFO"},
	{[]string{"chauffeur"}, "CHAUFFEUR"},
	{[]string{"vigil", "gendarme", "securite", "planton"}, "SECU"},
	{[]string{"administrateur", "redacteur", "services financiers", "ifsc", "isfc", "economiste",
		"juriste", "gestionnaire", "comptable", "sociologue", "journaliste", "logisticien"}, "ADMIN"},
	{[]string{"kinesither", "physiotherap", "nutrition", "psycholog", "radiologie", "ophtam", "ophtalmo",
		"hygiene", "sante publique", "promoteur", "epidemiolog", "preparateur en pharmacie", "chimiste"}, "AUTRE_SANTE"},
}

// spécialités médicales reconnues, dans l'ordre de test.
var specialites = []regle{
	{[]string{"chirurgie", "chirurgien", "traumatolog", "urolog"}, "MED_CHIR"},
	{[]string{"pediatrie", "pediatre"}, "MED_PED"},
	{[]string{"gynecolog"}, "MED_GYNE"},
	{[]string{"anesthes"}, "MED_ANESTH"},
	{[]string{"sante publique", "sante communautaire", "economie de la sante", "gestion hospitaliere", "management de sante"}, "MED_SP_PUB"},
	{[]string{"generaliste", "de famille"}, "MED_GEN"},
}

// CategorieDe classe un intitulé DRH. Un libellé inconnu tombe dans AUTRE
// plutôt que d'être perdu : ajouter un motif ci-dessus suffit à l'y sortir.
func CategorieDe(profession string) string {
	k := Norm(profession)
	if k == "" {
		return "AUTRE"
	}
	for _, r := range regles {
		if !matchAny(k, r.motifs) {
			continue
		}
		if r.code != "" {
			return r.code
		}
		for _, s := range specialites { // médecin : la spécialité décide
			if matchAny(k, s.motifs) {
				return s.code
			}
		}
		return "MED_AUTRE"
	}
	return "AUTRE"
}

func matchAny(k string, motifs []string) bool {
	for _, m := range motifs {
		if m == "ats" { // sigle : ne doit pas matcher « statisticien »
			if k == "ats" || strings.HasPrefix(k, "ats ") || strings.HasSuffix(k, " ats") {
				return true
			}
			continue
		}
		if strings.Contains(k, m) {
			return true
		}
	}
	return false
}
