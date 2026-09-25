# Plan — Personnel de l'État (DRH/CNPS) dans l'espace planification

> Statut : **validé le 25/09/2026**. Décisions : (1) **agrégats seulement** — aucune ligne par agent en base,
> les agrégats sont croisés zone × profession × tranche d'âge × sexe pour couvrir les questions prévisibles ;
> (2) départ à la retraite à **60 ans**, paramétrable (`DRH_AGE_RETRAITE`) faute de référence officielle ;
> (3) import d'un **CSV normalisé** documenté (`docs/drh-format.md`), converti depuis le .xlsx annuel de la DRH
> par un script fourni ; (4) rien dans l'espace public ; (5) les agents de bureaux de district comptent dans la
> densité de leur district ; (6) tranches d'âge **quinquennales recalculées** depuis l'année de naissance.
> Source : fichier DRH/CNPS 2026 reçu du MSHP, 10 162 agents de la fonction publique santé.
> Travail déjà fait : anonymisation (retrait du matricule), table de correspondances DRH → ISS
> (`data/DRH - correspondances structures.csv`, 32 entrées validées avec l'utilisateur).

## 0. Ce que l'analyse a établi

**Nature** : effectif **payé par l'État**, à ne pas confondre avec l'effectif **présent** déclaré par les
structures dans ISS (45 080 agents). Le fichier DRH en couvre donc environ un quart.

| Constat | Chiffre |
|---|---|
| Agents | 10 162 (99 % fonctionnaires) |
| Affectés à une structure de soins | 6 500 (64 %) — 388 structures |
| Bureaux de district (DPS/DCS/IRS) | 2 721 (26,8 %) |
| Administrations centrales et programmes | 875 (8,6 %) |
| Indéterminés | 66 (0,6 %) |
| Densité nationale | **5,56 agents de l'État pour 10 000 hab.** |
| Conakry | 26,6 % des agents pour 14 % de la population |
| Kaloum | 121 agents /10 000 hab. — **65 fois** Kérouané (1,86) |
| Âge médian | 43 ans ; **14,2 % atteignent 60 ans d'ici 5 ans**, 24,9 % d'ici 10 ans |
| Médecins partant d'ici 5 ans | **22,1 %** (399 sur 1 803) |
| Couverture du réseau | 388/3 097 structures (12,5 %) : 100 % des HN, 88 % des HR, 83 % des HP, 57 % des CS, **1,3 % des PS** |
| Féminisation | 59 % ; sages-femmes 99,9 %, médecins 31,3 % |

**Comparaison DRH / ISS** (national) : médecins ×1,3, ATS ×3,7, sages-femmes ×3,9, infirmiers ×4,9.
Le rapport très variable est en soi un indicateur : il mesure la part de personnel hors fonction publique.

**Incohérences détectées** — des districts où la DRH paie **plus** de médecins que les structures n'en
déclarent : Coyah (78 vs 22), Kindia (86 vs 57), Dubréka (51 vs 31), Kissidougou (37 vs 19), Dabola (22 vs 5).
Soit un défaut de déclaration ISS, soit des agents affectés mais absents. C'est la valeur ajoutée du
croisement : ni l'une ni l'autre source ne pouvait le révéler seule.

---

## 1. Principes

1. **Une source de plus, pas une source de vérité.** Le fichier DRH ne remplace pas ISS : il s'affiche
   **à côté**, et l'écart est l'information. Aucun agrégat ISS existant n'est modifié.
2. **Agrégats seulement, aucune donnée nominative en base.** Le matricule est déjà retiré ; à l'ingestion
   on ne conserve ni nom, ni date de naissance exacte — seulement la **tranche d'âge quinquennale**.
   Aucune exposition dans l'espace public (la frontière du palier 1 reste intacte).
3. **Millésimes.** Le fichier est annuel : chaque import crée un millésime (`drh_import`), le dernier
   actif sert les écrans, les précédents restent pour comparer dans le temps.
4. **La correspondance des structures est une donnée éditable**, comme le référentiel de normes :
   la table DRH → ISS s'importe, se complète et se corrige sans toucher au code.
5. **Ce qui n'est pas rattachable reste visible** (bureaux de district, administrations centrales) :
   c'est 35 % des agents, les masquer fausserait toute lecture.

---

## 2. Modèle de données

```sql
-- Un import (millésime) du fichier DRH
CREATE TABLE drh_import (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    label         TEXT NOT NULL,        -- « DRH/CNPS 2026 »
    annee         INTEGER NOT NULL,
    status        TEXT NOT NULL DEFAULT 'active',  -- active | archived
    n_agents      INTEGER, n_rattaches INTEGER, n_structures INTEGER,
    imported_at   TEXT NOT NULL, imported_by TEXT DEFAULT '',
    source_file   TEXT DEFAULT ''
);

-- PAS de table par agent (décision 1) : seuls des agrégats sont persistés.
-- Le croisement zone × profession × tranche d'âge × sexe est assez fin pour les écrans prévus,
-- et une cellule ne peut jamais désigner une personne si l'on n'affiche pas les effectifs < 1.

-- Correspondances libellé DRH → structure ISS (éditable, importable)
CREATE TABLE drh_correspondance (
    libelle_norm  TEXT PRIMARY KEY,     -- libellé normalisé (clé de rapprochement)
    libelle_drh   TEXT NOT NULL,
    org_unit_uid  TEXT DEFAULT '',
    statut        TEXT NOT NULL,        -- ok | bureau_district | non_rattache | a_trancher
    district      TEXT DEFAULT ''
);

-- Agrégats pré-calculés (servis tels quels)
CREATE TABLE drh_effectif (              -- dimension : global|region|district|sous_prefecture|structure|type
    import_id INTEGER, dimension TEXT, key TEXT, label TEXT,
    categorie TEXT,                      -- '' = toutes professions
    n_agents INTEGER, n_femmes INTEGER,
    n_structure INTEGER, n_bureau INTEGER, n_administration INTEGER,
    n_depart_5ans INTEGER, n_depart_10ans INTEGER,
    population REAL, ratio_10k REAL,
    PRIMARY KEY (import_id, dimension, key, categorie)
);

-- Pyramide : effectif par tranche quinquennale (croisé sexe), pour les départs et la structure d'âge
CREATE TABLE drh_pyramide (
    import_id INTEGER, dimension TEXT, key TEXT, categorie TEXT,
    tranche TEXT,                        -- '<25','25-29',…,'60+'
    n_agents INTEGER, n_femmes INTEGER,
    PRIMARY KEY (import_id, dimension, key, categorie, tranche)
);

-- Comparaison DRH vs ISS (une ligne par zone × profil)
CREATE TABLE drh_comparaison (
    import_id INTEGER, dimension TEXT, key TEXT, label TEXT, categorie TEXT,
    n_drh INTEGER, n_iss REAL, ecart REAL, ratio REAL,   -- ratio = ISS / DRH
    PRIMARY KEY (import_id, dimension, key, categorie)
);
```

---

## 3. Ingestion

Nouveau package `internal/drh` :

```
ParseCSV(r io.Reader) -> []AgentRow        CSV normalisé (docs/drh-format.md), validation colonne par colonne
Resolve(rows, correspondances, orgUnits)   rattachement : table → nom exact → type + nom propre dans le district
                                           → bureau de district → administration centrale
Aggregate(rows, population, issRH)         drh_effectif + drh_comparaison
```

Le **rattachement** reprend exactement l'algorithme mis au point pendant l'analyse (validé à 99,8 %) :
correspondance explicite, puis nom exact, puis appariement par **type de structure + nom propre** dans le
district de l'agent, avec déduction quand le district n'a qu'un seul établissement du type visé.

Déclenchement : **écran Admin → onglet « Personnel (DRH) »**, dépôt du fichier, rapport d'import
(agents lus, rattachés, libellés non reconnus avec leur effectif), puis activation du millésime.
Pas de lien avec la synchro DHIS2 : le fichier arrive une fois par an, par courriel.

Le recalcul des agrégats est refait à chaque synchro ISS (la comparaison dépend des effectifs ISS et de la
population) — même mécanique que `RecomputeConformite`.

---

## 4. API

**Admin** `POST /admin/drh/import` (multipart), `GET /admin/drh/imports`, `POST /admin/drh/imports/:id/activate`,
`DELETE /admin/drh/imports/:id`, `GET|PUT /admin/drh/correspondances`, `GET /admin/drh/correspondances/export.csv`,
`GET /admin/drh/non-reconnus` (liste à renvoyer à la DRH).

**Lecture (espace planification)**
```
GET /drh/summary                                   → millésime actif, effectifs, répartition, densité
GET /drh/effectifs?by=region|district|sous_prefecture|type|categorie&categorie=
GET /drh/pyramide?district=&categorie=             → tranches d'âge, départs 5 et 10 ans
GET /drh/comparaison?by=district&categorie=        → DRH vs ISS, écart, ratio
GET /drh/structures?district=&sort=effectif        → effectif par structure (+ structures sans agent)
GET /map/geo?level=                                → + drh_ratio_10k, drh_depart_5ans_pct
```

---

## 5. Front — page « Personnel de l'État » (`/personnel`)

Une page, cinq blocs (l'ordre suit la lecture d'un planificateur) :

1. **KPI** : agents, densité /10 000 hab., % en structure de soins, % de départs à 5 ans, millésime.
2. **Répartition** : par catégorie professionnelle, par affectation (structure / district / centrale),
   par sexe, par catégorie hiérarchique ; barres + tableau exportable.
3. **Carte** : densité d'agents /10 000 hab. par district et sous-préfecture (nouvelle métrique dans
   `ProGeoMap`), et une seconde métrique « % de départs d'ici 5 ans ».
4. **Pyramide des âges et départs** : histogramme par tranche, tableau des districts et professions les
   plus exposés — la sortie la plus actionnable du fichier.
5. **Comparaison DRH ↔ ISS** : tableau par district et par profil (DRH payés, ISS déclarés, écart, ratio),
   tri par écart, avec **mise en évidence des incohérences** (DRH > ISS). Note de lecture expliquant que
   l'écart normal traduit le personnel hors fonction publique.

Ajouts ailleurs : onglet **RH** d'Utilisation → colonne « dont fonctionnaires (DRH) » ; **détail d'une
structure** → bloc « Personnel de l'État affecté » (effectif par catégorie) ; **vue d'ensemble** → une
carte KPI « agents de l'État /10 000 hab. ».

**Rien dans l'espace public** (décision à confirmer : on pourrait plus tard publier la seule densité par
district, qui n'est pas sensible).

---

## 6. Arbre des fichiers

```
backend/internal/drh/            NOUVEAU : parse.go, resolve.go, aggregate.go, drh_test.go
backend/internal/store/drh_store.go   NOUVEAU : imports, agents, correspondances, agrégats, lectures
backend/internal/api/drh_handlers.go  NOUVEAU : admin + lecture
backend/internal/store/migrations.go  + tables §2
backend/internal/sync/sync.go         + recalcul des agrégats DRH en fin de pipeline
frontend/src/pages/Personnel.tsx            NOUVEAU
frontend/src/pages/admin/DrhImport.tsx      NOUVEAU
frontend/src/pages/{Admin,Usage,StructureDetail,Dashboard}.tsx, components/map/ProGeoMap.tsx  modifiés
docs/drh-format.md               NOUVEAU : colonnes attendues du fichier DRH, à transmettre à la DRH
README.md                        section « Personnel de l'État (DRH) »
```

---

## 7. Lots

| Lot | Contenu | Vérification |
|---|---|---|
| **A — Ingestion** ✅ | tables, parseur CSV, rattachement, correspondances, import admin, tests | **livré le 25/09/2026** — import du fichier réel : 10 162 agents, **99,4 % catégorisés** (6 500 en structure sur 388 structures, 2 721 en bureau, 875 en centrale, 66 non rattachés), 240 ms |
| **B — Agrégats & API** ✅ | rollups, densités, `drh_comparaison`, recalcul au sync, endpoints de lecture, métriques carte | **livré le 25/09/2026** — chiffres conformes à l'analyse : 14,1 % de départs à 5 ans, ATS ×3,68, infirmiers ×4,89, sages-femmes ×3,98, Kérouané 1,86 /10 000 hab. |
| **C — Front** ✅ | page Personnel, onglet admin, carte, détail structure, KPI | **livré le 25/09/2026** — parcours vérifié : import → page `/personnel` → filtre district → comparaison, pyramide, structures ; carte thématique ; bloc sur la fiche structure |
| **D — Doc** ✅ | `docs/drh-format.md`, README | **livré** au fil des lots : format CSV et convertisseur (A), section README « Personnel de l'État » — confidentialité, import, rattachement, agrégats, lecture de la comparaison, comment étendre (B), où cela se lit dans l'interface et écarts assumés (C) |

---

### Lot A — ce qui a été livré

`backend/internal/drh/` (parse, professions, resolve, aggregate, correspondances, seed embarqué, tests),
`store/drh_store.go` + tables `drh_import`/`drh_correspondance`/`drh_effectif`/`drh_pyramide`/`drh_comparaison`/`drh_non_reconnu`,
`sync/drh.go` (`RunDrhImport`), `api/drh_handlers.go` + routes admin, `frontend/src/pages/admin/DrhImport.tsx`
(onglet « Personnel (DRH) »), `scripts/drh_xlsx_to_csv.py`, `docs/drh-format.md`, `cmd/drhcheck` (essai à blanc),
section README, `DRH_AGE_RETRAITE`.

Écarts assumés par rapport au plan initial :
- **Pas de `ParseWorkbook`** : l'import n'accepte que le CSV normalisé, et **rejette toute colonne inconnue**.
  C'est la garantie qu'un fichier nominatif ne peut pas entrer par inadvertance.
- `drh_effectif` et `drh_pyramide` sont écrits **au grain le plus fin seulement** (structure / bureau / centrale /
  non rattaché × catégorie). Les rollups district / région / type, les ratios de population et la comparaison
  ISS restent au lot B, parce qu'ils dépendent aussi du snapshot ISS et doivent être recalculés à chaque synchro.
- `drh_effectif` porte en plus les colonnes `district` / `region`, pour que les rollups n'aient pas à rejoindre
  `structure_latest`.
- Ajout de `drh_non_reconnu` : la liste des libellés non rattachés est persistée et exportable en CSV, pour
  arbitrage et renvoi à la DRH.

### Lot B — ce qui a été livré

`drh/rollup.go` (rollups global / région / district / sous-préfecture / type, densités, `Compare`),
`sync.RecomputeDrh` appelé après chaque import **et** à la fin de `RunSync`, six endpoints de lecture
`/drh/*`, `drh_ratio_10k` et `drh_depart_5ans_pct` sur `/map/geo`, tests de rollup, de comparaison et
d'aller-retour en base.

Décisions prises en chemin :
- **Les non rattachés comptent dans leur district** (comme les bureaux de district) : ce sont de vrais agents
  de la préfecture, les écarter sous-estimerait la zone. L'administration centrale, elle, ne compte qu'au national.
- **Pas de pyramide des âges à la sous-préfecture** : les effectifs y sont trop petits pour qu'une répartition
  par tranche veuille dire quelque chose, et cela ferait exploser le nombre de lignes.
- **Ajout de `n_age_connu`** : le taux de départ se calcule dessus, pas sur l'effectif total (663 agents sans
  année de naissance en 2026 diluaient le taux de 14,1 % à 13,2 %).
- **La ligne « toutes catégories » de la comparaison** ne somme que les catégories qu'ISS a renseignées, des
  deux côtés, pour que l'écart ne mesure pas un trou de nomenclature.
- **Les noms de région de la DRH sont remplacés par ceux d'ISS** au rattachement (« BOKE » → « IRS Boké »),
  sinon chaque région comptait double dans les agrégats.

### Lot C — ce qui a été livré

`pages/Personnel.tsx` (KPI, répartition par catégorie et par affectation, effectifs par zone avec graphe de
densité, pyramide des âges, comparaison DRH ↔ ISS, structures du district, note de méthode), l'entrée de menu,
deux métriques sur la carte thématique, le bloc « Personnel de l'État affecté » sur la fiche d'une structure,
et une ligne « Personnel de l'État » sur la vue d'ensemble. Toutes ces vues se masquent d'elles-mêmes tant
qu'aucun millésime n'est importé (404 côté API).

Écarts assumés :
- **L'onglet RH d'Utilisation n'a pas été touché** : la comparaison de la page Personnel dit la même chose en
  plus complet (par catégorie *et* par district), une colonne « dont fonctionnaires » y aurait fait doublon.
- **La répartition par catégorie hiérarchique (A1/A2/…) n'est pas affichée** : la hiérarchie est lue dans le CSV
  mais n'est agrégée dans aucune dimension. L'ajouter demande une colonne dans `drh_effectif` et un re-import.

Trois bugs trouvés en vérifiant dans le navigateur :
- les couleurs des barres de densité étaient décalées (recharts applique les `<Cell>` dans l'ordre des données,
  pas dans celui du tableau d'origine — il fallait trier avant de les générer) ;
- la comparaison d'un district affichait en fait **tous** les districts mélangés : il manquait un filtre par zone
  dans `GetDrhComparaison` ;
- une préfecture inconnue d'ISS (« Kassa », 31 agents) fabriquait une région fantôme « CONAKRY » à côté de
  « DSV Conakry ». Elle reste visible comme district à arbitrer, mais ne crée plus de région.

## 8. Décisions à prendre avant de coder

1. **Granularité stockée** : une ligne par agent sans donnée identifiante (proposé, permet tous les
   croisements) — ou uniquement des agrégats pré-calculés (plus prudent, moins souple) ?
2. **Tranche d'âge** : garder la tranche du fichier (6 classes, déjà calculée par la DRH) ou recalculer
   des tranches quinquennales depuis l'année de naissance (plus fin pour les départs) ?
3. **Départ à la retraite à 60 ans** — à confirmer pour la fonction publique guinéenne (et faut-il
   distinguer les catégories A/B/C, dont l'âge de départ diffère parfois ?).
4. **Espace public** : rien pour l'instant (proposé), ou publier la densité par district ?
5. **Les 2 721 agents de bureaux de district** : les compter dans la densité du district (proposé) ou
   les isoler complètement ?
6. **Fichier source** : l'import se fait-il depuis le `.xlsx` brut de la DRH (pratique, mais il faudra
   gérer les variantes de colonnes chaque année) ou depuis un CSV normalisé que l'on documente ?
