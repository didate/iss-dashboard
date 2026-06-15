# Mission : Tableau de bord Qualité & Utilisation des données — Programme DHIS2 « ISS »

Tu vas construire une mini-application web de tableau de bord pour un programme DHIS2 de recensement de structures sanitaires. L'application a **deux axes** : la **qualité des données** (axe principal) et l'**utilisation / analyse descriptive** des structures. Lis entièrement ce brief avant de commencer, puis propose un plan d'implémentation que je validerai avant que tu codes.

---

## 1. Architecture imposée

- **Backend** : Go + Gin. Expose une API REST JSON.
- **Frontend** : React (Vite). Séparé du backend. **Ne fait QUE de l'affichage** : il consomme des endpoints qui renvoient des données déjà calculées. Aucun calcul métier, aucune règle qualité dans le front.
- **Base de données** : **SQLite** (fichier local, via `modernc.org/sqlite` — driver pur Go, pas de CGO, pour éviter les soucis de compilation).
- **Déploiement** : **deux conteneurs Docker distincts** (backend, frontend) orchestrés par `docker-compose.yml`, derrière Traefik (labels Traefik à inclure). **PAS de single-binary** — backend et front sont des images séparées. Le front est buildé en statique et servi par nginx.
- **Auth DHIS2** : le backend s'authentifie auprès de DHIS2 avec un **Personal Access Token (PAT)** stocké côté backend via variable d'environnement. Le PAT n'est JAMAIS exposé au front.

### Principe central : pré-calcul
Toute la logique (pull DHIS2 → parsing → application des règles qualité → agrégations d'utilisation) s'exécute **côté backend lors de l'ingestion**, et les résultats sont **persistés dans SQLite**. Les endpoints de lecture servent ces tables pré-calculées (réponses rapides, pas de recalcul à la volée). Le front lit et affiche, point.

### Déclenchement de l'ingestion
1. **Bouton admin** : un endpoint protégé `POST /api/admin/sync` lance un pull + recalcul complet. Le front a un écran admin avec un bouton « Synchroniser maintenant » et l'état de la dernière synchro (date, nb événements, durée, statut).
2. **Job planifié** : un scheduler interne (ex. `robfig/cron`) lance la même synchro automatiquement (intervalle configurable via env, ex. toutes les 6h). 
3. Les deux passent par **exactement le même** pipeline d'ingestion (factorise dans une fonction `RunSync()`).

L'écran d'accueil utilisateur ne déclenche jamais de pull ; il lit le dernier snapshot calculé.

---

## 2. La source de données : DHIS2

- Programme **événementiel sans inscription** (`WITHOUT_REGISTRATION`).
- **ID programme** : `AJy1cnAA50U` (« ISS Informations des Structures »).
- Une structure sanitaire = un **event**. L'org unit de l'event = la structure (ou son rattachement).
- **Endpoint de pull** (paginer, ne pas tout charger en mémoire d'un coup) :
  ```
  GET {DHIS2_BASE_URL}/api/events.json?program=AJy1cnAA50U
      &fields=event,orgUnit,orgUnitName,eventDate,status,dataValues[dataElement,value]
      &pageSize=200&page=N
  ```
  Header : `Authorization: ApiToken {PAT}`.
- **Métadonnées** (à puller aussi, pour les libellés et optionSets) :
  ```
  GET /api/dataElements.json?filter=name:like:ISS&fields=id,name,code,valueType,optionSet[id]&paging=false
  GET /api/optionSets.json?fields=id,name,options[code,name]&paging=false
  GET /api/organisationUnits.json?fields=id,name,level,parent[id,name]&paging=false  (pour le rattachement district/région)
  ```
- Persiste un cache local des métadonnées en SQLite (table `metadata_de`, `option_sets`) pour traduire UID → libellé sans rappeler DHIS2 à chaque lecture.

### Convention des data elements
Les DE ont des **codes** structurés : `ISS_<SECTION>_<ABREV>_<SUFFIXE>_DE`. Les sections sont préfixées dans le `name` aussi : `ISS_GEN`, `ISS_EQ`, `ISS_INFRA`, `ISS_RH`, `ISS_RH_SPE`, `ISS_LAB`, `ISS_COMMO`, `ISS_SVC`. **Identifie les DE par leur `code` quand c'est possible** (plus stable que l'UID entre instances), avec fallback sur UID. Les couples équipement suivent le motif `..._TOTAL_DE` / `..._FONC_DE`.

### OptionSets — comparer les CODES, pas les noms
Les valeurs stockées dans les events sont des **codes** d'option (ex. `'oui'`, `'operationnel'`, `'publique'`, `'aucune'`). Quand tu évalues une règle sur un champ à optionSet, compare au code. Récupère les codes réels depuis le pull des optionSets (ne les devine pas).

---

## 3. AXE 1 — Qualité des données (priorité)

Pour **chaque event**, le backend calcule une liste de problèmes (`issues`), chacun avec : `code` (identifiant règle), `severity` (`error` | `warning` | `info`), `rule` (catégorie lisible), `message` (texte précis avec valeurs). Persiste dans une table `quality_issue` (une ligne par issue) + une table `event_quality` (synthèse par event : nb d'issues par sévérité, pire sévérité, score).

### Règles à implémenter

**R1 — Champs obligatoires** (`severity`: warning, sauf date = error)
- Statut opérationnel manquant (DE code `ISS_GEN_STATUT_OPER_*` / UID `HpjvSNCEWM0`).
- Nom du responsable manquant (UID `GLngjZxh1Vm`).
- Date d'event absente (`error`).

**R2 — Cohérence total / fonctionnel** (`error`) — pour les **35 couples** d'équipements
- Pour chaque couple (`*_TOTAL_DE`, `*_FONC_DE`) : si fonctionnel ET total renseignés et `fonctionnel > total` → erreur `« {équipement} : fonctionnel ({f}) > total ({t}) »`.
- Si fonctionnel > 0 mais total vide → `warning` « total manquant ».
- **Découvre les couples dynamiquement** en appariant les DE dont le code finit par `_TOTAL_DE` et `_FONC_DE` avec la même racine. Ne pas coder les 35 paires en dur ; dériver des métadonnées.

**R3 — Service déclaré fonctionnel sans support** (`warning`)
- Service laboratoire = `'oui'` (UID `Zq34u53MgeI`) mais 0 microscope fonctionnel (UID `bWGkmx4RfoE`) ET 0 salle de labo.
- (Extensible : maternité sans table d'accouchement, bloc sans table opératoire — prévois la structure pour ajouter ces règles facilement, via une liste de specs `{service_field, support_fields[], message}`.)

**R4 — Cohérence commodités** (`warning` / `info`)
- « Dispose d'une source d'énergie » = vrai (UID `G6aAGwMfuOH`) mais aucune source cochée (réseau `ZKd23M3NVu0`, solaire `Z5G3epiH9hh`, générateur `Ff1uAvJbxXm` tous faux/vides) → warning.
- Eau aux points critiques = vrai (UID `mr2SQNgReyd`) mais source d'eau = `'aucune'` ou vide (UID `IzfXJ0Zrfxh`) → info.

**R5 — Valeurs aberrantes** (`info`)
- Pour chaque DE numérique d'équipement total : calcule médiane + MAD sur l'ensemble des events, signale les valeurs > médiane + 5×MAD ET > 50 en absolu. Calcul fait à l'ingestion, sur le jeu complet.

**R6 — Doublons** (`warning`)
- Plusieurs events actifs sur la même org unit → signale comme doublon potentiel.

**R7 — Complétude** (`info`)
- Event dont toutes les sommes d'équipements/RH sont nulles ou vides → structure « coquille vide » suspecte.

> Conçois le moteur de règles comme une liste de fonctions `Rule(event, context) []Issue` où `context` porte les stats globales (médianes), la table de traduction, et les couples découverts. Ajouter une règle = ajouter une fonction. Documente comment en ajouter.

### Score qualité
- Par event : `score = 100` si 0 issue, sinon dégressif (ex. −15 par error, −5 par warning, −1 par info, plancher 0). Persiste.
- Agrégats : score moyen global, par district, par région, par type de structure (public/privé). Pré-calculés dans des tables de synthèse.

---

## 4. AXE 2 — Utilisation / analyse descriptive

Pré-calcule et persiste ces agrégats (tables `usage_*`), servis tels quels au front :

- **Recensement** : nb structures total, opérationnelles, par statut juridique (public/parapublic/privé lucratif/non lucratif/confessionnel), par district et région.
- **Disponibilité des services** : pour chaque service de la section `ISS_SVC`, nb et % de structures où il est `'oui'` (fonctionnel). Matrice service × district.
- **Fonctionnalité des équipements** : pour chaque couple, taux = Σfonctionnel / Σtotal (%). Focalise sur chaîne du froid (réfrigérateurs, congélateurs, porte-vaccins), imagerie, lits, microscopes, ambulances.
- **Ressources humaines** : effectifs totaux par profil (DE `ISS_RH_*`), répartition par statut d'emploi (fonctionnaire/contractuel/bénévole), densité pour 10 000 hab. si une population est disponible (sinon, prévoir le champ et laisser nul).
- **Commodités (WASH)** : % structures avec énergie, avec eau aux points critiques, avec énergie solaire.
- **Plateau technique** : % structures avec labo fonctionnel, maternité, bloc opératoire, imagerie.

---

## 5. Schéma SQLite (proposition — affine si besoin)

```
sync_run(id, started_at, finished_at, status, events_pulled, duration_ms, error_text)
event(event_uid PK, org_unit_uid, org_unit_name, district, region, event_date, status, raw_json, sync_run_id)
event_value(event_uid, de_uid, de_code, value)            -- valeurs parsées
metadata_de(de_uid PK, code, name, value_type, option_set_id, section_prefix)
option_set(option_set_id, code, name)
quality_issue(id, event_uid, code, severity, rule, message, sync_run_id)
event_quality(event_uid PK, n_error, n_warning, n_info, worst_severity, score)
usage_recensement(dimension, key, label, n_structures, ...)   -- ou tables dédiées par axe
usage_service(service_code, service_label, district, n_oui, n_total, pct)
usage_equipement(equip_root, label, sum_total, sum_fonct, pct_fonct, district)
usage_rh(profil_code, label, statut, effectif, district)
quality_summary(dimension, key, label, avg_score, n_error, n_warning, n_info, n_structures)
```
Chaque synchro insère un nouveau `sync_run` et **remplace** les tables dérivées (ou versionne par `sync_run_id` puis purge les anciennes — au choix, documente).

---

## 6. API REST (backend → front)

**Admin (protégé par un token applicatif, header `X-Admin-Token`, valeur en env)**
```
POST /api/admin/sync              -> lance RunSync(), renvoie sync_run
GET  /api/admin/sync/status       -> état de la dernière/courante synchro
```

**Lecture (publique ou protégée selon DASHBOARD_PUBLIC env)**
```
GET /api/summary                  -> KPIs globaux : nb structures, score qualité moyen, compteurs sévérité, date dernière synchro
GET /api/quality/summary?by=district|region|statut   -> scores agrégés
GET /api/quality/issues?severity=&rule=&district=&search=&page=&pageSize=
                                  -> liste paginée des structures à problème (triée par pire sévérité)
GET /api/quality/event/{event_uid} -> détail : toutes les issues + valeurs clés de la structure
GET /api/usage/recensement?by=district|region|statut
GET /api/usage/services?district=
GET /api/usage/equipements?focus=chaine_froid|imagerie|all
GET /api/usage/rh?by=profil|statut&district=
GET /api/meta/filters             -> listes pour les filtres (districts, régions, règles, services)
```
Toutes les réponses sont du JSON déjà calculé. Pagination côté serveur pour les listes.

---

## 7. Frontend React (affichage seulement)

Pages :
1. **Vue d'ensemble** : cartes KPI (structures analysées, score qualité global, compteurs error/warning/info, date dernière synchro). Graphes de synthèse (score par district, problèmes par règle).
2. **Qualité** : tableau filtrable/paginé des structures à problème (filtres : sévérité, règle, district, recherche par nom). Clic → panneau de détail (liste des issues + valeurs de la structure). Badges de sévérité colorés.
3. **Utilisation** : onglets recensement / services / équipements / RH / commodités, avec tableaux et graphes (recharts).
4. **Admin** : bouton « Synchroniser maintenant » (appelle `/api/admin/sync` avec le token admin saisi/stocké en session), affichage de l'état et de l'historique des `sync_run`.

Contraintes front : pas de logique métier, juste fetch + affichage + filtres (les filtres passent en query params au backend, qui filtre sur les tables pré-calculées). Tableaux denses, lisibles, esthétique sobre type console d'analyse. Recharts pour les graphes. Pas de localStorage pour des données métier (juste éventuellement le token admin en mémoire de session).

---

## 8. Configuration (.env)

Backend :
```
DHIS2_BASE_URL=https://...
DHIS2_PAT=...                  # Personal Access Token
DHIS2_PROGRAM_ID=AJy1cnAA50U
SQLITE_PATH=/data/iss.db
SYNC_CRON=0 */6 * * *          # toutes les 6h
ADMIN_TOKEN=...                # protège les endpoints admin
DASHBOARD_PUBLIC=true          # si false, lecture aussi protégée
PORT=8080
```
Front :
```
VITE_API_BASE_URL=https://api.dashboard...   # URL du backend
```

---

## 9. Livrables attendus

```
/backend
  main.go
  internal/dhis2/        (client API, pagination, auth PAT)
  internal/store/        (SQLite, migrations, requêtes)
  internal/quality/      (moteur de règles : une fonction par règle + registry)
  internal/usage/        (calculs d'agrégats d'utilisation)
  internal/sync/         (RunSync : pull -> parse -> rules -> aggregate -> persist)
  internal/api/          (handlers Gin, routes, middleware auth)
  internal/scheduler/    (cron)
  Dockerfile
  go.mod
/frontend
  src/ (pages, components, api client)
  Dockerfile            (build Vite -> nginx)
  nginx.conf
docker-compose.yml      (2 services + Traefik labels + volume pour /data SQLite)
.env.example
README.md               (setup, run local, déploiement, comment ajouter une règle qualité)
Makefile                (optionnel : build, run, lint)
```

### Qualité de code
- Go idiomatique, erreurs gérées explicitement, pas de panique non maîtrisée.
- Le pipeline de synchro doit être **idempotent** et **transactionnel** (une synchro ratée ne corrompt pas le dernier snapshot valide : calcule dans une transaction / tables temporaires, bascule à la fin).
- Logs clairs sur chaque étape de la synchro (events pullés, issues détectées, durée).
- Le moteur de règles doit être **testé** (tests unitaires Go sur des events fabriqués couvrant chaque règle).
- README : comment lancer en local (sans Docker), avec Docker, et **comment ajouter une nouvelle règle qualité** (la partie la plus susceptible d'évoluer).

---

## 10. Déroulé demandé

1. D'abord, **propose un plan** : schéma SQLite définitif, liste des modules Go, contrat des endpoints, et l'arbre de fichiers. Attends ma validation.
2. Implémente le backend (client DHIS2 → store → quality → usage → sync → api → scheduler), avec tests du moteur de règles.
3. Implémente le front.
4. Dockerise (2 images) + docker-compose + Traefik + README.

Commence par le plan.