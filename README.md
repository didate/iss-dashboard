# ISS Dashboard — Carte sanitaire, qualite & utilisation des donnees DHIS2

Application construite sur le programme DHIS2 « ISS Informations des Structures Sanitaires ». Deux espaces :

- **Carte sanitaire publique** (`/`) : carte des structures de sante, recherche, « autour de moi », fiche par structure (identite, type, statut, localisation, services) — sans connexion, projections reduites uniquement.
- **Espace planification** (`/tableau-de-bord`…) : **qualite des donnees** (regles automatiques), **analyse descriptive** (recensement, services, equipements, RH, commodites, couverture demographique), **taux de rapportage** et **geolocalisation** (couverture GPS, structures a positionner).

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
- **Pre-calcul** : toute la logique s'execute cote backend lors de la synchro, les resultats sont persistes dans SQLite, les endpoints servent des donnees pre-calculees (y compris le GeoJSON public, servi avec ETag + gzip)
- **Auth** : JWT (login/password), roles admin et viewer. L'espace public (`/iss/api/public/*`) est toujours ouvert et ne sert que des projections reduites ; l'espace planification passe derriere le JWT quand `DASHBOARD_PUBLIC=false`

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
# Le backend ecoute sur http://localhost:8081 (PORT dans .env ; 8080 est souvent pris par une autre appli)

# 2. Frontend (dans un autre terminal)
cd frontend
npm install
VITE_API_BASE_URL=http://localhost:8081/iss npm run dev
# Le frontend ecoute sur http://localhost:3000/iss/
```

`VITE_API_BASE_URL` doit inclure le prefixe `/iss` : le client appelle `${VITE_API_BASE_URL}/api/...`.

Le backend ne lit pas `.env` tout seul : exportez-le avant `go run` (`set -a; source .env; set +a`). Mettez les valeurs contenant `*` entre guillemets (`SYNC_CRON="0 */6 * * *"`), sinon zsh interrompt la lecture du fichier.

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
# false en production : la carte publique reste ouverte, le reste exige un login
DASHBOARD_PUBLIC=false
PORT=8080
# Carte sanitaire (voir section Configuration)
DHIS2_POPULATION_DX=total:ksBi2JIApqW+oVYNP4fGnTo,moins5:hLcbHlNiRqP,fap:vRIHUMcfSnT,grossesses:IYeo7xNzEWy,accouchements:UQxlKligKNQ
DHIS2_POPULATION_LEVELS=1,2,3,4
DHIS2_POPULATION_FACTOR=12
DHIS2_TYPOLOGY_GROUPSET=01 TOUTES LES STRUCTURES
DHIS2_HOSPITAL_GROUPSET=07 HÖPITAUX
DHIS2_OWNERSHIP_GROUPSET=02 PUBLIC PRIVEE
EOF

# Lancer
docker compose pull && docker compose up -d
```

### Premiere connexion

1. Ouvrez l'interface : la carte publique s'affiche. Cliquez **Espace planification** → page de login
2. Identifiants par defaut : `admin` / `<valeur de ADMIN_TOKEN>`
3. Allez dans **Admin** → **Synchroniser** pour lancer le premier pull DHIS2
4. La synchro tourne en arriere-plan (~30-60s), le statut se met a jour automatiquement

La synchro est aussi lancee automatiquement par le scheduler (par defaut toutes les 6h, configurable via `SYNC_CRON`).

## Pages

### Espace public (sans connexion)

| Page | Description |
|---|---|
| **Carte** (`/`) | Fond OpenStreetMap, structures geolocalisees en clusters colores par type (regroupement desactivable). Recherche par nom, filtres type / service / region / district (dans l'URL, partageables), « Autour de moi » (geolocalisation du navigateur, tri par distance cote serveur). Clic marqueur → popup ; clic dans la liste → deplacement + popup. |
| **Fiche** (`/fs/:uid`) | Identite, type, statut juridique et operationnel, rattachement, mini-carte, plateau technique, services fonctionnels, QR code, copie du lien, itineraire OSM. `uid` = UID de l'unite d'organisation DHIS2 (stable entre recensements). |
| **Annuaire** (`/annuaire`) | Registre en tableau, memes filtres que la carte, pagine, **export CSV** (open data) avec les filtres actifs. |
| **A propos** (`/a-propos`) | Sources, chiffres cles, limites. |

La fiche publique est volontairement **reduite** : pas de RH, pas d'equipements chiffres, pas de nom/telephone du responsable, pas de qualite. Cette frontiere est garantie par l'API (`/api/public/*` ne lit que `structure_latest` et les blobs pre-calcules) et par un test (`usage/public_snapshot_test.go`) qui verifie que le GeoJSON public ne fuit rien.

### Espace planification

| Page | Description |
|---|---|
| **Vue d'ensemble** (`/tableau-de-bord`) | KPIs (structures, score qualite, erreurs, taux de rapportage, derniere synchro), graphes score par district et issues par region |
| **Qualite** | Tableau filtrable/pagine des structures a probleme (severite, regle, district, recherche). Clic → panneau de detail. Export CSV. |
| **Utilisation** | Onglets : Rapportage, Recensement (dont par type de structure), Plateau technique, Services, Matrice, Equipements, RH, Commodites, **Couverture** (ratios pour 10 000 habitants), Structures fermees |
| **Structures** | Liste filtrable (district, type, GPS, nom) → detail complet (type et sa source, sous-prefecture, coordonnees, lien vers la fiche publique, issues, valeurs) |
| **Comparaison** | Comparaison de districts |
| **Carte** | Choropletes par district (rapportage, qualite, services, equipements, WASH, RH) + **Couverture geo** (district ou sous-prefecture : % GPS, score, structures /10 000 hab., nombre) + **Structures (points)** colores par score qualite |
| **GPS** (`/geolocalisation`) | Couverture GPS nationale et par district / sous-prefecture, liste des structures sans coordonnees, export CSV pour les equipes terrain |
| **Admin** | Synchronisation manuelle, export Excel, gestion des utilisateurs, historique des synchros |

## Configuration

| Variable | Description | Defaut |
|---|---|---|
| `DHIS2_BASE_URL` | URL de l'instance DHIS2 | — |
| `DHIS2_PAT` | Personal Access Token DHIS2 | — |
| `DHIS2_PROGRAM_ID` | ID du programme ISS | `AJy1cnAA50U` |
| `SQLITE_PATH` | Chemin du fichier SQLite | `./iss.db` |
| `SYNC_CRON` | Expression cron pour la synchro auto | `0 */6 * * *` |
| `ADMIN_TOKEN` | Mot de passe du compte admin par defaut | — |
| `DASHBOARD_PUBLIC` | `true` = tout lisible sans auth (dev), `false` = espace planification derriere JWT ; `/api/public/*` reste toujours ouvert | `true` |
| `PORT` | Port du backend (en Docker, le conteneur reste sur 8080 : `docker-compose*.yml`) | `8081` |
| `VITE_API_BASE_URL` | URL du backend **avec le prefixe `/iss`** (build-time frontend) | `http://localhost:8081/iss` |

### Carte sanitaire (UIDs et noms propres a l'instance DHIS2)

| Variable | Description | Defaut |
|---|---|---|
| `DHIS2_POPULATION_DX` | Indicateurs de population : `indicateur:uid[+uid],...`. Les UIDs d'un meme indicateur sont sommes (ex. feminin + masculin). Vide = ratios demographiques desactives | — |
| `DHIS2_POPULATION_LEVELS` | Niveaux d'org unit pour lesquels lire la population (1 = national … 4 = sous-prefecture) | `1,2,3,4` |
| `DHIS2_POPULATION_FACTOR` | Facteur applique a la derniere valeur mensuelle. `12` quand la saisie mensuelle vaut population annuelle / 12 (convention « cible mensuelle ») ; `1` pour des valeurs brutes | `12` |
| `DHIS2_TYPOLOGY_GROUPSET` | Nom du group set DHIS2 qui classe les structures (PS, CS, CSA, CMC, hopitaux) | `01 TOUTES LES STRUCTURES` |
| `DHIS2_HOSPITAL_GROUPSET` | Group set qui sous-type les hopitaux (nationaux / regionaux / prefectoraux) | `07 HÖPITAUX` |
| `DHIS2_OWNERSHIP_GROUPSET` | Group set Public / Prive | `02 PUBLIC PRIVEE` |

Population : le data set est mensuel, donc pour chaque org unit on garde **la derniere periode ou tous les UIDs de l'indicateur sont renseignes** (jamais une somme annuelle, fausse sur une annee incomplete), multipliee par `DHIS2_POPULATION_FACTOR`.

Groupes d'OU : ils sont lus via les group sets (champs imbriques), ce qui contourne un partage manquant sur certains groupes — mais donnez quand meme aux groupes un partage « lecture publique des metadonnees » dans DHIS2.

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

### Espace public (toujours ouvert, projections reduites)

| Methode | Route | Description |
|---|---|---|
| `GET` | `/iss/api/public/points.geojson` | GeoJSON pre-calcule de toutes les structures geolocalisees (uid, nom, type, statut, rattachement, services `oui`). ETag, gzip, `Cache-Control: max-age=3600` |
| `GET` | `/iss/api/public/filters` | Types (avec effectifs), services, regions, districts |
| `GET` | `/iss/api/public/structures?search=&type=&service=&district=&region=&near=lat,lng&radius_km=&limit=` | Recherche ; avec `near`, tri par distance (haversine) et rayon |
| `GET` | `/iss/api/public/annuaire?…&page=&pageSize=` | Registre pagine (memes filtres + `sous_prefecture`) |
| `GET` | `/iss/api/public/structures.csv?…` | Registre complet en CSV (`;`, UTF-8 BOM) |
| `GET` | `/iss/api/public/structure/:uid` | Fiche reduite (uid = org unit) |
| `GET` | `/iss/api/public/summary` | Nombre de structures, repartition par type, % GPS, date de synchro |

### Espace planification (publique si `DASHBOARD_PUBLIC=true`, sinon JWT)

| Methode | Route | Description |
|---|---|---|
| `GET` | `/iss/api/geo/coverage?level=3\|4&region=` | Couverture par district / sous-prefecture (structures, GPS, score, population, types) |
| `GET` | `/iss/api/geo/missing?district=&region=&type=&page=&pageSize=` | Structures sans coordonnees (dernier event par org unit) |
| `GET` | `/iss/api/geo/missing.csv?district=&region=&type=` | Idem, liste complete en CSV (`;`, UTF-8 BOM) |
| `GET` | `/iss/api/usage/couverture?by=global\|region\|district\|sous_prefecture&indicator=` | Ratios pour 10 000 habitants (structures, lits, medecins, sages_femmes, infirmiers, ats, personnel_soignant), a tous les niveaux |
| `GET` | `/iss/api/map/geo?level=3\|4` | Polygones + proprietes de couverture, dont `ratios` et `numerators` par indicateur |
| `GET` | `/iss/api/export/pdf?district=` ou `?region=` | Rapport PDF d'un district ou d'une region (agregats des districts) |
| `GET` | `/iss/api/map/points` | Structures geolocalisees avec score qualite |
| `GET` | `/iss/api/structures?district=&sous_prefecture=&search=&type=&gps=oui\|non&page=&pageSize=` | Liste des structures |
| `GET` | `/iss/api/quality/issues?…&region=&sous_prefecture=` | Filtres geographiques supplementaires |

### Lecture (publique si `DASHBOARD_PUBLIC=true`)

| Methode | Route | Description |
|---|---|---|
| `GET` | `/iss/api/summary` | KPIs globaux |
| `GET` | `/iss/api/quality/summary?by=district\|region\|statut` | Scores qualite agreges |
| `GET` | `/iss/api/quality/issues?severity=&rule=&district=&search=&page=&pageSize=` | Liste paginee des structures a probleme |
| `GET` | `/iss/api/quality/event/:uid` | Detail d'un event (issues + valeurs) |
| `GET` | `/iss/api/usage/reporting?by=district\|region\|global` | Taux de rapportage |
| `GET` | `/iss/api/usage/recensement?by=district\|region\|type\|statut_structure\|statut_juridique` | Recensement |
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

Les codes ci-dessous sont ceux **reellement emis** par les fonctions (et persistes dans `quality_issue.rule_code`) ; le registre `engine.go` les reprend.

| Code | Nom | Severite | Description |
|---|---|---|---|
| R1 | Champs obligatoires | error/warning | Date absente (error), statut operationnel ou nom responsable manquant (warning) |
| R2 | Coherence total/fonctionnel | error/warning | Pour les 36 couples d'equipements : fonctionnel > total (error), fonctionnel sans total (warning) |
| R3 | Service sans support | warning | Service declare fonctionnel mais aucun equipement/infrastructure de support (labo sans microscope, maternite sans table d'accouchement, chirurgie sans table operatoire) |
| R4 | Coherence commodites | warning/info | Energie declaree sans source cochee (warning), eau aux points critiques sans source d'eau (info) |
| R5 | Valeurs aberrantes | info | Desactivee (seuils mediane + MAD non pertinents sur ce jeu) |
| R6 | Soumissions multiples | warning | Plusieurs events sur la meme org unit la meme annee |
| R7 | Completude | info | Structure « coquille vide » (aucun equipement ni RH renseigne) |
| R8 | Rapport apres fermeture | warning | Event date apres la date de fermeture de l'org unit |
| R9 | Valeur invalide | warning | Valeur hors de l'option set du data element |
| R10 / R11 | Maternite sans sage-femme / Labo sans technicien | warning | Service declare sans le personnel correspondant |
| R13 | Aucun service declare | warning | Structure operationnelle sans aucun service |
| R15 / R16 | Source d'eau / d'energie non renseignee | info | Structure operationnelle sans information WASH |
| **R14** | Coordonnees GPS manquantes | warning | L'org unit n'a pas de geometrie Point : la structure n'apparait pas sur la carte |
| **R17** | Type de structure indetermine | info | Aucun groupe de typologie DHIS2, plusieurs groupes, ou type deduit du nom |
| **R18** | Statut juridique incoherent | info | Absente des groupes Public/Prive, ou groupe ≠ formulaire ISS |

R12 (pharmacie sans pharmacien) existe mais n'est volontairement pas enregistree : elle signalerait ~2 300 postes de sante, qui n'ont pas de pharmacien par norme.

## Comment ajouter une nouvelle regle qualite

1. **Creer le fichier** `backend/internal/quality/r19_nom_regle.go` (prenez un code libre : verifiez `engine.go` **et** les `RuleCode` codes en dur dans les fonctions) :

```go
package quality

import "iss-dashboard-backend/internal/models"

func CheckNomRegle(event *models.Event, ctx *QualityContext) []models.Issue {
    var issues []models.Issue

    val := GetEventValue(event, "UID_DU_DATA_ELEMENT")
    if val == "" {
        issues = append(issues, models.Issue{
            RuleCode: "R19",
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
{Code: "R19", Name: "Nom de la regle", Fn: CheckNomRegle},
```

3. **Ajouter un test** dans `backend/internal/quality/quality_test.go` :

```go
func TestR19_CasNominal(t *testing.T) {
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
- `event.HasGPS()`, `event.TypeCode`, `event.TypeSource`, `event.SousPrefecture` — attributs carte sanitaire poses a l'ingestion
- `ctx.Typologie.Ownership(orgUnitUID)` — statut juridique d'apres les groupes DHIS2 (nil si groupes non charges : testez-le)

## Typologie des structures

Le type de chaque structure est resolu a l'ingestion (`internal/typologie`) et stocke dans `event.type_code` / `event.type_source` :

1. **Groupe d'OU** du set `DHIS2_TYPOLOGY_GROUPSET` → `PS`, `CS`, `CSA`, `CMC`, `HP` ; `HP` est affine par le set `DHIS2_HOSPITAL_GROUPSET` en `HN` / `HR` / `HP` (`type_source = group`, ou `group_multiple` si l'OU est dans plusieurs groupes — le plus specifique gagne).
2. Sinon **prefixe du nom** (`PS`, `CSR`, `CSU`, `HP`, `CHU`, `CLINIQUE`, `CABINET`, `CM`…) → `type_source = name`. Une structure privee sans prefixe reconnu devient `AUTRE_PRIVE`.
3. Sinon `INDETERMINE` (`type_source = none`).

Tout ce qui n'est pas « un seul groupe » remonte en issue **R17** : le nettoyage se fait dans DHIS2, l'app fournit la liste.

Pour **ajouter un type ou un prefixe** : `backend/internal/typologie/mapping.go` (`groupNameToCode`, `prefixToCode`, `Labels`, `groupPriority`) + `typologie_test.go` ; cote front, `frontend/src/utils/typologie.ts` (libelles) et `typeColor()` dans `frontend/src/api/public.ts` (couleur). Les codes sont structurants (le palier « normes » s'indexera dessus) : ne renommez pas un code existant sans migration.

## Export Excel

### Depuis l'interface

Connectez-vous en admin → page **Admin** → bouton **Export Excel**. Le fichier contient toutes les structures avec leurs 225 data elements et scores qualite.

### Depuis la ligne de commande

```bash
# Prerequis
pip3 install openpyxl

# Lancer une synchro puis exporter
curl -X POST http://localhost:8081/iss/api/admin/sync -H "Authorization: Bearer <token>"
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
    typologie/    Resolution du type de structure (groupes d'OU, prefixe du nom) et du statut juridique
    quality/      Moteur de regles (R1-R18, score, tests)
    usage/        Agregateurs (recensement, services, equipements, RH, commodites, rapportage, geo, couverture, snapshot public)
    sync/         Orchestrateur RunSync()
    api/          Handlers Gin + middleware JWT + export Excel
    scheduler/    Cron (robfig/cron)
  Dockerfile

frontend/
  src/
    api/          Client API type + auth JWT
    api/          Client API type + auth JWT ; public.ts = client sans jeton de l'espace public
    pages/        Dashboard, Quality, Usage, Structures, StructureDetail, Comparison, MapView, Geolocalisation, Admin, Login
    pages/public/ PublicMap, PublicFiche, About
    components/   Layout, PublicLayout, KpiCard, DataTable, ScoreBar, SeverityBadge, ExportCSV, MethodNote, charts/
    components/map/ ClusterLayer (clusters, partage public/pro), ProGeoMap, InvalidateOnResize
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
  |-- Pull metadonnees (data elements, option sets, org units avec geometrie)
  |-- Pull org units assignees au programme (pour le taux de rapportage)
  |-- Pull groupes d'OU (typologie, hopitaux, public/prive) — non bloquant
  |-- Pull population via analytics (derniere periode mensuelle × facteur) — non bloquant
  |-- Pull events (pagine, 200/page)
  |-- Enrichissement : region / district / sous-prefecture, lat-lng, type de structure
  |-- Construction du contexte qualite (couples equipement, medianes, compteurs, typologie)
  |-- Execution des regles sur chaque event, scores qualite
  |-- Agregats d'utilisation (recensement, services, equipements, RH, commodites, rapportage)
  |-- Agregats geo (usage_geo niveaux 3-4) et couverture demographique (usage_couverture)
  |-- Snapshot public (GeoJSON des points + filtres, serialise une fois, ETag)
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

Les tests couvrent les regles qualite, le calcul de score, l'integration du moteur, la typologie, les agregats geo / couverture, l'enrichissement des events, le parsing de configuration et la non-fuite de donnees dans le snapshot public.

Pour rejouer une synchro complete en ligne de commande (meme configuration que le serveur) :

```bash
set -a; source .env; set +a; cd backend && go run ./cmd/testsync
```
