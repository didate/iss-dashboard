# Plan — Dashboard configurable + filtres « à la Superset »

> Statut : **Phases 1-3 livrées** (branche `feat/dashboard-configurable`) · Phase 4 (polish/tests/doc) à faire.
> - ✅ Phase 1 — moteur backend (snapshot, filtres live croisés/valeurs, mesures, endpoints) — commit `8b61b23`
> - ✅ Phase 2 — front config-driven : route `/dashboard-v2`, grille, 5 widgets, barre de filtres — `db35ece`
> - ✅ Phase 3 — builder admin : édition drag/resize, menus guidés, sauvegarde ; catalogue backend — `975b301`
> - ⬜ Phase 4 — tests moteur de filtres, états vide/chargement soignés, doc « ajouter widget/mesure/filtre »
> Décisions verrouillées avec l'utilisateur :
> - **Filtres** : croisés + sur valeurs (à la Superset) → moteur d'agrégation live.
> - **Portée de la config** : **globale**, éditée par l'admin, vue par tous.
> - **Mise en page** : **grille drag & drop** (react-grid-layout).
> - **Création d'analyses** : **escalier A→B** — widgets paramétrés d'abord (Modèle A),
>   config structurée en `{mesure, dimension, sélecteur}` pour brancher le moteur générique
>   (Modèle B) ensuite sans réécriture.

---

## 1. Vision

Le Dashboard devient une **grille de widgets** définie par une configuration que l'admin édite,
et une **barre de filtres à gauche** applique des filtres croisés qui **recalculent tous les widgets**
sur le sous-ensemble de structures sélectionné.

Exemple : `Région = Kankan  +  Statut = Privé  +  Labo = oui`
→ TOUS les widgets (KPI, carte, top 5…) se recalculent sur ce sous-ensemble.

### Écart assumé au brief
Le brief impose « lecture = tables pré-calculées ». Ici, **quand des filtres sont actifs**, on agrège
**à la volée**. On garde le pré-calcul comme défaut rapide (aucun filtre).
La règle « le front ne calcule rien » reste **respectée** : tout le calcul reste backend.

---

## 2. Constat technique (issu de l'exploration du code)

- Les fonctions `Compute*` (`internal/usage/*.go`, `internal/sync/sync.go`) travaillent **en mémoire**
  sur une slice `[]Event`, avec regroupement par map, et produisent déjà toutes les dimensions
  (global / district / région / statut).
- `event` porte les colonnes `district`, `region`, `status`. Le **statut juridique** et les **services**
  ne sont pas des colonnes : ce sont des `event_value` (`de_code` = `ISS_STATUT_STRUCT_DE`,
  `ISS_STATUT_PUB_DE`, `ISS_STATUT_PRIV_DE`, DE de section `ISS_SVC`/`ISS_LAB`, etc.).
- Conséquence clé : **filtrer = filtrer la slice `[]Event` en amont**, puis appeler les `Compute*`
  **inchangées**. Pas de réécriture des agrégations, pas de changement de schéma, pas d'index requis
  pour le chemin filtré (il est en mémoire).
- Seul le chemin **géo/carte** (`GetMapData`) lit les tables pré-calculées + la géométrie et demandera
  un **chemin filtré dédié** (recalcul par district depuis les events filtrés puis fusion avec la géométrie).

---

## 2bis. Modèle de création des analyses (widget authoring) — escalier A→B

Une **analyse** = une visualisation configurée. L'admin ne code rien : il ajoute un widget et règle
ses paramètres. La config d'un widget est structurée dès le départ en **`{ viz, mesure, dimension, sélecteur }`**
pour que le Modèle A (palette paramétrée) et le Modèle B (builder générique) partagent la même forme.

### Modèle A — widgets paramétrés (livré en premier)
Palette de **types de visualisation** (KPI, Carte, Barres, Donut, Table), chacun avec des paramètres
prévus. Réutilise le code existant (dont la carte Densité RH par profil déjà livrée).

**Exemple — « carte densité ATS » :**
```
+ Ajouter une analyse
  → Visualisation : Carte
  → Mesure        : Densité RH
  → Sélecteur     : profil = ATS        ◄── sélecteur RH-par-profil déjà en place
  → Dimension     : District  (imposée par la viz Carte)
  → Titre         : "Densité ATS par district"
```
Stocké :
```json
{ "id": "w_ats", "type": "widget", "viz": "map", "title": "Densité ATS par district",
  "measure": "rh_density", "dimension": "district",
  "selector": { "profil": "ATS_CODE" }, "grid": { "x": 0, "y": 0, "w": 6, "h": 4 } }
```

### Modèle B — moteur de mesures générique (extension ultérieure)
Le builder laisse choisir **Visualisation × Mesure × Dimension × Sélecteur** librement → catalogue
d'analyses infini (« ATS en barres par région », « table ATS × district », « KPI total ATS »…).
Techniquement = un **registre de mesures** backend paramétrable :

| Mesure | Paramètre (sélecteur) | Calcul par clé de dimension |
|---|---|---|
| `count_structures` | — | nb d'events |
| `rh_effectif` | `profil` | Σ effectif du profil |
| `rh_density` | `profil` | Σ effectif profil / nb structures |
| `equip_total` / `equip_fonc` | `equip_root` | Σ total / Σ fonctionnel |
| `service_pct` | `service_code` | % events où service = `oui` |
| `score_avg` | — | score qualité moyen |

`dimension ∈ { global, district, region, statut_juridique, service, profil_rh, … }`.
Le Modèle B **réutilise** `applyFilter` + les `Compute*` : une mesure = une projection d'un `Compute*`
existant. Passer de A à B = exposer ces mesures dans le builder, **pas** réécrire le calcul.

### Ligne de partage
- La carte ATS (et ~90 % des besoins réels) est couverte **par le Modèle A**, quasi gratuitement.
- Le Modèle B ne sert que pour les analyses **non prévues** ; il s'ajoute sans casser la config A
  car la forme `{ viz, mesure, dimension, sélecteur }` est commune.

---

## 3. Architecture — 3 briques

### Brique 1 — Moteur de filtrage live (backend) — *le cœur*

**Principe « matching-uid-set once »** : un `FilterSet` → on calcule une fois l'ensemble d'`event`
qui matchent → chaque agrégat tourne restreint à ce sous-ensemble.

- **Modèle de filtre**
  - *Structurels* (colonnes de `event`) : région, district, statut opérationnel.
  - *Sur valeurs* (via `event_value` par `de_code`) : statut juridique, service X = `oui`,
    type de structure, plateau technique…
- **Chemin live raffiné**
  1. Après chaque `RunSync`, mettre en **cache mémoire** le snapshot `{ events []Event, ctx *Context }`
     (au démarrage, reconstruire depuis SQLite si le cache est vide). Trivial à quelques milliers d'events.
  2. `POST /api/dashboard/data` : parse les filtres → `filtered := applyFilter(snapshot.events, filters)`
     → rejoue les **mêmes** `Compute*` sur le sous-ensemble → façonne le payload de chaque widget demandé
     → renvoie tout en **un seul aller-retour**.
  3. **Sans filtre** = servir les tables pré-calculées existantes (rapide).
     **Avec filtre** = chemin live. Chiffres cohérents (même code de calcul).
- **Ce qui est réellement du code neuf backend**
  - parsing / matching des filtres (`applyFilter` sur `[]Event`),
  - façonnage des payloads par widget (registry côté backend),
  - handler batch `POST /api/dashboard/data`,
  - stockage de la config,
  - chemin filtré de la carte.
- **Réutilisé tel quel** : toutes les fonctions d'agrégation.

### Brique 2 — Stockage de la configuration (backend + admin)

- Table `dashboard_config(id=1, config_json TEXT, updated_at, updated_by)` — **une seule ligne globale**.
- `config_json` :
  ```json
  {
    "version": 1,
    "widgets": [
      { "id": "w1", "viz": "kpi", "title": "Structures analysées",
        "measure": "count_structures", "dimension": "global", "selector": {},
        "grid": { "x": 0, "y": 0, "w": 3, "h": 2 } },
      { "id": "w_ats", "viz": "map", "title": "Densité ATS par district",
        "measure": "rh_density", "dimension": "district", "selector": { "profil": "ATS_CODE" },
        "grid": { "x": 3, "y": 0, "w": 6, "h": 4 } }
    ],
    "enabledFilters": ["region", "district", "statut_juridique", "statut_operationnel", "service"]
  }
  ```
  Forme commune `{ viz, measure, dimension, selector }` (cf. §2bis) : le Modèle A remplit ces champs
  via des sélecteurs guidés, le Modèle B via un builder libre — même schéma, aucune migration.
- **Versionnée** (`version`) pour ne pas casser une config sauvée quand on ajoutera des widgets.
- Endpoints :
  - `GET  /api/dashboard/config` — lecture (public/selon `DASHBOARD_PUBLIC`)
  - `PUT  /api/admin/dashboard/config` — protégé admin
  - `GET  /api/meta/filter-defs` — catalogue des filtres + listes d'options
    (régions, districts, statuts, services…)

### Brique 3 — Frontend config-driven + grille + filtres

- **Registry de widgets** : `type → composant React`. On **découpe le Dashboard actuel** en widgets discrets
  (chaque KPI, la mini-carte, le donut statut, Top/Bottom, tableaux…).
- **Page Dashboard** : charge la config → rend une grille **react-grid-layout** → chaque widget reçoit
  `(params, filterState, data)`, `data` venant du batch endpoint. Re-fetch au changement de filtre
  (avec debounce).
- **Barre de filtres (gauche)** : construite depuis `filter-defs` + `enabledFilters` ; état stocké dans
  l'**URL** (réutilise `useUrlState`) → vues partageables / bookmarkables.
- **Builder admin** : palette de widgets → glisser sur la grille, redimensionner, éditer titre/params,
  choisir les filtres activés, **Enregistrer** (`PUT`). Mode édition réservé au rôle `admin`.

---

## 4. Contrat d'API

```
GET  /api/dashboard/config           -> config globale (widgets + layout + filtres activés)
PUT  /api/admin/dashboard/config     -> enregistre la config (admin)
GET  /api/meta/filter-defs           -> dimensions filtrables + listes d'options
POST /api/dashboard/data             -> { widgets:[...], filters:{...} } => { [widgetId]: payload }
```

---

## 5. Catalogues v1

### Widgets (issus du refactor de l'existant)
- **KPI** : structures, taux de rapportage, score qualité, % opérationnel, médecins/structure,
  effectif RH, % énergie, % eau aux points critiques.
- **Carte thématique** (rapportage / score / services / équipements / WASH / densité RH).
- **Donut** statut juridique.
- **Top / Bottom districts** (rapportage & score qualité).
- **Tableau services** ; **équipements chaîne du froid**.

Chaque widget déclare : sa source de données + les filtres qu'il honore.

### Filtres
Région · District · Statut juridique · Statut opérationnel · Type de structure ·
Service (= `oui`) · Plateau technique.
Chaque filtre → un builder de prédicat sur `[]Event` (colonne ou `event_value`).

---

## 6. Découpage en phases

| Phase | Contenu | Résultat visible |
|---|---|---|
| **1 — Moteur backend** | cache snapshot + `applyFilter` + runner de lecture des `Compute*` ; `POST /dashboard/data` ; `GET /meta/filter-defs` ; chemin filtré de la carte | API filtrable, config par défaut en dur |
| **2 — Front config-driven** | registry widgets + rendu depuis config + barre de filtres (lecture seule de la config) | Dashboard filtrable à la Superset |
| **3 — Builder admin** | grille drag/resize éditable, palette, choix des filtres, sauvegarde | l'admin compose le dashboard |
| **4 — Polish** | états vide/chargement, vues partageables, perf/index si besoin, **tests du moteur de filtres**, doc « ajouter un widget / un filtre » | robustesse + doc |

### Découpage proposé de la Phase 1
1. cache snapshot + `applyFilter` + runner de lecture des `Compute*`,
2. `POST /api/dashboard/data` avec 2–3 widgets pilotes (KPI + un tableau + la carte),
3. `dashboard_config` + `GET/PUT config` + `GET /meta/filter-defs`.

---

## 7. Risques / points d'attention

- **Perf** : agrégation live à chaque filtre → mitigée par le cache mémoire des events, le calcul
  de l'uid-set une seule fois, le batch unique, et un debounce côté front. À quelques milliers d'events,
  largement sous 100 ms. (Index `event_value(de_code, event_uid)` / `event(district, region, status)`
  seulement si on repasse un jour par SQL.)
- **Écart au « pré-calcul pur »** : assumé et cadré (live uniquement sous filtres). **À valider.**
- **Dépendance** `react-grid-layout` au front.
- **Versioning de config** dès le départ pour l'évolutivité.
- **Carte en mode filtré** : seul chemin nécessitant un recalcul dédié (par district) + fusion géométrie.
- **Cache mémoire & déploiement** : OK tant que le backend est mono-conteneur (cas actuel).
  À revoir si multi-réplicas un jour.

---

## 8. Fichiers concernés (indicatif)

**Backend**
- `internal/store/migrations.go` — table `dashboard_config`.
- `internal/store/queries.go` — get/put config ; chemin filtré carte.
- `internal/sync/sync.go` — peupler le cache snapshot après `RunSync`.
- `internal/dashboard/` (nouveau) — `applyFilter`, runner de lecture, registry backend des widgets.
- `internal/api/` — handlers `dashboard_config`, `dashboard_data`, `filter-defs` + routes.

**Frontend**
- `src/pages/Dashboard.tsx` — refactor en rendu piloté par config + grille.
- `src/components/widgets/` (nouveau) — registry `type → composant`.
- `src/components/FilterSidebar.tsx` (nouveau).
- `src/pages/Admin.tsx` (ou nouvelle page builder) — édition de la config.
- `src/api/client.ts` — `getDashboardConfig`, `putDashboardConfig`, `getFilterDefs`, `postDashboardData`.
- `src/types/index.ts` — types config / widget / filtre.

---

## 9. Questions de validation ouvertes
1. Feu vert pour démarrer la **Phase 1 (backend)** avec le découpage proposé ?
2. OK pour l'écart assumé au « pré-calcul pur » (agrégation live sous filtres) ?
3. Mode édition réservé au rôle `admin` existant : confirmé ?
