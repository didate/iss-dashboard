// Package typologie résout le type d'une structure sanitaire (poste de santé,
// centre de santé, hôpital…) et son statut juridique à partir des groupes
// d'unités d'organisation DHIS2, avec repli sur le préfixe du nom.
//
// Les noms de group sets sont configurables (ils varient par instance) ; les
// correspondances nom de groupe → code et préfixe de nom → code ci-dessous sont
// du métier stable, et testées.
package typologie

import "strings"

// Codes de type. Le palier « normes » s'indexera dessus : ne pas renommer sans migration.
const (
	PS          = "PS"
	CS          = "CS"
	CSA         = "CSA"
	CMC         = "CMC"
	HP          = "HP"
	HR          = "HR"
	HN          = "HN"
	Cabinet     = "CABINET"
	Clinique    = "CLINIQUE"
	AutrePrive  = "AUTRE_PRIVE"
	ASC         = "ASC"
	Indetermine = "INDETERMINE"
)

// Sources de résolution, stockées dans event.type_source.
const (
	SourceGroup         = "group"          // un seul groupe du set de typologie
	SourceGroupMultiple = "group_multiple" // plusieurs groupes : le premier par ordre de priorité est retenu
	SourceName          = "name"           // déduit du préfixe du nom
	SourceNone          = "none"           // indéterminé
)

// Labels lisibles par code.
var Labels = map[string]string{
	PS:          "Poste de santé",
	CS:          "Centre de santé",
	CSA:         "Centre de santé amélioré",
	CMC:         "Centre médico-communal",
	HP:          "Hôpital préfectoral",
	HR:          "Hôpital régional",
	HN:          "Hôpital national",
	Cabinet:     "Cabinet médical",
	Clinique:    "Clinique / centre médical",
	AutrePrive:  "Autre structure privée",
	ASC:         "Site communautaire",
	Indetermine: "Type indéterminé",
}

// Label returns the human label for a code, or the code itself if unknown.
func Label(code string) string {
	if l, ok := Labels[code]; ok {
		return l
	}
	return code
}

// groupNameToCode maps a normalised group name (numeric prefix stripped, upper
// case) of the typology set to a type code. "HOPITAUX" is refined by the hospital set.
var groupNameToCode = map[string]string{
	"PS":       PS,
	"CS":       CS,
	"CSA":      CSA,
	"CMC":      CMC,
	"HOPITAUX": HP,
	"ASC":      ASC,
}

// groupPriority breaks ties when an org unit sits in several typology groups: the
// most specific level wins (an org unit in CS and PS is far more likely a CS).
var groupPriority = []string{HN, HR, HP, CMC, CSA, CS, PS, ASC}

// prefixToCode maps the first token of an org unit name to a type code.
var prefixToCode = map[string]string{
	"PS":           PS,
	"POSTE":        PS,
	"CS":           CS,
	"CSR":          CS,
	"CSU":          CS,
	"CSC":          CS,
	"CENTRE":       CS,
	"CSA":          CSA,
	"CMC":          CMC,
	"HP":           HP,
	"HOPITAL":      HP,
	"HÔPITAL":      HP,
	"HR":           HR,
	"HN":           HN,
	"CHU":          HN,
	"CABINET":      Cabinet,
	"CLINIQUE":     Clinique,
	"POLYCLINIQUE": Clinique,
	"CM":           Clinique,
}

// normalizeGroupName strips a leading numeric ordinal ("06 PS" → "PS") and upper-cases.
func normalizeGroupName(name string) string {
	name = strings.TrimSpace(name)
	fields := strings.Fields(name)
	if len(fields) > 1 && isDigits(fields[0]) {
		fields = fields[1:]
	}
	return strings.ToUpper(strings.Join(fields, " "))
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// hospitalSubtype refines HOPITAUX from the hospital group set's group name.
func hospitalSubtype(groupName string) string {
	n := strings.ToLower(groupName)
	switch {
	case strings.Contains(n, "nation"):
		return HN
	case strings.Contains(n, "region") || strings.Contains(n, "région"):
		return HR
	case strings.Contains(n, "prefect") || strings.Contains(n, "préfect"):
		return HP
	}
	return ""
}

// ownershipFromGroup maps a group name of the ownership set to the option codes
// used by the ISS form (ISS_STATUT_STRUCT_DE : "publique" / "privée").
func ownershipFromGroup(groupName string) string {
	n := strings.ToLower(groupName)
	switch {
	case strings.Contains(n, "priv"):
		return "privée"
	case strings.Contains(n, "public"):
		return "publique"
	}
	return ""
}

// codeFromName derives a type from the first token of the org unit name.
func codeFromName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	// Split on whitespace, "_", "-" and "/" so "CSR-Yombiro" or "PS_Patagara" work.
	first := strings.FieldsFunc(name, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '_' || r == '-' || r == '/' || r == '.'
	})
	if len(first) == 0 {
		return ""
	}
	return prefixToCode[strings.ToUpper(first[0])]
}
