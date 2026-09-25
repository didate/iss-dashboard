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
# true = espace planification ouvert en lecture (bandeau « prototype », donnees personnelles masquees) ;
# false = login obligatoire pour l'espace planification. La carte publique reste ouverte dans les deux cas.
DASHBOARD_PUBLIC=true
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
| **Carte** (`/`) | Fond OpenStreetMap, structures geolocalisees dessinees sur un canvas unique, colorees par type, regroupement par grille optionnel (desactive par defaut). Recherche par nom, filtres type / service / region / district (dans l'URL, partageables), « Autour de moi » (geolocalisation du navigateur, tri par distance cote serveur). Clic marqueur → popup ; clic dans la liste → deplacement + popup. |
| **Fiche** (`/fs/:uid`) | Identite, type, statut juridique et operationnel, rattachement, mini-carte, plateau technique, services fonctionnels, QR code, copie du lien, itineraire OSM. `uid` = UID de l'unite d'organisation DHIS2 (stable entre recensements). |
| **Annuaire** (`/annuaire`) | Registre en tableau, memes filtres que la carte, pagine, **export CSV** (open data) avec les filtres actifs. |
| **A propos** (`/a-propos`) | Sources, chiffres cles, limites. |

La fiche publique est volontairement **reduite** : pas de detail RH ni d'equipements chiffres, pas de nom/telephone du responsable, pas de qualite. Elle expose des **agregats** (effectif RH total, medecins, personnel soignant, eau aux points critiques, source d'energie, score de disponibilite des 7 services principaux — curatif, CPN, accouchement, PEV, PTME, laboratoire, pharmacie), calcules au sync (`usage.ExtrasComputer`, table `public_extra`) et montres dans le popup de la carte comme dans la fiche. Cette frontiere est garantie par l'API (`/api/public/*` ne lit que `structure_latest` et les blobs pre-calcules) et par un test (`usage/public_snapshot_test.go`) qui verifie que le GeoJSON public ne fuit rien.

### Espace planification

Ouvert en lecture quand `DASHBOARD_PUBLIC=true` (choix actuel, phase prototype) avec deux garde-fous : un bandeau
« Prototype » en tete de chaque page des deux espaces (`frontend/src/components/PrototypeBanner.tsx`, a retirer quand le
MSHP aura valide donnees et normes) et le masquage du nom / telephone du responsable dans le detail et le PDF d'une structure pour
les lecteurs non connectes (`store.StripPersonalValues`, codes `ISS_GEN_NOM_RESP_DE` / `ISS_GEN_TEL_RESP_DE`).

| Page | Description |
|---|---|
| **Vue d'ensemble** (`/tableau-de-bord`) | KPIs (structures, score qualite, erreurs, taux de rapportage, derniere synchro), graphes score par district et issues par region |
| **Qualite** | Tableau filtrable/pagine des structures a probleme (severite, regle, district, recherche). Clic → panneau de detail. Export CSV. |
| **Utilisation** | Onglets : Rapportage, Recensement (dont par type de structure), Plateau technique, Services, Matrice, Equipements, RH, Commodites, **Couverture** (ratios pour 10 000 habitants), Structures fermees |
| **Structures** | Liste filtrable (district, type, GPS, nom) → detail complet (type et sa source, sous-prefecture, coordonnees, lien vers la fiche publique, issues, valeurs) |
| **Comparaison** | Comparaison de districts |
| **Carte** | Choropletes par district (rapportage, qualite, services, equipements, WASH, RH) + **Couverture geo** (district ou sous-prefecture : % GPS, score, structures /10 000 hab., nombre) + **Structures (points)** colores par score qualite |
| **GPS** (`/geolocalisation`) | Couverture GPS nationale et par district / sous-prefecture, liste des structures sans coordonnees, export CSV pour les equipes terrain |
| **Normes** (`/conformite`) | Conformite des structures au referentiel de normes actif : score et % conformes par type et par zone, table des ecarts « il manque X de Y dans Z » exportable, liste des structures par statut → detail (chaque exigence attendu / observe) |
| **Personnel** (`/personnel`) | Agents payes par l'Etat (fichier DRH/CNPS) : KPI (effectif, densite /10 000 hab., part en structure, departs a 5 ans), repartition par categorie et par lieu d'affectation, effectifs par region / district / sous-prefecture / type, pyramide des ages, comparaison DRH ↔ ISS avec les incoherences mises en evidence, et — quand un district est choisi — ses structures avec leur effectif, celles sans aucun agent comprises |
| **Admin** | Synchronisation manuelle, export Excel, gestion des utilisateurs, historique des synchros ; onglet **Normes** (versions du referentiel, editeur, import/export CSV, activation) ; onglet **Personnel (DRH)** (import d'un millesime, rapport de rattachement, libelles non reconnus, table de correspondance) |

## Configuration

| Variable | Description | Defaut |
|---|---|---|
| `DHIS2_BASE_URL` | URL de l'instance DHIS2 | — |
| `DHIS2_PAT` | Personal Access Token DHIS2 | — |
| `DHIS2_PROGRAM_ID` | ID du programme ISS | `AJy1cnAA50U` |
| `SQLITE_PATH` | Chemin du fichier SQLite | `./iss.db` |
| `SYNC_CRON` | Expression cron pour la synchro auto | `0 */6 * * *` |
| `ADMIN_TOKEN` | Mot de passe du compte admin par defaut | — |
| `DASHBOARD_PUBLIC` | `true` = espace planification lisible sans connexion (le nom et le telephone du responsable restent masques aux anonymes, l'export Excel et l'admin exigent un login) ; `false` = tout l'espace planification derriere JWT. `/api/public/*` reste toujours ouvert | `true` |
| `DRH_AGE_RETRAITE` | Age de depart a la retraite retenu pour les projections de depart (aucune reference officielle trouvee pour la fonction publique guineenne) | `60` |
| `PORT` | Port du backend (en Docker, le conteneur reste sur 8080 : `docker-compose*.yml`) | `8081` |
| `VITE_API_BASE_URL` | URL du backend **avec le prefixe `/iss`** (build-time frontend) | `http://localhost:8081/iss` |
| `VITE_TILE_URL` | Fond de carte (build-time frontend). Le defaut est `https://tile.openstreetmap.org/{z}/{x}/{y}.png` — **sans** sous-domaines `{s}.`, l'ancienne forme etant desormais bloquee par OSM (« App is not following the tile usage policy »). Alternatives sans cle : Esri `https://server.arcgisonline.com/ArcGIS/rest/services/World_Street_Map/MapServer/tile/{z}/{y}/{x}`, OSM-FR `https://{s}.tile.openstreetmap.fr/osmfr/{z}/{x}/{y}.png`. CARTO exige une cle depuis 2025 | tuiles OSM |
| `VITE_TILE_ATTRIBUTION` | Attribution affichee sur les cartes, a changer avec le fond | attribution OSM |

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

### Personnel de l'Etat — DRH/CNPS (JWT + role admin)

| Methode | Route | Description |
|---|---|---|
| `POST` | `/iss/api/admin/drh/import` | Importe un millesime (`file` multipart, `.csv` ou `.csv.gz`, + `annee`, `label`, `age_retraite`), strict : une ligne invalide → rien n'est importe |
| `GET` | `/iss/api/admin/drh/imports` | Liste des millesimes |
| `POST` | `/iss/api/admin/drh/imports/:id/activate` | Rend ce millesime actif (les autres sont archives) |
| `DELETE` | `/iss/api/admin/drh/imports/:id` | Supprime un millesime et ses agregats |
| `GET` | `/iss/api/admin/drh/imports/:id/non-reconnus[.csv]` | Libelles non rattaches, avec leur effectif |
| `GET` / `PUT` | `/iss/api/admin/drh/correspondances` | Table DRH → ISS / remplacement complet par CSV |
| `GET` | `/iss/api/admin/drh/correspondances/export.csv` | Export au format d'import |

### Normes (JWT + role admin)

| Methode | Route | Description |
|---|---|---|
| `GET` / `POST` | `/iss/api/admin/normes` | Liste des versions / creation d'un brouillon `{name, notes}` |
| `PUT` / `DELETE` | `/iss/api/admin/normes/:id` | Nom et notes / suppression (brouillon seulement) |
| `POST` | `/iss/api/admin/normes/:id/duplicate` | Nouvelle version brouillon copiee |
| `POST` | `/iss/api/admin/normes/:id/activate` | Archive l'actif, active celui-ci, recalcule la conformite |
| `GET` / `PUT` | `/iss/api/admin/normes/:id/rules` | Regles / remplacement complet (chaque regle validee contre le catalogue) |
| `POST` | `/iss/api/admin/normes/:id/rules/import?mode=replace\|append` | Import CSV (`file` multipart), strict : une ligne invalide → rien n'est importe, erreurs par ligne |
| `GET` | `/iss/api/admin/normes/:id/rules/export.csv` | Export CSV au format d'import |
| `GET` | `/iss/api/admin/normes/targets` | Catalogue des cibles admissibles (services, profils RH, equipements, infra) |
| `POST` | `/iss/api/admin/normes/recompute` | Recalcul manuel de la conformite |

### Personnel de l'Etat (lecture, meme regle d'acces que l'espace planification)

| Methode | Route | Description |
|---|---|---|
| `GET` | `/iss/api/drh/summary` | Millesime actif, effectif national, densite, repartition par categorie, catalogue |
| `GET` | `/iss/api/drh/effectifs?by=global\|region\|district\|sous_prefecture\|type\|centrale&categorie=&key=&district=` | Effectifs pre-calcules (`categorie=*` renvoie toutes les categories detaillees ; `by=centrale` detaille les directions et programmes) |
| `GET` | `/iss/api/drh/pyramide?by=global\|region\|district\|type&key=&categorie=` | Tranches quinquennales, tranches vides comprises |
| `GET` | `/iss/api/drh/comparaison?by=global\|district&categorie=` | DRH vs ISS : effectifs, ecart, ratio, les ratios les plus bas d'abord |
| `GET` | `/iss/api/drh/structures?district=&search=` | Effectif par structure, y compris les structures sans aucun agent |
| `GET` | `/iss/api/drh/structure/:uid` | Personnel de l'Etat affecte a une structure, par categorie |

Sans millesime importe, ces routes repondent `404` : le front masque la page au lieu d'afficher des graphes vides.
`/iss/api/map/geo` porte en plus `drh_ratio_10k` et `drh_depart_5ans_pct` par district et sous-prefecture.

### Conformite (lecture, meme regle d'acces que l'espace planification)

| Methode | Route | Description |
|---|---|---|
| `GET` | `/iss/api/meta/normes` | Referentiel actif et dernier calcul |
| `GET` | `/iss/api/conformite/summary?by=global\|region\|district\|sous_prefecture\|type&type=` | Score moyen, structures evaluees, % conformes |
| `GET` | `/iss/api/conformite/gaps?by=&key=&type=&kind=&level=&limit=` | Ecarts tries par deficit |
| `GET` | `/iss/api/conformite/structures?region=&district=&sous_prefecture=&type=&status=conforme\|non_conforme\|non_evalue&search=&page=` | Structures avec score et manques |
| `GET` | `/iss/api/quality/event/:uid` | Existant, + bloc `conformite` (exigences attendu / observe / statut) |
| `GET` | `/iss/api/map/geo?level=` | Existant, + `conformite_score`, `pct_conformes` |

### Export (JWT requis)

| Methode | Route | Description |
|---|---|---|
| `GET` | `/iss/api/export/excel` | Telecharge un fichier Excel avec toutes les donnees brutes |

### Espace public (toujours ouvert, projections reduites)

| Methode | Route | Description |
|---|---|---|
| `GET` | `/iss/api/public/points.geojson` | GeoJSON pre-calcule de toutes les structures geolocalisees (uid, nom, type, statut, rattachement, services `oui`, agregats RH / eau / energie / score services). ETag, gzip, `Cache-Control: no-cache` : le navigateur revalide a chaque chargement et recoit un 304 tant que la synchro n'a rien change |
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
| `GET` | `/iss/api/map/geo?level=3\|4` | Polygones + proprietes de couverture, dont `ratios` et `numerators` par indicateur, `conformite_score` et les deux metriques de personnel `drh_ratio_10k` / `drh_depart_5ans_pct` |
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

## Normes et conformite

Le palier « normes » compare chaque structure a ce qu'elle *devrait* avoir pour son type. Le referentiel est une
**donnee editee par l'administration**, pas du code : aucun document officiel n'etant disponible, il est saisi et
versionne dans l'app, et le texte du MSHP s'y importera le jour venu.

### Cycle de vie d'un referentiel

```
Creer (brouillon) → importer un CSV ou editer la grille → Activer
                                                          ↓
                   l'ancien actif est archive ; la conformite est recalculee
                                                          ↓
      pour modifier sans casser l'actif : Dupliquer → nouvelle version brouillon → … → Activer
```

Une seule version est active ; les versions archivees restent consultables (tracabilite : « ce rapport a ete
produit avec le referentiel v2 »). La conformite est recalculee a chaque activation, a chaque modification du
referentiel actif, en fin de synchronisation, ou a la demande — sans appel a DHIS2 (`sync.RecomputeConformite`).

### Format CSV (`;`, UTF-8, lignes `#` ignorees)

```
type_code;kind;target;label;min_value;level
CS;service;ISS_SVC_CPN_DE;;1;essentiel
CS;rh;ISS_RH_SAGEF;Sage-femme;1;essentiel
HP;rh;ISS_RH_MED_;Medecin (tout profil);3;essentiel
HP;equipement;ISS_EQUI_TABLE_OP;;1;essentiel
*;infra;ISS_INFRA_LATRINES_DE;Latrines;1;essentiel
```

| Colonne | Valeurs |
|---|---|
| `type_code` | `PS`, `CS`, `CSA`, `CMC`, `HP`, `HR`, `HN`, `CABINET`, `CLINIQUE`, `AUTRE_PRIVE` ou `*` (tous types). Une regle specifique au type prime sur `*` pour la meme cible |
| `kind` | `service` (cible = code DE `ISS_SVC_*`, attendu = `oui`), `rh` (racine de profil `ISS_RH_*` ; un suffixe `_` = prefixe, ex. `ISS_RH_MED_` = tout medecin ; tous statuts d'emploi sommes), `equipement` (racine du couple total/fonctionnel ; le minimum porte sur les unites **fonctionnelles**), `infra` (code DE `ISS_INFRA_*`) |
| `target` | Doit exister dans le catalogue (`GET /admin/normes/targets`, construit depuis les metadonnees ISS) — sinon la ligne est rejetee |
| `label` | Libelle affiche ; vide = repris du catalogue |
| `min_value` | Minimum ; force a 1 pour un service |
| `level` | `essentiel` (poids 2) ou `recommande` (poids 1) |

`docs/normes-exemple.csv` est un referentiel de demonstration **explicitement non officiel** (212 regles, seuils
indicatifs) qui s'importe tel quel ; il sert a voir l'outil vivre, pas a evaluer le pays.

### Evaluation

- Par structure (dernier recensement de l'org unit), chaque exigence de son type donne un statut : **ok**,
  **manque** (observe < minimum) ou **inconnu** (donnee non renseignee dans ISS).
- **Score** = 100 × Σ poids(ok) ÷ Σ poids(ok + manque). Les inconnus sont hors denominateur : la qualite des
  donnees ne contamine pas la conformite (elle est traitee par le moteur qualite).
- **Conforme** = aucune exigence essentielle manquante. Critere binaire et severe par construction.
- Agregats (`conformite_summary`, `conformite_gap`) par region / district / sous-prefecture / type ; le deficit
  d'un ecart = total a combler (effectifs, unites) pour que toutes les structures concernees atteignent le minimum.
- Structures privees : memes normes que le public du meme type (aucune regle privee dans l'exemple : seules les
  regles `*` s'appliquent, d'ou des taux de conformite trompeurs a 100 %).

Un « manque » peut etre une erreur de saisie (valeur 0 dans ISS) : verifier le detail de la structure avant de
conclure a un deficit reel.

### Etendre

- Nouvelle famille d'exigence : `normes.Kinds` + `Evaluator.observe` (`backend/internal/normes/evaluate.go`) +
  `BuildNormeCatalog` (`backend/internal/store/normes_store.go`) + un test dans `evaluate_test.go`.
- Ponderation ou definition de « conforme » : `normes.Summarize`.
- Nouveau type de structure : voir « Typologie des structures » — les regles s'indexent sur ces codes.

## Personnel de l'Etat (DRH/CNPS)

Deuxieme source de donnees, independante de DHIS2 : le fichier annuel des agents **payes par l'Etat**,
transmis par la DRH du Ministere. A ne pas confondre avec l'effectif **present** declare par les structures
dans ISS — l'ecart entre les deux est justement une information (part du personnel hors fonction publique,
defaut de declaration, agents affectes mais absents).

### Confidentialite

Le fichier source est nominatif ; l'application n'en a besoin qu'en effectifs.

1. Le `.xlsx` de la DRH est converti en **CSV normalise** par `scripts/drh_xlsx_to_csv.py`, qui laisse de cote
   matricule, nom, date de naissance exacte et poste occupe (seule l'**annee** de naissance est gardee).
2. L'import **rejette toute colonne inconnue** : un fichier nominatif ne peut pas entrer par inadvertance.
3. **Aucune ligne par agent n'est persistee.** L'ingestion calcule les agregats et ne garde qu'eux
   (`drh_effectif`, `drh_pyramide`), croises zone x profession x tranche d'age x sexe.
4. Rien n'est expose dans l'espace public.

Le convertisseur supprime aussi les **lignes saisies deux fois** (meme matricule et meme contenu) : 125 sur le
millesime 2026, soit 10 037 agents au lieu de 10 162. Les matricules en doublon dont les lignes different sont
conserves et listes pour arbitrage par la DRH (`--doublons`), et les matricules de remplissage (`ND`, `0`) ne
sont jamais traites comme des identifiants.

Le format des colonnes est decrit dans [`docs/drh-format.md`](docs/drh-format.md) — c'est le document a
transmettre a la DRH pour les millesimes suivants.

### Importer un millesime

```bash
python3 scripts/drh_xlsx_to_csv.py "CNPS DRH 2026.xlsx" drh-2026.csv
```

Puis **Admin → Personnel (DRH)** → millesime → *Importer un fichier*. Le `.csv.gz` est accepte : le fichier
annuel fait 1,4 Mo, au-dela de la limite d'envoi par defaut d'nginx (`client_max_body_size`, 1 Mo), qui le
rejette en `413` avant meme que la requete atteigne l'application — 70 Ko compresse, le probleme disparait. L'ecran affiche le rapport : agents lus,
rattaches a une structure, en bureau de district, en administration centrale, non rattaches, et les libelles
non reconnus (exportables en CSV pour arbitrage). Chaque import cree un millesime ; le precedent est archive,
pas supprime, ce qui permet de comparer dans le temps.

Pour un essai a blanc, sans toucher la base de production :

```bash
cd backend && go run ./cmd/drhcheck /chemin/copie-de-iss.db drh-2026.csv correspondances.csv 2026
```

### Rattachement aux structures ISS

`structure_affectation` est un texte libre : il ne correspond pas toujours au nom ISS. `internal/drh/resolve.go`
essaie, dans cet ordre :

| Ordre | Regle | Source |
|---|---|---|
| 1 | Table de correspondance validee a la main | `table` |
| 2 | Nom normalise identique a une structure ISS | `exact` |
| 3 | Sigle de bureau de district (`DPS`, `DCS`, `IRS`, `DSP`) ou d'administration centrale / institut / programme national | `prefixe` |
| 3 bis | Service heberge par l'hopital du district (`CT-EPi`…) → l'hopital prefectoral de la prefecture de l'agent | `service_district` |
| 4 | Type devine + nom propre, dans le district de l'agent | `approx` |
| 5 | Seul etablissement de ce type dans le district | `deduit` |
| — | Rien de tout cela → **non rattache**, visible dans le rapport | `inconnu` |

Ce qui reste ambigu n'est jamais rattache au hasard. Sur le millesime 2026 : **99,3 % des agents categorises**
(6 435 en structure sur 388 structures, 2 707 en bureau de district, 829 en administration centrale et
programmes, 66 non rattaches, sur 10 037 agents apres dedoublonnage).

La table de correspondance est une **donnee editable**, pas du code : elle est embarquee comme graine
(`backend/internal/drh/seed/correspondances.csv`, chargee une seule fois sur une base neuve), puis remplacable
par CSV depuis l'ecran d'admin.

### Agregats

Chaque import ecrit les cellules au grain le plus fin (une structure, un bureau de district, l'administration
centrale, ou un libelle non rattache) x categorie professionnelle. Les rollups en sont deduits :

| Dimension | Qui y compte |
|---|---|
| `global` | tout le monde |
| `region`, `district` | les structures de la zone, son bureau de district, et les agents dont le libelle n'a pas pu etre rattache — ce sont de vrais agents de la prefecture, les ecarter sous-estimerait la zone |
| `sous_prefecture`, `type` | uniquement les agents affectes a une structure, les seuls dont on connaisse le lieu exact et le type |
| `centrale` | une cle par direction, institut ou programme national (42 entites sur 2026), lisible via `GET /drh/effectifs?by=centrale` |

L'administration centrale ne compte qu'au national : elle n'est pas « dans » le district dont elle a l'adresse.
La densite pour 10 000 habitants utilise la meme population DHIS2 que le reste de l'application. Le **taux de
depart** se calcule sur `n_age_connu`, pas sur l'effectif total : 663 agents du millesime 2026 n'ont pas d'annee
de naissance, et les inclure au denominateur ferait passer le risque pour plus faible qu'il n'est.

Les rollups, les densites et la comparaison sont **recalcules a chaque synchronisation DHIS2** (`RecomputeDrh`) :
le fichier de personnel ne bouge pas, mais la population et les effectifs ISS auxquels on le compare, si.

### Lire la comparaison DRH / ISS

Les deux sources ne mesurent pas la meme chose : ISS compte le personnel **present** declare par la structure,
la DRH ceux qu'elle **paie**. Un ratio ISS/DRH superieur a 1 est donc normal et mesure la part de personnel hors
fonction publique (national 2026 : medecins generalistes x1,5, ATS x3,7, sages-femmes x4,0, infirmiers x4,9).

Un ratio **inferieur a 1** est une anomalie : l'Etat paie plus d'agents que les structures n'en declarent. Soit
un defaut de declaration ISS, soit des agents affectes mais absents. Sur 2026, pour les medecins generalistes :
Mali (17 payes / 6 declares), Dabola (13/5), Coyah (48/22), Gaoual (10/6), Telimele (28/17), Mandiana (30/19).
Ni l'une ni l'autre source ne pouvait le reveler seule.

Seules les categories ayant un equivalent ISS sont comparees, et la ligne « toutes categories » somme des deux
cotes les **memes** categories — celles qu'ISS a effectivement renseignees — pour que l'ecart ne mesure pas un
trou de nomenclature.

### Ou cela se lit dans l'interface

- **Page Personnel** (`/personnel`) : la lecture complete, filtrable par district et par categorie.
- **Vue d'ensemble** : une ligne « Personnel de l'Etat » avec la densite nationale, masquee tant qu'aucun millesime
  n'est importe.
- **Carte → Couverture geo** : deux metriques, « Agents de l'Etat pour 10 000 hab. » et « % de departs a la retraite
  d'ici 5 ans » (echelle inversee : un taux eleve est un risque, donc rouge).
- **Detail d'une structure** : bloc « Personnel de l'Etat affecte », par categorie, avec les departs a 5 ans.

L'onglet RH d'**Utilisation** n'a volontairement pas ete touche : la comparaison DRH ↔ ISS de la page Personnel
dit la meme chose en plus complet (par categorie *et* par district), une colonne de plus y aurait fait doublon.

La repartition par **categorie hierarchique** (A1/A2/B1/B2/C/D) prevue au plan n'est pas affichee : la hierarchie
est lue dans le CSV mais n'est agregee dans aucune dimension. L'ajouter demande une colonne de plus dans
`drh_effectif` et un re-import.

### Etendre

- **Nouveau metier mal classe** : ajouter un motif dans `regles` ou `specialites`
  (`backend/internal/drh/professions.go`), et le cas dans `TestCategorieDe`. Un libelle inconnu tombe dans
  `AUTRE` plutot que d'etre perdu.
- **Nouvelle categorie** : l'ajouter a `drh.Categories` avec sa famille et, si un equivalent existe, le
  `profil_code` ISS correspondant — c'est ce qui rend la comparaison DRH ↔ ISS possible.
- **Nouveau service heberge par l'hopital du district** : une ligne dans `servicesDuDistrict`
  (`resolve.go`) et un cas dans `TestResolveServiceDuDistrict`. Ces libelles ne peuvent pas passer par la table
  de correspondance : le meme libelle existe dans plusieurs districts et doit se resoudre differemment dans
  chacun. Le rattachement vise l'hopital prefectoral, a defaut regional, a defaut national — et s'abstient
  quand le district en compte deux du meme type.
- **Nouveau sigle d'administration centrale** : `centralePrefixe` dans `resolve.go`. Le sigle doit rester
  majoritairement en majuscules (`estSigle`) : sans ce garde-fou le motif des programmes nationaux happait
  « Pneumologie » et rangeait un service hospitalier parmi les programmes.

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
    store/        SQLite (migrations, persistance transactionnelle, queries, users, normes_store, conformite_store)
    typologie/    Resolution du type de structure (groupes d'OU, prefixe du nom) et du statut juridique
    quality/      Moteur de regles (R1-R18, score, tests)
    normes/       Referentiel de normes (format CSV, catalogue, evaluation, agregats, tests)
    drh/          Personnel de l'Etat : parseur CSV, rattachement aux structures, categories de metier, agregats
    usage/        Agregateurs (recensement, services, equipements, RH, commodites, rapportage, geo, couverture, snapshot public)
    sync/         Orchestrateur RunSync(), RecomputeConformite(), RunDrhImport()
    api/          Handlers Gin + middleware JWT + export Excel
    scheduler/    Cron (robfig/cron)
  cmd/drhcheck/ Essai a blanc d'un millesime DRH sur une copie de la base
  Dockerfile

frontend/
  src/
    api/          Client API type + auth JWT ; public.ts = client sans jeton de l'espace public
    pages/        Dashboard, Quality, Usage, Structures, StructureDetail, Comparison, MapView, Geolocalisation, Conformite, Personnel, Admin, Login
    pages/admin/  NormesEditor, DrhImport
    pages/public/ PublicMap, PublicFiche, About
    components/   Layout, PublicLayout, KpiCard, DataTable, ScoreBar, SeverityBadge, ExportCSV, MethodNote, charts/
    components/map/ BaseTileLayer (fond de carte unique, fournisseur configurable), PointsCanvasLayer (points sur canvas unique, partage public/pro), ProGeoMap, GeoLabels, IndicatorHelp, ConakryInset, InvalidateOnResize
    types/        Types TypeScript miroir de l'API
    utils/        Helpers (formatage nombres)
  Dockerfile
  nginx.conf

scripts/
  export_excel.py        Export Excel en ligne de commande
  drh_xlsx_to_csv.py     Conversion du .xlsx annuel de la DRH en CSV normalise (depersonnalise)
  README.md

docs/
  normes-exemple.csv    Referentiel de normes de demonstration (non officiel)
  drh-format.md         Colonnes attendues du fichier de personnel, a transmettre a la DRH

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
  |-- Persistance atomique (DELETE + INSERT dans une seule transaction SQLite)
  +-- Conformite au referentiel de normes actif (RecomputeConformite, non bloquant)
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

Les tests couvrent les regles qualite, le calcul de score, l'integration du moteur, la typologie, les agregats geo / couverture, l'enrichissement des events, le parsing de configuration, la non-fuite de donnees dans le snapshot public, le format et la validation des normes, l'evaluation de la conformite (statuts, priorite des regles, ponderation) et ses agregats, et le middleware d'acces (mode public / protege).

Pour rejouer une synchro complete en ligne de commande (meme configuration que le serveur) :

```bash
set -a; source .env; set +a; cd backend && go run ./cmd/testsync
```
