# ISS Dashboard — Qualite & Utilisation des donnees DHIS2

Tableau de bord pour le programme DHIS2 « ISS Informations des Structures Sanitaires ». Trois axes : **qualite des donnees** (detection automatique de problemes), **analyse descriptive** (recensement, services, equipements, RH, commodites) et **taux de rapportage**.

## Architecture

```
┌──────────────┐       ┌──────────────┐       ┌────────┐
│   Frontend   │──────▶│   Backend    │──────▶│  DHIS2 │
│  React/Vite  │  API  │   Go / Gin   │  PAT  │        │
│   (nginx)    │  REST │   (SQLite)   │       │        │
└──────────────┘       └──────────────┘       └────────┘
```

- **Backend** : Go + Gin, SQLite (modernc.org/sqlite, pur Go sans CGO)
- **Frontend** : React + TypeScript + Tailwind + Recharts — affichage uniquement, aucun calcul metier
- **Pre-calcul** : toute la logique s'execute cote backend lors de la synchro, les resultats sont persistes dans SQLite, les endpoints servent des donnees pre-calculees
- **Auth** : JWT (login/password), roles admin et viewer

## Lancement rapide

### Prerequis

- Go 1.25+
- Node.js 20+
- Docker + Docker Compose (pour le deploiement)

### En local (sans Docker)

```bash
# 1. Backend
cp .env.example .env
# Editez .env avec vos vraies valeurs (DHIS2_BASE_URL, DHIS2_PAT, ADMIN_TOKEN)

cd backend
go mod tidy
go run .
# Le backend ecoute sur http://localhost:8080

# 2. Frontend (dans un autre terminal)
cd frontend
npm install
VITE_API_BASE_URL=http://localhost:8080 npm run dev
# Le frontend ecoute sur http://localhost:3000
```

### Avec Docker Compose (production)

Sur le serveur, utilisez `docker-compose.prod.yml` (images GHCR, pas de build local) :

```bash
# Creer le .env sur le serveur
cat > .env << 'EOF'
DHIS2_BASE_URL=https://votre-instance.dhis2.org
DHIS2_PAT=votre_personal_access_token
DHIS2_PROGRAM_ID=AJy1cnAA50U
SQLITE_PATH=/data/iss.db
SYNC_CRON=0 */6 * * *
ADMIN_TOKEN=votre_mot_de_passe_admin
DASHBOARD_PUBLIC=true
PORT=8080
EOF

# Lancer
docker compose pull && docker compose up -d
```

### Premiere connexion

1. Ouvrez l'interface → **Se connecter** (lien en bas de la sidebar)
2. Identifiants par defaut : `admin` / `<valeur de ADMIN_TOKEN>`
3. Allez dans **Admin** → **Synchroniser** pour lancer le premier pull DHIS2
4. La synchro tourne en arriere-plan (~30-60s), le statut se met a jour automatiquement

La synchro est aussi lancee automatiquement par le scheduler (par defaut toutes les 6h, configurable via `SYNC_CRON`).

## Pages du dashboard

| Page | Description |
|---|---|
| **Vue d'ensemble** | KPIs (structures, score qualite, erreurs, taux de rapportage, derniere synchro), graphes score par district et issues par region |
| **Qualite** | Tableau filtrable/pagine des structures a probleme (severite, regle, district, recherche). Clic → panneau de detail. Export CSV. |
| **Utilisation** | 8 onglets : Rapportage, Recensement, Plateau technique, Services, Matrice services×district, Equipements, RH, Commodites |
| **Admin** | Synchronisation manuelle, export Excel, gestion des utilisateurs, historique des synchros (protege par login) |

## Configuration

| Variable | Description | Defaut |
|---|---|---|
| `DHIS2_BASE_URL` | URL de l'instance DHIS2 | — |
| `DHIS2_PAT` | Personal Access Token DHIS2 | — |
| `DHIS2_PROGRAM_ID` | ID du programme ISS | `AJy1cnAA50U` |
| `SQLITE_PATH` | Chemin du fichier SQLite | `./iss.db` |
| `SYNC_CRON` | Expression cron pour la synchro auto | `0 */6 * * *` |
| `ADMIN_TOKEN` | Mot de passe du compte admin par defaut | — |
| `DASHBOARD_PUBLIC` | `true` = lecture sans auth, `false` = JWT requis | `true` |
| `PORT` | Port du backend | `8080` |
| `VITE_API_BASE_URL` | URL du backend (build-time frontend) | `http://localhost:8080` |

## API

### Authentification

| Methode | Route | Description |
|---|---|---|
| `POST` | `/iss/api/auth/login` | Login → retourne un JWT (24h) |
| `GET` | `/iss/api/auth/me` | Utilisateur courant (JWT requis) |

### Admin (JWT + role admin)

| Methode | Route | Description |
|---|---|---|
| `POST` | `/iss/api/admin/sync` | Lance une synchronisation |
| `GET` | `/iss/api/admin/sync/status` | Etat de la synchro + historique |
| `GET` | `/iss/api/admin/users` | Liste des utilisateurs |
| `POST` | `/iss/api/admin/users` | Creer un utilisateur |
| `DELETE` | `/iss/api/admin/users/:id` | Supprimer un utilisateur |

### Export (JWT requis)

| Methode | Route | Description |
|---|---|---|
| `GET` | `/iss/api/export/excel` | Telecharge un fichier Excel avec toutes les donnees brutes |

### Lecture (publique si `DASHBOARD_PUBLIC=true`)

| Methode | Route | Description |
|---|---|---|
| `GET` | `/iss/api/summary` | KPIs globaux |
| `GET` | `/iss/api/quality/summary?by=district\|region\|statut` | Scores qualite agreges |
| `GET` | `/iss/api/quality/issues?severity=&rule=&district=&search=&page=&pageSize=` | Liste paginee des structures a probleme |
| `GET` | `/iss/api/quality/event/:uid` | Detail d'un event (issues + valeurs) |
| `GET` | `/iss/api/usage/reporting?by=district\|region\|global` | Taux de rapportage |
| `GET` | `/iss/api/usage/recensement?by=district\|region\|statut` | Recensement |
| `GET` | `/iss/api/usage/services?district=` | Disponibilite des services |
| `GET` | `/iss/api/usage/services/matrix` | Matrice services × district |
| `GET` | `/iss/api/usage/equipements?focus=chaine_froid\|imagerie\|all&district=` | Fonctionnalite equipements |
| `GET` | `/iss/api/usage/rh?district=` | Ressources humaines |
| `GET` | `/iss/api/usage/rh/summary?district=` | Resume RH (effectifs, ratio medecins) |
| `GET` | `/iss/api/usage/plateau?district=` | Plateau technique |
| `GET` | `/iss/api/usage/commodites?district=` | Commodites (WASH/energie) |
| `GET` | `/iss/api/meta/filters` | Listes pour les filtres front |

## Regles qualite

Le moteur de regles evalue chaque structure (event) et produit des issues avec trois niveaux de severite : `error` (-15 points), `warning` (-5 points), `info` (-1 point). Score = max(0, 100 - penalites).

| Code | Nom | Severite | Description |
|---|---|---|---|
| R1 | Champs obligatoires | error/warning | Date absente (error), statut operationnel ou nom responsable manquant (warning) |
| R2 | Coherence total/fonctionnel | error/warning | Pour les 36 couples d'equipements : fonctionnel > total (error), fonctionnel sans total (warning) |
| R3 | Service sans support | warning | Service declare fonctionnel mais aucun equipement/infrastructure de support (labo sans microscope, maternite sans table d'accouchement, chirurgie sans table operatoire) |
| R4 | Coherence commodites | warning/info | Energie declaree sans source cochee (warning), eau aux points critiques sans source d'eau (info) |
| R5 | Valeurs aberrantes | info | Valeur > mediane + 5×MAD et > 50 en absolu |
| R6 | Doublons | warning | Plusieurs events actifs sur la meme org unit |
| R7 | Completude | info | Structure « coquille vide » (aucun equipement ni RH renseigne) |

## Comment ajouter une nouvelle regle qualite

1. **Creer le fichier** `backend/internal/quality/r8_nom_regle.go` :

```go
package quality

import "iss-dashboard-backend/internal/models"

func CheckNomRegle(event *models.Event, ctx *QualityContext) []models.Issue {
    var issues []models.Issue

    val := GetEventValue(event, "UID_DU_DATA_ELEMENT")
    if val == "" {
        issues = append(issues, models.Issue{
            RuleCode: "R8",
            Severity: "warning",
            RuleName: "Nom de la regle",
            Message:  "Description du probleme detecte",
        })
    }

    return issues
}
```

2. **Enregistrer la regle** dans `backend/internal/quality/engine.go` :

```go
{Code: "R8", Name: "Nom de la regle", Fn: CheckNomRegle},
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

4. **Tester** : `cd backend && go test ./internal/quality/ -v`

### Helpers disponibles

- `GetEventValue(event, uid)` — valeur par UID du data element
- `GetEventValueByCode(event, code, ctx)` — valeur par code du DE
- `ParseNum(s)` — parse une string en float64 (0 si invalide)
- `IsTruthy(v)` — vrai si `"true"`, `"1"` ou `"oui"`
- `ctx.EquipPairs` — couples TOTAL/FONC decouverts dynamiquement
- `ctx.Medians[uid]` — mediane et MAD pour un DE numerique
- `ctx.OrgUnitCounts[orgUnitUID]` — nombre d'events par org unit
- `ctx.CodeToUID[code]` / `ctx.UIDToCode[uid]` — traduction code <-> UID

## Export Excel

### Depuis l'interface

Connectez-vous en admin → page **Admin** → bouton **Export Excel**. Le fichier contient toutes les structures avec leurs 225 data elements et scores qualite.

### Depuis la ligne de commande

```bash
# Prerequis
pip3 install openpyxl

# Lancer une synchro puis exporter
curl -X POST http://localhost:8080/iss/api/admin/sync -H "Authorization: Bearer <token>"
python3 scripts/export_excel.py
```

Voir `scripts/README.md` pour plus de details.

## Structure du projet

```
backend/
  main.go
  internal/
    config/       Configuration (.env)
    models/       Structs Go (Event, User, Issue, etc.)
    dhis2/        Client HTTP DHIS2 (pagination, auth PAT)
    store/        SQLite (migrations, persistance transactionnelle, queries, users)
    quality/      Moteur de regles (R1-R7, score, tests)
    usage/        Agregateurs (recensement, services, equipements, RH, commodites, rapportage)
    sync/         Orchestrateur RunSync()
    api/          Handlers Gin + middleware JWT + export Excel
    scheduler/    Cron (robfig/cron)
  Dockerfile

frontend/
  src/
    api/          Client API type + auth JWT
    pages/        Dashboard, Quality, Usage, Admin, Login
    components/   Layout, KpiCard, DataTable, ScoreBar, SeverityBadge, ExportCSV, MethodNote, charts/
    types/        Types TypeScript miroir de l'API
    utils/        Helpers (formatage nombres)
  Dockerfile
  nginx.conf

scripts/
  export_excel.py   Export Excel en ligne de commande
  README.md

docker-compose.yml        Dev (build local)
docker-compose.prod.yml   Production (images GHCR)
.github/workflows/
  ci.yml                  CI : build + test
  cd.yml                  CD : push images GHCR + deploy SSH
```

## Pipeline de synchronisation

```
RunSync()
  |-- Pull metadonnees (data elements, option sets, org units)
  |-- Pull org units assignees au programme (pour le taux de rapportage)
  |-- Pull events (pagine, 200/page)
  |-- Enrichissement hierarchie (district/region via org units)
  |-- Construction du contexte qualite (couples equipement, medianes, compteurs)
  |-- Execution des 7 regles sur chaque event
  |-- Calcul des scores qualite
  |-- Calcul des agregats d'utilisation (8 axes)
  |-- Calcul du taux de rapportage (exclut structures fermees)
  +-- Persistance atomique (DELETE + INSERT dans une seule transaction SQLite)
```

La synchro est **idempotente** et **transactionnelle** : en cas d'erreur, le rollback preserve le dernier snapshot valide. Les sync_run orphelines (crash) sont auto-nettoyees au demarrage.

## CI/CD

- **CI** (`ci.yml`) : a chaque push/PR sur main → build Go + tests + build React + type check + build Docker
- **CD** (`cd.yml`) : a chaque push sur main → build images → push GHCR → deploy via SSH

### Secrets GitHub requis

| Secret | Description |
|---|---|
| `DEPLOY_HOST` | IP ou hostname du serveur |
| `DEPLOY_USER` | Utilisateur SSH |
| `DEPLOY_SSH_KEY` | Cle privee SSH |
| `DEPLOY_PATH` | Chemin du docker-compose sur le serveur |

### Variable GitHub

| Variable | Valeur |
|---|---|
| `VITE_API_BASE_URL` | URL du backend (ex: `https://apps.example.com/iss`) |

## Tests

```bash
cd backend && go test ./... -v
```

22 tests unitaires couvrent toutes les regles qualite (R1-R7), le calcul de score, et l'integration du moteur de regles.
