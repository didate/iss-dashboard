# ISS Dashboard — Qualité & Utilisation des données DHIS2

Tableau de bord pour le programme DHIS2 « ISS Informations des Structures Sanitaires ». Deux axes : **qualité des données** (détection automatique de problèmes) et **analyse descriptive** (recensement, services, équipements, RH, commodités).

## Architecture

```
┌──────────────┐       ┌──────────────┐       ┌────────┐
│   Frontend   │──────▶│   Backend    │──────▶│  DHIS2 │
│  React/Vite  │  API  │   Go / Gin   │  PAT  │        │
│   (nginx)    │  REST │   (SQLite)   │       │        │
└──────────────┘       └──────────────┘       └────────┘
```

- **Backend** : Go + Gin, SQLite (modernc.org/sqlite, pur Go sans CGO)
- **Frontend** : React + TypeScript + Tailwind + Recharts — affichage uniquement, aucun calcul métier
- **Pré-calcul** : toute la logique s'exécute côté backend lors de la synchro, les résultats sont persistés dans SQLite, les endpoints servent des données pré-calculées

## Lancement rapide

### Prérequis

- Go 1.22+
- Node.js 20+
- Docker + Docker Compose (pour le déploiement)

### En local (sans Docker)

```bash
# 1. Backend
cp .env.example .env
# Éditez .env avec vos vraies valeurs (DHIS2_BASE_URL, DHIS2_PAT, ADMIN_TOKEN)

cd backend
go mod tidy
go run .
# Le backend écoute sur http://localhost:8080

# 2. Frontend (dans un autre terminal)
cd frontend
npm install
VITE_API_BASE_URL=http://localhost:8080 npm run dev
# Le frontend écoute sur http://localhost:3000
```

### Avec Docker Compose

```bash
cp .env.example .env
# Éditez .env

docker compose build
docker compose up -d
```

Le `docker-compose.yml` inclut les labels Traefik. Adaptez les noms de domaine (`api.iss.example.com`, `iss.example.com`) dans le fichier.

### Première synchronisation

1. Ouvrez l'interface → page **Admin**
2. Saisissez le token admin (valeur de `ADMIN_TOKEN` dans `.env`)
3. Cliquez **Synchroniser maintenant**
4. La synchro tourne en arrière-plan ; le statut se met à jour automatiquement

La synchro est aussi lancée automatiquement par le scheduler (par défaut toutes les 6h, configurable via `SYNC_CRON`).

## Configuration

| Variable | Description | Défaut |
|---|---|---|
| `DHIS2_BASE_URL` | URL de l'instance DHIS2 | — |
| `DHIS2_PAT` | Personal Access Token DHIS2 | — |
| `DHIS2_PROGRAM_ID` | ID du programme ISS | `AJy1cnAA50U` |
| `SQLITE_PATH` | Chemin du fichier SQLite | `./iss.db` |
| `SYNC_CRON` | Expression cron pour la synchro auto | `0 */6 * * *` |
| `ADMIN_TOKEN` | Token pour les endpoints admin | — |
| `DASHBOARD_PUBLIC` | `true` = lecture sans auth, `false` = token requis | `true` |
| `PORT` | Port du backend | `8080` |
| `VITE_API_BASE_URL` | URL du backend (build-time frontend) | `http://localhost:8080` |

## API

### Admin (header `X-Admin-Token` requis)

| Méthode | Route | Description |
|---|---|---|
| `POST` | `/api/admin/sync` | Lance une synchronisation |
| `GET` | `/api/admin/sync/status` | État de la synchro courante/dernière + historique |

### Lecture

| Méthode | Route | Description |
|---|---|---|
| `GET` | `/api/summary` | KPIs globaux |
| `GET` | `/api/quality/summary?by=district\|region\|statut` | Scores qualité agrégés |
| `GET` | `/api/quality/issues?severity=&rule=&district=&search=&page=&pageSize=` | Liste paginée des structures à problème |
| `GET` | `/api/quality/event/:uid` | Détail d'un event (issues + valeurs) |
| `GET` | `/api/usage/recensement?by=district\|region\|statut` | Recensement |
| `GET` | `/api/usage/services?district=` | Disponibilité des services |
| `GET` | `/api/usage/equipements?focus=chaine_froid\|imagerie\|all&district=` | Fonctionnalité équipements |
| `GET` | `/api/usage/rh?district=` | Ressources humaines |
| `GET` | `/api/usage/commodites?district=` | Commodités (WASH/énergie) |
| `GET` | `/api/meta/filters` | Listes pour les filtres front |

## Règles qualité

Le moteur de règles évalue chaque structure (event) et produit des issues avec trois niveaux de sévérité : `error` (-15 points), `warning` (-5 points), `info` (-1 point). Score = max(0, 100 - pénalités).

| Code | Nom | Sévérité | Description |
|---|---|---|---|
| R1 | Champs obligatoires | error/warning | Date absente (error), statut opérationnel ou nom responsable manquant (warning) |
| R2 | Cohérence total/fonctionnel | error/warning | Pour les 36 couples d'équipements : fonctionnel > total (error), fonctionnel sans total (warning) |
| R3 | Service sans support | warning | Service déclaré fonctionnel mais aucun équipement/infrastructure de support (labo sans microscope, maternité sans table d'accouchement, chirurgie sans table opératoire) |
| R4 | Cohérence commodités | warning/info | Énergie déclarée sans source cochée (warning), eau aux points critiques sans source d'eau (info) |
| R5 | Valeurs aberrantes | info | Valeur > médiane + 5×MAD et > 50 en absolu |
| R6 | Doublons | warning | Plusieurs events actifs sur la même org unit |
| R7 | Complétude | info | Structure « coquille vide » (aucun équipement ni RH renseigné) |

## Comment ajouter une nouvelle règle qualité

1. **Créer le fichier** `backend/internal/quality/r8_nom_regle.go` :

```go
package quality

import "iss-dashboard-backend/internal/models"

func CheckNomRegle(event *models.Event, ctx *QualityContext) []models.Issue {
    var issues []models.Issue

    // Votre logique ici
    // Utilisez GetEventValue(event, uid) ou GetEventValueByCode(event, code, ctx)
    // pour lire les valeurs de l'event

    val := GetEventValue(event, "UID_DU_DATA_ELEMENT")
    if val == "" {
        issues = append(issues, models.Issue{
            RuleCode: "R8",
            Severity: "warning",
            RuleName: "Nom de la règle",
            Message:  "Description du problème détecté",
        })
    }

    return issues
}
```

2. **Enregistrer la règle** dans `backend/internal/quality/engine.go`, ajoutez une ligne dans le slice `Registry` :

```go
{Code: "R8", Name: "Nom de la règle", Fn: CheckNomRegle},
```

3. **Ajouter un test** dans `backend/internal/quality/quality_test.go` :

```go
func TestR8_CasNominal(t *testing.T) {
    evt := makeEvent("e1", map[string]string{
        "UID": "valeur_problematique",
    })
    ctx := buildTestContext([]*models.Event{evt})
    issues := CheckNomRegle(evt, ctx)
    if len(issues) != 1 {
        t.Fatalf("expected 1 issue, got %d", len(issues))
    }
}
```

4. **Tester** :

```bash
cd backend && go test ./internal/quality/ -v
```

C'est tout. La règle sera automatiquement exécutée lors de la prochaine synchro, les issues seront persistées et visibles dans le dashboard.

### Helpers disponibles dans le contexte qualité

- `GetEventValue(event, uid)` — valeur par UID du data element
- `GetEventValueByCode(event, code, ctx)` — valeur par code du DE
- `ParseNum(s)` — parse une string en float64 (0 si invalide)
- `IsTruthy(v)` — vrai si `"true"`, `"1"` ou `"oui"`
- `ctx.EquipPairs` — couples TOTAL/FONC découverts dynamiquement
- `ctx.Medians[uid]` — médiane et MAD pour un DE numérique
- `ctx.OrgUnitCounts[orgUnitUID]` — nombre d'events par org unit
- `ctx.CodeToUID[code]` / `ctx.UIDToCode[uid]` — traduction code ↔ UID

## Structure du projet

```
backend/
  main.go
  internal/
    config/       Configuration (.env)
    models/       Structs Go
    dhis2/        Client HTTP DHIS2 (pagination, auth PAT)
    store/        SQLite (migrations, persistance transactionnelle, queries)
    quality/      Moteur de règles (R1-R7, score, tests)
    usage/        Agrégateurs (recensement, services, équipements, RH, commodités)
    sync/         Orchestrateur RunSync()
    api/          Handlers Gin + middleware
    scheduler/    Cron (robfig/cron)
  Dockerfile

frontend/
  src/
    api/          Client API typé
    pages/        Dashboard, Quality, Usage, Admin
    components/   Layout, KpiCard, DataTable, ScoreBar, SeverityBadge, charts/
    types/        Types TypeScript miroir de l'API
  Dockerfile
  nginx.conf

docker-compose.yml
.env.example
```

## Pipeline de synchronisation

```
RunSync()
  ├─ Pull métadonnées (data elements, option sets, org units)
  ├─ Pull events (paginé, 200/page)
  ├─ Enrichissement hiérarchie (district/région via org units)
  ├─ Construction du contexte qualité (couples équipement, médianes, compteurs)
  ├─ Exécution des 7 règles sur chaque event
  ├─ Calcul des scores qualité
  ├─ Calcul des agrégats d'utilisation (5 axes)
  └─ Persistance atomique (DELETE + INSERT dans une seule transaction SQLite)
```

La synchro est **idempotente** et **transactionnelle** : en cas d'erreur, le rollback préserve le dernier snapshot valide.

## Tests

```bash
cd backend && go test ./... -v
```

22 tests unitaires couvrent toutes les règles qualité (R1-R7), le calcul de score, et l'intégration du moteur de règles.
