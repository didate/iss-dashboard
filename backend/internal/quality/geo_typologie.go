package quality

import (
	"fmt"
	"strings"

	"iss-dashboard-backend/internal/models"
	"iss-dashboard-backend/internal/typologie"
)

// R14 — Coordonnées GPS manquantes (warning).
// La position vient de org_unit.geometry (type Point) et est copiée sur l'event
// à l'ingestion ; sans elle la structure n'apparaît pas sur la carte de points.
func CheckMissingGPS(evt *models.Event, ctx *QualityContext) []models.Issue {
	if evt.HasGPS() {
		return nil
	}
	return []models.Issue{{
		Severity: "warning",
		Message:  "Aucune coordonnée GPS sur l'unité d'organisation DHIS2 : la structure n'est pas positionnable sur la carte",
	}}
}

// R17 — Type de structure indéterminé (info).
// Le type est résolu à l'ingestion (event.type_code / type_source). Tout ce qui
// n'est pas « un seul groupe DHIS2 du set de typologie » est signalé : le
// nettoyage se fait dans DHIS2, l'app fournit la liste.
func CheckTypologie(evt *models.Event, ctx *QualityContext) []models.Issue {
	switch evt.TypeSource {
	case typologie.SourceGroup:
		return nil
	case typologie.SourceGroupMultiple:
		return []models.Issue{{
			Severity: "info",
			Message:  fmt.Sprintf("Structure présente dans plusieurs groupes de typologie DHIS2 (type retenu : %s)", typologie.Label(evt.TypeCode)),
		}}
	case typologie.SourceName:
		return []models.Issue{{
			Severity: "info",
			Message:  fmt.Sprintf("Aucun groupe de typologie DHIS2 : type déduit du nom (%s)", typologie.Label(evt.TypeCode)),
		}}
	default: // SourceNone or empty (old snapshot)
		return []models.Issue{{
			Severity: "info",
			Message:  "Type de structure indéterminé : aucun groupe de typologie DHIS2 et préfixe de nom non reconnu",
		}}
	}
}

// R18 — Statut juridique incohérent (info).
// Compare le set de groupes Public/Privé de DHIS2 au champ ISS_STATUT_STRUCT_DE
// du formulaire. Silencieux si les groupes n'ont pas été chargés.
func CheckOwnership(evt *models.Event, ctx *QualityContext) []models.Issue {
	if ctx.Typologie == nil {
		return nil
	}
	fromGroup := ctx.Typologie.Ownership(evt.OrgUnitUID)
	fromForm := strings.TrimSpace(GetEventValueByCode(evt, "ISS_STATUT_STRUCT_DE", ctx))

	switch {
	case fromGroup == "":
		return []models.Issue{{
			Severity: "info",
			Message:  "Structure absente des groupes DHIS2 Public / Privé",
		}}
	case fromForm != "" && !strings.EqualFold(fromForm, fromGroup):
		return []models.Issue{{
			Severity: "info",
			Message:  fmt.Sprintf("Statut juridique : groupe DHIS2 « %s » mais formulaire « %s »", fromGroup, fromForm),
		}}
	}
	return nil
}
