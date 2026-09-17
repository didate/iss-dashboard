# Plan — Carte sanitaire, palier 1

> Statut : **validé le 17/09/2026** — règle « type indéterminé » (R17) en `info`, codes de type et routes tels que proposés. Branche `feat/carte-sanitaire-p1`.
> Décisions verrouillées avec l'utilisateur (17 sept. 2026) :
> - Deux publics : **grand public** (sans login) et **planificateurs** MSHP/DRS/DPS (login existant).
> - Fiche publique **réduite (option B)** : identité, type, statut, localisation, rattachement, services offerts.
>   Pas de RH, pas d'équipements chiffrés, pas de nom/téléphone du responsable, pas de qualité.
> - Structures privées visibles du public.
> - Normes/écarts (palier 2) et accessibilité routière (palier 3) hors périmètre ici.

## 0. Ce que l'instance DHIS2 contient (vérifié le 17/09/2026, compte de service)

| Donnée | État réel | Conséquence |
|---|---|---|
| GPS structures recensées | 2 640 / 3 097 (85 %) — niv. 5 : 86 %, niv. 6 : 84 % | carte de points immédiate ; 457 manquantes = liste de travail |
| Polygones | niv. 1–3 : 100 %, niv. 4 : 342/378 | choroplèthe possible par **sous-préfecture** |
| Typologie | set `01 TOUTES LES STRUCTURES` classe 3 075/3 097 (PS 2 488, CS 527, HOP 40, CMC 16, CSA 14) ; 17 sans groupe, 5 dans plusieurs | groupes = source de vérité, préfixe du nom = repli |
| Sous-type hôpital | set `07 HÖPITAUX` (nationaux 3, régionaux 8, préfectoraux 26) | sous-typage HN / HR / HP |
| Public/privé | set `02 PUBLIC PRIVEE` : 113 privées = exactement le total ISS (46+29+24+14) ; 32 ni l'un ni l'autre | contrôle croisé → règle qualité |
| Population | data set `SIS_POPULATION` **mensuel** ; DE `RGPH Population, Feminin` `ksBi2JIApqW` + `Masculin` `oVYNP4fGnTo` ; tranches < 5 ans `hLcbHlNiRqP`, FAP `vRIHUMcfSnT`, grossesses attendues `IYeo7xNzEWy`, accouchements attendus `UQxlKligKNQ` | ratios /10 000 hab et ratios ciblés (SF / grossesses attendues) |

Piège identifié : la population étant saisie **mensuellement**, une agrégation annuelle via analytics donne des
valeurs fausses (`70 729,5` pour 2026). On prendra **la dernière période mensuelle renseignée** par org unit.

---

## 1. Principes de conception (et pourquoi)

1. **Deux espaces, un seul backend, une seule ingestion.**
   Un groupe de routes `/iss/api/public/*` est **toujours ouvert** et ne sert que des projections réduites,
   pré-calculées au sync. Tout le reste (`/iss/api/*` actuel) reste tel quel et passe derrière le JWT quand
   `DASHBOARD_PUBLIC=false`. Ainsi la frontière public/pro est **une frontière d'API**, pas un `if` dans le front :
   le front public ne peut physiquement pas afficher ce qu'il n'a pas.
2. **La typologie et la géographie deviennent des attributs de la structure**, calculés au sync et stockés,
   comme `district`/`region` le sont déjà. Les règles qualité et les agrégats les lisent ; personne ne les recalcule.
3. **Le référentiel DHIS2 est la source de vérité, l'app en signale les trous.** Groupe d'OU absent, GPS absent,
   statut juridique absent → issues qualité, donc visibles dans les écrans existants, filtrables par district,
   exportables. Pas de nouveau « module » : on réutilise le circuit qualité.
4. **Pré-calcul intégral, y compris le GeoJSON public** : 3 100 points × propriétés réduites ≈ 1 Mo, généré une
   fois par sync, servi avec ETag + gzip. Le seul calcul « à la volée » toléré est la recherche « autour de moi »
   (haversine sur 3 100 points en mémoire : < 1 ms).
5. **Les UID propres à l'instance vivent dans la configuration**, jamais dans le code (population, noms des group
   sets). Une autre instance DHIS2 = un autre `.env`, zéro recompilation.

---

## 2. Schéma SQLite — delta

Tables nouvelles (toutes vidées et recréées dans `PersistSyncData`, même transaction que le reste) :

```sql
-- Appartenance aux groupes d'OU (brut DHIS2, pour traçabilité et règles)
CREATE TABLE org_unit_group (
    group_uid   TEXT NOT NULL,
    group_name  TEXT NOT NULL,
    set_name    TEXT DEFAULT '',        -- nom du group set parent ('' si hors set)
    ou_uid      TEXT NOT NULL,
    PRIMARY KEY (group_uid, ou_uid)
);
CREATE INDEX idx_oug_ou ON org_unit_group(ou_uid);

-- Population par org unit (dernière période mensuelle renseignée)
CREATE TABLE population (
    ou_uid      TEXT NOT NULL,
    indicator   TEXT NOT NULL,          -- total | moins5 | fap | grossesses | accouchements
    period      TEXT NOT NULL,          -- ex. 202508
    value       REAL NOT NULL,
    PRIMARY KEY (ou_uid, indicator)
);

-- Couverture géographique et démographique par unité administrative (niveaux 3 et 4)
CREATE TABLE usage_geo (
    level               INTEGER NOT NULL,   -- 3 = préfecture/district, 4 = sous-préfecture
    ou_uid              TEXT NOT NULL,
    name                TEXT NOT NULL,
    parent_name         TEXT DEFAULT '',
    n_structures        INTEGER DEFAULT 0,
    n_gps               INTEGER DEFAULT 0,
    pct_gps             REAL,
    avg_score           REAL,
    population          REAL,               -- NULL si inconnue
    n_par_type          TEXT DEFAULT '{}',  -- JSON {"PS":12,"CS":3,...}
    PRIMARY KEY (level, ou_uid)
);

-- Ratios démographiques (format long : une ligne par dimension × clé × indicateur)
CREATE TABLE usage_couverture (
    dimension   TEXT NOT NULL,   -- global | region | district | sous_prefecture
    key         TEXT NOT NULL,
    label       TEXT DEFAULT '',
    indicator   TEXT NOT NULL,   -- structures | lits | medecins | sages_femmes | infirmiers | ...
    numerator   REAL DEFAULT 0,
    population  REAL,            -- NULL → ratio NULL
    ratio_10k   REAL,            -- numerator / population × 10 000
    PRIMARY KEY (dimension, key, indicator)
);

-- Projections pré-calculées servies telles quelles (JSON) ; une ligne par clé
CREATE TABLE snapshot_blob (
    key         TEXT PRIMARY KEY,   -- 'public_points_geojson', 'public_filters'
    etag        TEXT NOT NULL,
    json        BLOB NOT NULL,
    built_at    TEXT NOT NULL
);
```

Colonnes ajoutées (via `ALTER TABLE … ADD COLUMN`, comme déjà fait pour `geometry`) :

```sql
ALTER TABLE event ADD COLUMN sous_prefecture      TEXT DEFAULT '';
ALTER TABLE event ADD COLUMN sous_prefecture_uid  TEXT DEFAULT '';
ALTER TABLE event ADD COLUMN district_uid         TEXT DEFAULT '';
ALTER TABLE event ADD COLUMN type_code            TEXT DEFAULT '';   -- PS|CS|CSA|CMC|HP|HR|HN|CABINET|CLINIQUE|AUTRE_PRIVE|INDETERMINE
ALTER TABLE event ADD COLUMN type_source          TEXT DEFAULT '';   -- group|name|none
ALTER TABLE event ADD COLUMN lat                  REAL;              -- NULL si pas de GPS
ALTER TABLE event ADD COLUMN lng                  REAL;
```

Pourquoi sur `event` et non `org_unit` : l'event **est** la structure recensée dans tout le reste du code
(`district`, `region` y sont déjà) ; les requêtes de liste/fiche/filtres ne joignent rien de plus.
`org_unit.geometry` reste la source brute.

`usage_recensement` gagne une dimension `type` (clé = `type_code`) sans changement de schéma.
`quality_summary` idem (dimension `type`).

---

## 3. Ingestion — modifications de `RunSync`

Étape 1 (métadonnées), ajouts :
- `client.FetchOrgUnitGroups()` → `GET /api/organisationUnitGroups.json?fields=id,name,groupSets[name],organisationUnits[id]&paging=false`
  → `UpsertOrgUnitGroups`.
- `client.FetchPopulation(dx []PopSpec, levels []int)` → une requête analytics par niveau :
  `GET /api/analytics.json?dimension=dx:<uids>&dimension=ou:LEVEL-3&dimension=pe:LAST_12_MONTHS&skipMeta=true`
  ; en Go, pour chaque (ou, indicateur) on garde **la période max** ; `total = F + M` sommés sur la même période.
  Échec de cet appel = **avertissement** (log) et population vide, pas d'échec du sync : la carte sanitaire
  reste utilisable sans ratios.

Étape 3 (enrichissement), `enrichEvent` étendu :
- remonte aussi le niveau 4 → `sous_prefecture`, `sous_prefecture_uid`, et `district_uid` ;
- `lat/lng` depuis `org_unit.geometry` si `type == "Point"` ;
- `type_code` / `type_source` via `typologie.Resolve(ou, groups)` :
  1. groupe du set `01 TOUTES LES STRUCTURES` (si unique) → `PS|CS|CSA|CMC|HOP` ; si `HOP`, affiner via
     `07 HÖPITAUX` → `HN|HR|HP` (sinon `HP` par défaut) ; `type_source = group`
  2. sinon préfixe du nom (`PS`, `CS`, `CSR`, `CSU`, `CSA`, `CMC`, `CM`, `HP`, `HR`, `HN`, `CABINET`, `CLINIQUE`,
     `POLYCLINIQUE`, `CDT`…) → `type_source = name`
  3. sinon `INDETERMINE`, `type_source = none`.
  Pour le privé (set `02 PUBLIC PRIVEE` = Privé, ou `ISS_STATUT_STRUCT_DE = privée`) : sous-type par préfixe du
  nom si le set 01 ne l'a pas classé (`CABINET`, `CLINIQUE`, `AUTRE_PRIVE`).

Étape 4 (qualité), contexte enrichi : `OrgUnitGroups map[uid][]Group`, `HasGPS map[uid]bool`, `TypeOf map[uid]TypeInfo`.

Étape 6 (agrégats), nouveaux calculs dans `internal/usage/geo.go` :
- `ComputeGeo(events, orgUnits, population)` → `usage_geo` niveaux 3 et 4 ;
- `ComputeCouverture(events, usageRH, usageEquipements, population)` → `usage_couverture` :
  indicateurs `structures`, `lits` (racine équipement lits), `medecins`, `sages_femmes`, `infirmiers`, `ats`
  (mêmes codes de profil que `usage_rh`), pour `global | region | district`. `sous_prefecture` seulement pour
  `structures` (les RH/équipements ne sont pas agrégés au niveau 4 aujourd'hui — extension possible).
- `BuildPublicSnapshot(events, ctx)` → `snapshot_blob['public_points_geojson']` et `['public_filters']`.

Étape 7 (persist) : ajouter les nouvelles tables à la liste de purge/insertion, dans la même transaction.

---

## 4. Règles qualité nouvelles (`internal/quality/geo_typologie.go`)

> Numérotation : `R15`/`R16` étaient déjà émis par les règles WASH (le registre était désaligné des codes réels, corrigé au passage). Les nouvelles règles sont donc **R14, R17, R18**.

| Code | Nom | Sévérité | Condition |
|---|---|---|---|
| R14 | Coordonnées GPS manquantes | warning | `!ctx.HasGPS[evt.OrgUnitUID]` |
| R17 | Type de structure indéterminé | info | `type_source == none` ; ou `type_source == name` (« type déduit du nom, groupe DHIS2 absent ») ; ou OU dans **plusieurs** groupes du set 01 |
| R18 | Statut juridique incohérent | info | OU ni dans `Public` ni dans `Privé` du set 02 ; **ou** set 02 ≠ `ISS_STATUT_STRUCT_DE` (ex. groupe Public, formulaire « privée ») |

Score : inchangé (−5 par warning, −1 par info). Effet attendu sur le jeu actuel : ~457 R14 (warning), ~22 R17 (info), ~61 R18 (info : 32 hors groupes + 29 groupe ≠ formulaire).
Ces règles s'ajoutent au `Registry` comme les autres ; tests unitaires sur events fabriqués (GPS présent/absent,
groupe unique/multiple/absent, préfixe reconnu/inconnu, statut cohérent/incohérent).

---

## 5. API — contrat

### Public (`/iss/api/public`, jamais authentifié, cache HTTP agressif)

```
GET /public/points.geojson
    → FeatureCollection pré-calculée. properties: uid, name, type_code, type_label, statut_juridique,
      statut_op, district, region, sous_prefecture, services: [codes 'oui']
      Headers: ETag, Cache-Control: public, max-age=3600. 304 si If-None-Match.

GET /public/structures?search=&type=&service=&district=&region=&near=lat,lng&radius_km=&limit=50
    → [{uid, name, type_code, type_label, district, region, lat, lng, distance_km?, statut_op}]
      'near' : tri par distance haversine, calcul serveur.

GET /public/structure/:uid
    → { uid, name, type_code, type_label, statut_juridique, statut_op, region, district, sous_prefecture,
        lat, lng, services: [{code,label}], plateau: {labo, maternite, imagerie, urgences, pharmacie},
        recense_le }
      404 si inconnu. AUCUN champ RH/équipement/responsable/qualité.

GET /public/filters
    → { types: [{code,label,n}], services: [{code,label}], regions: [...], districts: [{name, region}] }

GET /public/summary
    → { n_structures, n_par_type: {...}, pct_gps, derniere_synchro }
```

### Pro (groupe existant, derrière `DashboardAuth`)

```
GET /geo/coverage?level=3|4&region=
    → lignes usage_geo (n_structures, n_gps, pct_gps, avg_score, population, n_par_type)
GET /geo/missing?district=&type=&page=&pageSize=
    → structures sans GPS (= issues R14), avec district, sous-préfecture, type, date recensement
GET /geo/missing.csv?district=          → export CSV pour les équipes terrain
GET /map/districts?level=3|4&layer=      → existant, étendu : level=4 renvoie les polygones sous-préfectoraux
                                           avec les couches calculables au niveau 4 (structures, gps, qualité, population)
GET /usage/couverture?by=region|district|sous_prefecture&indicator=
    → lignes usage_couverture
GET /usage/recensement?by=type           → existant, nouvelle dimension
GET /quality/summary?by=type             → existant, nouvelle dimension
GET /structures?type=&gps=oui|non        → existant, deux filtres de plus
GET /meta/filters                        → existant, + types
```

---

## 6. Frontend

### Espace public (nouveau, sans sidebar, en-tête léger)

| Route | Page | Contenu |
|---|---|---|
| `/` | `PublicMap` | carte plein écran (Leaflet + `leaflet.markercluster`), barre de recherche, filtres type/service/région, bouton « autour de moi » (géoloc navigateur → `near=`), liste latérale des résultats. Clic marqueur → panneau résumé + lien fiche. |
| `/fs/:uid` | `PublicFiche` | fiche réduite, mini-carte, services en badges, plateau en pictos, bouton « copier le lien », QR code (`qrcode.react`). |
| `/a-propos` | `About` | source, date de synchro, limites (à la manière de la note explicative guinéenne, mais en une colonne courte). |

L'accès pro se fait via un lien « Espace planification » dans l'en-tête → `/login`.

### Espace pro (existant)

- Le tableau de bord actuel passe de `/` à **`/tableau-de-bord`** (redirection depuis les anciens liens `/` → seulement
  si connecté, sinon carte publique). Les autres routes ne bougent pas.
- `MapView` : sélecteur **Niveau : préfecture | sous-préfecture** ; couche **Points** (mêmes marqueurs que le public,
  mais colorés par score qualité, clic → `StructureDetail`) ; couche **Couverture GPS**.
- Nouvelle page **`/geolocalisation`** : KPI (% GPS national), tableau par district/sous-préfecture, liste des
  structures sans GPS avec filtres et export CSV.
- `Usage` : onglet **Couverture** (ratios /10 000 hab, tableau + barres) ; recensement par **type**.
- `Structures` : colonne type, filtre type, filtre GPS.
- `StructureDetail` : type, sous-préfecture, coordonnées, lien vers la fiche publique.

Contraintes inchangées : aucun calcul métier dans le front, filtres en query params, recharts, pas de localStorage
métier. Le clustering de marqueurs est de l'affichage, pas du métier.

---

## 7. Configuration

```
# Population (UIDs propres à l'instance) — indicateur:uid[+uid]...
DHIS2_POPULATION_DX=total:ksBi2JIApqW+oVYNP4fGnTo,moins5:hLcbHlNiRqP,fap:vRIHUMcfSnT,grossesses:IYeo7xNzEWy,accouchements:UQxlKligKNQ
DHIS2_POPULATION_LEVELS=3,4
# Typologie
DHIS2_TYPOLOGY_GROUPSET=01 TOUTES LES STRUCTURES
DHIS2_HOSPITAL_GROUPSET=07 HÖPITAUX
DHIS2_OWNERSHIP_GROUPSET=02 PUBLIC PRIVEE
# Prod : espace pro derrière login, espace public toujours ouvert
DASHBOARD_PUBLIC=false
```

Le mapping *nom de groupe → code type* (`06 PS → PS`, `Hôpitaux nationaux → HN`…) vit dans
`internal/typologie/mapping.go` avec le mapping des préfixes de nom ; c'est du métier stable, testé.

---

## 8. Arbre des fichiers

```
backend/internal/
  dhis2/client.go              + FetchOrgUnitGroups, FetchPopulation (analytics)
  models/models.go             + OrgUnitGroup, PopulationRow, UsageGeoRow, CouvertureRow, PublicFeature…
  typologie/                   NOUVEAU : mapping.go (tables), resolve.go (Resolve), typologie_test.go
  store/migrations.go          + tables §2, + ALTER event
  store/store.go               + UpsertOrgUnitGroups, purge/insert nouvelles tables, snapshot_blob
  store/queries.go             + GetPublicStructure, SearchPublicStructures, GetGeoCoverage, GetMissingGPS,
                                 GetCouverture ; GetMapData(level)
  quality/context.go           + OrgUnitGroups, HasGPS, TypeOf
  quality/geo_typologie.go     NOUVEAU : R14, R17, R18
  quality/quality_test.go      + tests R14, R17, R18
  usage/geo.go                 NOUVEAU : ComputeGeo, ComputeCouverture
  usage/public_snapshot.go     NOUVEAU : BuildPublicSnapshot (GeoJSON + filtres)
  usage/recensement.go         + dimension type
  sync/sync.go                 + étapes §3 ; enrichEvent étendu
  api/public_handlers.go       NOUVEAU : 5 endpoints publics, ETag
  api/geo_handlers.go          NOUVEAU : coverage, missing, missing.csv, couverture
  api/router.go                + groupe /public (sans auth), + routes pro
  config/config.go             + variables §7 (parsing de DHIS2_POPULATION_DX)

frontend/src/
  pages/public/PublicMap.tsx, PublicFiche.tsx, About.tsx      NOUVEAU
  components/PublicLayout.tsx                                 NOUVEAU
  components/map/PointsLayer.tsx (partagé public/pro)         NOUVEAU
  pages/Geolocalisation.tsx                                   NOUVEAU
  pages/MapView.tsx, Usage.tsx, Structures.tsx, StructureDetail.tsx, Dashboard.tsx   modifiés
  api/client.ts, types/index.ts                               + endpoints/types
  App.tsx, components/Layout.tsx                              routes, lien espace public ↔ pro
  package.json                                                + leaflet.markercluster, qrcode.react

README.md   + section « Carte sanitaire » : espaces public/pro, variables, comment étendre la typologie
```

---

## 9. Ordre de livraison (chaque lot = commit(s) + démo, validation avant le suivant)

| Lot | Contenu | Vérification |
|---|---|---|
| **A — Données** | groupes d'OU, typologie, GPS→lat/lng, sous-préfecture, population, `usage_geo`, `usage_couverture`, R14/R17/R18, tests | `go test ./...` ; sync sur l'instance ; requêtes SQLite : distribution des types, % GPS, ratios de quelques districts comparés à la main |
| **B — API** | endpoints publics + pro, snapshot GeoJSON, ETag | `curl` sur chaque endpoint, taille et temps de `/public/points.geojson` |
| **C — Espace public** | PublicLayout, PublicMap, PublicFiche, About | parcours : recherche → marqueur → fiche → lien partagé ; mobile |
| **D — Espace pro** | MapView niveau 4 + points, page Géolocalisation, onglet Couverture, filtres type/GPS | parcours DPS : « mes structures sans GPS » → export CSV |
| **E — Doc & conf** | README, `.env.example`, `docker-compose` (rien à changer côté images) | relecture |

---

## 10. Points ouverts / risques

1. **`DASHBOARD_PUBLIC`** change de sens en pratique : aujourd'hui `true` en local. En prod il faudra le passer à
   `false` pour que « public » = uniquement `/public/*`. Documenté dans le README ; aucun changement de code
   côté middleware.
2. **Population au niveau 4** : à vérifier que `SIS_POPULATION` est bien saisi au niveau sous-préfecture (sinon
   ratios niveau 4 = NULL, prévu).
3. **Confidentialité de la position** : les coordonnées d'une structure sont publiques par nature (c'est le but
   d'une carte sanitaire) ; rien d'autre de la fiche ne l'est. Le nom du responsable et son téléphone n'entrent
   dans aucune projection publique — à re-vérifier lors de la revue du lot B.
4. **Doublons d'events sur une même OU** (R12) : la fiche publique et le GeoJSON prennent **l'event le plus récent**
   par org unit ; les compteurs publics comptent des **org units**, pas des events.
5. **17 structures sans groupe et 5 en plusieurs groupes** : l'app les signalera (R17) ; le nettoyage se fait
   dans DHIS2, pas dans l'app.
6. **Palier 2 (normes)** s'appuiera sur `type_code` : le choix des codes (`PS|CS|CSA|CMC|HP|HR|HN|…`) est donc
   structurant — à valider maintenant.
