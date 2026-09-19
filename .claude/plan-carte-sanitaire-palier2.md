# Plan — Carte sanitaire, palier 2 : normes et écarts

> Statut : **livré le 19/09/2026** (4 lots) — validé le 18/09/2026 (privé soumis aux normes du type ; essentiel ×2 ; inconnu hors score ; édition admin ; CSV d'exemple non officiel avec seuils chiffrés, médecin en CS recommandé).
> Contexte : aucun document officiel de normes n'est disponible (voir mémoire `carte-sanitaire-normes`).
> Le palier est donc conçu autour d'un **référentiel de normes éditable dans l'app**, versionné, à remplir
> progressivement et à valider par le MSHP. Le jour où le texte officiel arrive, on l'importe dans ce référentiel.

## 0. Ce que ça apporte

Le palier 1 dit *ce que chaque structure a*. Le palier 2 dit *ce qui lui manque par rapport à ce qu'elle devrait
avoir*, structure par structure, puis agrégé : « dans le district X, 12 CS sur 15 n'ont pas de sage-femme, il
manque 9 sages-femmes pour atteindre la norme ». C'est la fonction planification d'une carte sanitaire, et ce
que la cible OpenHEXA n'a qu'en germe (« score de disponibilité des 7 services principaux »).

Trois familles d'exigences, toutes indexées sur `type_code` (PS | CS | CSA | CMC | HP | HR | HN | privé) :

| Famille | Exemple de norme | Donnée ISS comparée |
|---|---|---|
| **Services** (paquet minimum) | un CS doit offrir CPN, accouchement, PEV, curatif, labo | DE `ISS_SVC_*` = `oui` |
| **Ressources humaines** (effectif minimal) | un CS : ≥ 1 sage-femme, ≥ 2 infirmiers ; un HP : ≥ 1 chirurgien | somme des DE du profil (`ISS_RH_SAGEF`, `ISS_RH_MED_CHIR`…), tous statuts d'emploi |
| **Équipements & infrastructures** (dotation minimale) | un CS : ≥ 1 table d'accouchement fonctionnelle, ≥ 1 réfrigérateur fonctionnel, ≥ 1 salle d'accouchement | `*_FONC` du couple équipement (`ISS_EQUI_TABLE_ACC`), ou DE infra (`ISS_INFRA_SALLE_ACC_DE`) |

Hors périmètre de ce palier (palier 3) : normes de **couverture** (1 CS pour N habitants, rayon d'action) et
accessibilité routière — elles demandent une analyse spatiale, pas un référentiel.

---

## 1. Principes de conception

1. **Le référentiel est une donnée, pas du code.** Tables SQLite éditées par l'admin via l'API et l'écran Admin ;
   import/export CSV pour travailler dans Excel avec le Ministère. Aucune norme codée en dur.
2. **Versionné, une seule version active.** On édite un brouillon, on l'active ; les versions précédentes
   restent consultables (traçabilité : « ce rapport a été produit avec le référentiel v2 du 3 octobre »).
3. **Conformité ≠ qualité.** Nouveau package `internal/normes`, nouvelles tables `conformite_*`. Le score
   qualité (données fiables ?) et le score de conformité (structure aux normes ?) sont deux choses ; on ne
   mélange pas leurs règles ni leurs pénalités.
4. **Recalcul sans DHIS2.** Modifier ou activer un référentiel déclenche `RecomputeConformite()` sur les
   events déjà en base (quelques secondes) ; `RunSync` l'appelle aussi en fin de pipeline.
5. **Non renseigné ≠ non conforme.** Une exigence dont la donnée ISS est vide est comptée « inconnue », pas
   « manquante ». Sinon la qualité des données contaminerait la conformité — et l'inverse est déjà signalé par
   le moteur qualité.
6. **Espace public : rien de tout ça** (décision option B du palier 1). À reconsidérer plus tard si le MSHP
   veut publier « services attendus / offerts » sur la fiche.

---

## 2. Modèle de données

```sql
-- Une version du référentiel
CREATE TABLE norme_set (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT NOT NULL,              -- ex. "Normes MSHP – brouillon BSD"
    version      INTEGER NOT NULL,           -- 1, 2, 3… (incrémenté à la duplication)
    status       TEXT NOT NULL DEFAULT 'draft', -- draft | active | archived
    notes        TEXT DEFAULT '',
    created_at   TEXT NOT NULL,
    created_by   TEXT DEFAULT '',
    activated_at TEXT
);

-- Une exigence : « les structures de type T doivent avoir au moins N de X »
CREATE TABLE norme_rule (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    set_id       INTEGER NOT NULL REFERENCES norme_set(id) ON DELETE CASCADE,
    type_code    TEXT NOT NULL,              -- PS | CS | … | * (tous types)
    kind         TEXT NOT NULL,              -- service | rh | equipement | infra
    target       TEXT NOT NULL,              -- code DE (service/infra), racine profil RH, racine équipement
    label        TEXT NOT NULL,              -- libellé lisible (pré-rempli depuis les métadonnées)
    min_value    REAL NOT NULL DEFAULT 1,    -- service : 1 = doit être 'oui' ; rh/equip/infra : minimum
    level        TEXT NOT NULL DEFAULT 'essentiel', -- essentiel | recommande
    UNIQUE (set_id, type_code, kind, target)
);
CREATE INDEX idx_norme_rule_set ON norme_rule(set_id);

-- Résultat par structure × exigence (référentiel actif), recalculé
CREATE TABLE conformite_item (
    event_uid    TEXT NOT NULL,
    rule_id      INTEGER NOT NULL,
    kind         TEXT NOT NULL,
    target       TEXT NOT NULL,
    label        TEXT NOT NULL,
    level        TEXT NOT NULL,
    expected     REAL NOT NULL,
    observed     REAL,                       -- NULL = non renseigné
    status       TEXT NOT NULL,              -- ok | manque | inconnu
    PRIMARY KEY (event_uid, rule_id)
);

-- Synthèse par structure
CREATE TABLE event_conformite (
    event_uid        TEXT PRIMARY KEY,
    set_id           INTEGER NOT NULL,
    n_rules          INTEGER, n_ok INTEGER, n_manque INTEGER, n_inconnu INTEGER,
    n_manque_essentiel INTEGER,
    score            REAL                    -- voir §3
);

-- Agrégats : conformité moyenne et écarts (long)
CREATE TABLE conformite_summary (          -- dimension: global|region|district|sous_prefecture|type
    dimension TEXT, key TEXT, label TEXT, type_code TEXT DEFAULT '',
    n_structures INTEGER, avg_score REAL, n_conformes INTEGER,  -- conformes = 0 manque essentiel
    PRIMARY KEY (dimension, key, type_code)
);
CREATE TABLE conformite_gap (              -- « il manque X de Y dans Z »
    dimension TEXT, key TEXT, type_code TEXT, kind TEXT, target TEXT, label TEXT, level TEXT,
    n_concernees INTEGER,                   -- structures du type soumises à l'exigence
    n_manque INTEGER, n_inconnu INTEGER,
    deficit REAL,                           -- Σ max(0, attendu − observé) ; pour un service = n_manque
    PRIMARY KEY (dimension, key, type_code, kind, target)
);
```

Tables `conformite_*` vidées/recréées à chaque recalcul, dans une transaction, comme les `usage_*`.

---

## 3. Évaluation (package `internal/normes`)

```go
type Rule struct { TypeCode, Kind, Target, Label string; Min float64; Level string }
type Item struct { Rule; Expected float64; Observed *float64; Status string } // ok|manque|inconnu

func Evaluate(evt *models.Event, rules []Rule, ctx *quality.QualityContext) []Item
```

Résolution de l'observé, par famille :
- `service` : valeur du DE `target` — `oui` → 1 ; `non` / `oui_pas_fonctionnel` → 0 ; vide → inconnu.
- `rh` : somme des DE dont la racine (via `usage.DiscoverRHProfiles`) = `target`, tous statuts (fonctionnaire +
  contractuel + bénévole) ; aucun DE renseigné → inconnu. `target` peut être un préfixe (`ISS_RH_MED_` = un
  médecin quel qu'il soit).
- `equipement` : `*_FONC` du couple `ctx.EquipPairs` de racine `target` (fonctionnel, pas total) ; vide → inconnu.
- `infra` : valeur numérique du DE `target` ; vide → inconnu.

Les règles `type_code = '*'` s'appliquent à tous les types ; une règle spécifique au type prime sur `*` pour
le même `(kind, target)`. Les structures `INDETERMINE` ne sont évaluées que par les règles `*`.

**Score de conformité** (0–100) : `100 × Σ poids(ok) / Σ poids(ok + manque)` avec poids essentiel = 2,
recommandé = 1 ; les `inconnu` sont hors dénominateur. Une structure est **conforme** si `n_manque_essentiel = 0`.
Pas de score si aucune règle ne s'applique au type (NULL, affiché « non évalué »).

Tests unitaires : une règle par famille (ok / manque / inconnu), priorité type vs `*`, préfixe RH, score et
pondération, structure sans règle.

---

## 4. Déclenchement et pipeline

```
RecomputeConformite(st)                          ← admin : activation / modification du set actif ; fin de RunSync
  |-- charge le set actif et ses règles
  |-- charge les events (dernier par org unit → mêmes structures que l'annuaire), métadonnées, contexte
  |-- Evaluate() par structure → conformite_item, event_conformite
  |-- agrège conformite_summary (global / region / district / sous_prefecture / type) et conformite_gap
  +-- persistance transactionnelle
```

Un `norme_set` activé sans règle → conformité vide (pas d'erreur). Le log de sync indique
« Conformité : set v3 (42 règles), 3 097 structures évaluées, 61 % conformes ».

---

## 5. API

**Admin (JWT + rôle admin)**
```
GET    /admin/normes                       → liste des sets (version, statut, nb règles)
POST   /admin/normes                       → crée un set vide (draft)
POST   /admin/normes/:id/duplicate         → nouvelle version brouillon copiée (pour modifier l'actif sans le casser)
PUT    /admin/normes/:id                   → nom, notes
POST   /admin/normes/:id/activate          → archive l'actif courant, active celui-ci, recalcule
DELETE /admin/normes/:id                   → seulement un brouillon
GET    /admin/normes/:id/rules             → règles
PUT    /admin/normes/:id/rules             → remplace toutes les règles (édition en grille)
POST   /admin/normes/:id/rules/import      → CSV (type_code;kind;target;label;min_value;level), validation
                                              des cibles contre les métadonnées, rapport d'erreurs par ligne
GET    /admin/normes/:id/rules/export.csv
POST   /admin/normes/recompute             → recalcul manuel
GET    /admin/normes/targets               → catalogue des cibles possibles (services, profils RH, équipements,
                                              infra) avec libellés, pour l'éditeur
```

**Lecture (espace planification)**
```
GET /conformite/summary?by=global|region|district|sous_prefecture|type&type=
GET /conformite/gaps?by=district&key=&type=&kind=&level=      → écarts triés par déficit
GET /conformite/structures?district=&type=&status=conforme|non_conforme&page=   → liste + score
GET /quality/event/:uid                                        → existant, + bloc `conformite` (items)
GET /map/geo?level=                                            → existant, + avg_score_conformite, pct_conformes
GET /meta/normes                                               → set actif (nom, version, date) pour l'affichage
```

---

## 6. Front (espace planification)

- **Admin → onglet « Normes »** : liste des versions ; éditeur en grille du brouillon (lignes = exigences,
  colonnes = type / famille / cible (menu depuis `/targets`) / minimum / niveau) ; import/export CSV ;
  boutons Dupliquer / Activer ; bandeau « référentiel actif : v2 – 42 règles – activé le … ».
- **Page « Conformité »** (`/conformite`) : KPI (score moyen, % structures conformes, nb écarts essentiels) ;
  tableau par type puis par district/région ; **table des écarts** « il manque X » filtrable (district, type,
  famille) et exportable — c'est la sortie planification ; graphe barres.
- **Carte** : couche « Conformité » dans `ProGeoMap` (score moyen, % conformes) niveaux 3-4 ; couche points
  colorés par conformité.
- **Détail structure** : section « Conformité aux normes » (attendu / observé / statut par exigence, score).
- **Vue d'ensemble** : une carte KPI « % structures conformes » quand un référentiel est actif.

---

## 7. Amorçage sans document officiel

- Le set v1 est créé **vide et actif** : l'app fonctionne, rien n'est évalué.
- Un fichier `docs/normes-exemple.csv` est fourni, **explicitement non officiel** (en-tête de commentaire),
  avec une trentaine de lignes plausibles (paquet minimum PS/CS, effectifs CS/HP, chaîne du froid…) pour que
  l'admin puisse tester l'import et voir l'outil vivre. À valider avec toi : je préfère te soumettre son
  contenu ligne par ligne plutôt que d'inventer des normes guinéennes.
- Recommandation d'usage : le BSD/DNEHS remplit le brouillon dans Excel à partir de l'export CSV, l'admin
  importe, active, et le rapport d'écarts sert de base de discussion.

---

## 8. Arbre des fichiers

```
backend/internal/
  normes/               NOUVEAU : rules.go (types, parsing CSV, validation des cibles), evaluate.go,
                        aggregate.go, recompute.go (RecomputeConformite), normes_test.go
  store/normes_store.go NOUVEAU : CRUD sets/règles, persistance conformite_*, requêtes de lecture
  store/migrations.go   + tables §2
  api/normes_handlers.go NOUVEAU : admin + lecture
  api/router.go         + routes
  sync/sync.go          + appel RecomputeConformite en fin de pipeline
  models/models.go      + types
docs/normes-exemple.csv NOUVEAU (non officiel)
frontend/src/
  pages/Conformite.tsx, pages/admin/NormesEditor.tsx        NOUVEAU
  pages/Admin.tsx, Dashboard.tsx, StructureDetail.tsx, MapView.tsx / components/map/ProGeoMap.tsx  modifiés
  api/client.ts, types/index.ts, components/Layout.tsx (entrée « Conformité »)
README.md               section « Normes et conformité » (cycle brouillon → activation, format CSV)
```

---

## 9. Lots

| Lot | Contenu | Vérification |
|---|---|---|
| **A — Référentiel** ✅ | tables, CRUD, import/export CSV avec validation des cibles, catalogue des cibles, tests | `curl` : créer, importer l'exemple (212 règles, 0 erreur contre le catalogue réel), exporter, dupliquer, activer, supprimer un brouillon, validation PUT, 401 sans droit |
| **B — Évaluation** ✅ | `normes.Evaluate`, agrégats, `RecomputeConformite`, hook dans RunSync, endpoints de lecture, tests | recalcul réel : 212 règles, 3 097 structures évaluées en 1,1 s, 409 conformes ; PS 70 (8 % conformes), CS 86 (37 %), HP 87 (7 %) ; écarts par district cohérents (Kankan CS : 19/28 sans médecin) ; détail HN Donka |
| **C — Front** ✅ | onglet Admin Normes, page Conformité, couche carte, détail structure, KPI | vérifié dans le navigateur : éditeur en grille (212 règles, catalogue des cibles), page Conformité (KPI, par type, par zone, écarts Kankan/CS, structures), détail HN Donka (46 exigences, 7 manques), lien depuis la vue d'ensemble |
| **D — Doc** ✅ | README, CSV d'exemple annoté | relecture |

---

## 10. Décisions à prendre avant de coder

1. **Structures privées** : soumises aux mêmes normes que le public du même type (`CS` privé = `CS`), ou
   exclues par défaut ? Proposition : soumises, avec un filtre statut sur la page Conformité.
2. **Pondération** essentiel ×2 / recommandé ×1 et « conforme = 0 manque essentiel » : OK ?
3. **`inconnu` hors score** (principe 5) : OK ? L'alternative (inconnu = manque) durcit le score mais mélange
   qualité et conformité.
4. **Édition** : l'admin seul édite (rôle `admin`), les `viewer` lisent. Faut-il un rôle intermédiaire
   « éditeur de normes » ? Proposition : non pour l'instant.
5. **CSV d'exemple** : je te propose son contenu pour validation avant de le mettre dans le dépôt, ou on livre
   le set vide seulement.
