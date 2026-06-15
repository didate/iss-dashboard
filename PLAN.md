# Plan d'implémentation — Tableau de bord ISS

## Données clés extraites du programme

- **225 data elements** répartis en 8 sections
- **36 couples équipement** (TOTAL/FONC) à détecter dynamiquement — attention aux codes irréguliers (`ISS_EQUI_TABLE__PAN_OP_FONC_DE` vs `ISS_EQUI_TABLE_PAN_TOTAL_DE`, `ISS_NB_LIT_HOSP_*`, `ISS_NB_PESE_BEBE*`)
- **6 option sets** : statut publique, statut privé, services fonctionnalité (`oui`/`non`/`oui_pas_fonctionnel`), statut structure, statut opérationnel (`operationnel`/`non_operationnel`/`ferme_temporairement`), source d'eau
- **40 services** (section ISS_SVC + ISS_LAB) utilisant l'optionSet `RGsTov6dBHH`
- **~53 RH** avec pattern `_FN_DE`/`_CT_DE`/`_BN_DE` (fonctionnaire/contractuel/bénévole)

---

## 1. Schéma SQLite définitif

```sql
-- Historique des synchros
CREATE TABLE sync_run (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at      TEXT NOT NULL,           -- ISO8601
    finished_at     TEXT,
    status          TEXT NOT NULL DEFAULT 'running',  -- running|success|error
    events_pulled   INTEGER DEFAULT 0,
    issues_found    INTEGER DEFAULT 0,
    duration_ms     INTEGER,
    error_text      TEXT
);

-- Métadonnées DHIS2
CREATE TABLE metadata_de (
    de_uid          TEXT PRIMARY KEY,
    code            TEXT,
    name            TEXT NOT NULL,
    value_type      TEXT,
    option_set_id   TEXT,
    section_prefix  TEXT                     -- ISS_GEN, ISS_EQ, ISS_INFRA, etc.
);
CREATE INDEX idx_metadata_de_code ON metadata_de(code);

CREATE TABLE option_set (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    option_set_id   TEXT NOT NULL,
    option_code     TEXT NOT NULL,
    option_name     TEXT NOT NULL
);
CREATE INDEX idx_option_set_osid ON option_set(option_set_id);

CREATE TABLE org_unit (
    uid             TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    level           INTEGER,
    parent_uid      TEXT,
    parent_name     TEXT
);

-- Events (une structure = un event)
CREATE TABLE event (
    event_uid       TEXT PRIMARY KEY,
    org_unit_uid    TEXT NOT NULL,
    org_unit_name   TEXT,
    district        TEXT,                    -- déduit de l'org unit hierarchy
    region          TEXT,                    -- déduit de l'org unit hierarchy
    event_date      TEXT,
    status          TEXT,
    raw_json        TEXT,                    -- JSON complet pour debug
    sync_run_id     INTEGER REFERENCES sync_run(id)
);
CREATE INDEX idx_event_orgunit ON event(org_unit_uid);
CREATE INDEX idx_event_district ON event(district);

-- Valeurs parsées de chaque event
CREATE TABLE event_value (
    event_uid       TEXT NOT NULL REFERENCES event(event_uid),
    de_uid          TEXT NOT NULL,
    de_code         TEXT,
    value           TEXT,
    PRIMARY KEY (event_uid, de_uid)
);
CREATE INDEX idx_event_value_code ON event_value(de_code);

-- Issues qualité (une ligne par problème)
CREATE TABLE quality_issue (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    event_uid       TEXT NOT NULL REFERENCES event(event_uid),
    rule_code       TEXT NOT NULL,            -- R1, R2, R3...
    severity        TEXT NOT NULL,            -- error, warning, info
    rule_name       TEXT NOT NULL,            -- libellé catégorie
    message         TEXT NOT NULL,
    sync_run_id     INTEGER REFERENCES sync_run(id)
);
CREATE INDEX idx_qi_event ON quality_issue(event_uid);
CREATE INDEX idx_qi_severity ON quality_issue(severity);
CREATE INDEX idx_qi_rule ON quality_issue(rule_code);

-- Synthèse qualité par event
CREATE TABLE event_quality (
    event_uid       TEXT PRIMARY KEY REFERENCES event(event_uid),
    n_error         INTEGER DEFAULT 0,
    n_warning       INTEGER DEFAULT 0,
    n_info          INTEGER DEFAULT 0,
    worst_severity  TEXT,                    -- error > warning > info > NULL
    score           INTEGER DEFAULT 100      -- 0-100
);

-- Agrégat qualité par dimension
CREATE TABLE quality_summary (
    dimension       TEXT NOT NULL,            -- global, district, region, statut
    key             TEXT NOT NULL,            -- valeur (nom district, etc.) ou 'all'
    label           TEXT,
    avg_score       REAL,
    n_error         INTEGER DEFAULT 0,
    n_warning       INTEGER DEFAULT 0,
    n_info          INTEGER DEFAULT 0,
    n_structures    INTEGER DEFAULT 0,
    PRIMARY KEY (dimension, key)
);

-- Usage : recensement
CREATE TABLE usage_recensement (
    dimension       TEXT NOT NULL,            -- global, district, region, statut_juridique
    key             TEXT NOT NULL,
    label           TEXT,
    n_structures    INTEGER DEFAULT 0,
    n_operationnel  INTEGER DEFAULT 0,
    n_non_operationnel INTEGER DEFAULT 0,
    n_ferme_temp    INTEGER DEFAULT 0,
    PRIMARY KEY (dimension, key)
);

-- Usage : services
CREATE TABLE usage_service (
    service_code    TEXT NOT NULL,
    service_label   TEXT,
    district        TEXT NOT NULL DEFAULT 'all',
    n_oui           INTEGER DEFAULT 0,       -- fonctionnel
    n_oui_pas_fonc  INTEGER DEFAULT 0,       -- prévu mais non fonctionnel
    n_non           INTEGER DEFAULT 0,
    n_total         INTEGER DEFAULT 0,       -- events avec cette donnée
    pct_fonctionnel REAL DEFAULT 0,
    PRIMARY KEY (service_code, district)
);

-- Usage : équipements
CREATE TABLE usage_equipement (
    equip_root      TEXT NOT NULL,            -- racine du code (ex: ISS_EQUI_FRIGO)
    label           TEXT,
    district        TEXT NOT NULL DEFAULT 'all',
    sum_total       INTEGER DEFAULT 0,
    sum_fonct       INTEGER DEFAULT 0,
    pct_fonct       REAL DEFAULT 0,
    category        TEXT,                    -- chaine_froid, imagerie, transport, etc.
    PRIMARY KEY (equip_root, district)
);

-- Usage : RH
CREATE TABLE usage_rh (
    profil_code     TEXT NOT NULL,            -- racine (ISS_RH_INF, ISS_RH_MED_GEN...)
    label           TEXT,
    district        TEXT NOT NULL DEFAULT 'all',
    effectif_fonc   INTEGER DEFAULT 0,
    effectif_contr  INTEGER DEFAULT 0,
    effectif_benev  INTEGER DEFAULT 0,
    effectif_total  INTEGER DEFAULT 0,
    PRIMARY KEY (profil_code, district)
);

-- Usage : commodités (WASH + énergie)
CREATE TABLE usage_commodite (
    indicator       TEXT NOT NULL,            -- energie, eau_pts_critiques, solaire
    district        TEXT NOT NULL DEFAULT 'all',
    n_oui           INTEGER DEFAULT 0,
    n_total         INTEGER DEFAULT 0,
    pct             REAL DEFAULT 0,
    PRIMARY KEY (indicator, district)
);
```

**Stratégie de remplacement** : chaque `RunSync()` exécute tout dans une transaction. En cas de succès, `DELETE` puis `INSERT` sur toutes les tables dérivées (quality_issue, event_quality, quality_summary, usage_*). En cas d'erreur, `ROLLBACK` — l'ancien snapshot reste intact.

---

## 2. Modules Go

```
backend/
├── main.go                          # init config, DB, scheduler, router, start server
├── go.mod / go.sum
├── Dockerfile
├── internal/
│   ├── config/
│   │   └── config.go                # struct Config, chargement .env
│   ├── models/
│   │   └── models.go                # structs Go : Event, DataValue, SyncRun, Issue, etc.
│   ├── dhis2/
│   │   └── client.go                # HTTP client DHIS2 : FetchEvents (paginé), FetchMetadata
│   ├── store/
│   │   ├── store.go                 # interface Store + implem SQLite
│   │   ├── migrations.go            # CREATE TABLE IF NOT EXISTS
│   │   └── queries.go               # méthodes Read pour les endpoints API
│   ├── quality/
│   │   ├── engine.go                # type Rule, Registry, RunAll(events, ctx) → issues
│   │   ├── context.go               # QualityContext : metadata, couples, médianes
│   │   ├── r1_required.go           # R1 — champs obligatoires
│   │   ├── r2_total_fonc.go         # R2 — cohérence total/fonctionnel
│   │   ├── r3_service_support.go    # R3 — service sans support
│   │   ├── r4_commodites.go         # R4 — cohérence commodités
│   │   ├── r5_outliers.go           # R5 — valeurs aberrantes
│   │   ├── r6_duplicates.go         # R6 — doublons org unit
│   │   ├── r7_completeness.go       # R7 — coquille vide
│   │   ├── score.go                 # calcul score par event
│   │   └── quality_test.go          # tests unitaires
│   ├── usage/
│   │   ├── recensement.go           # agrégats recensement
│   │   ├── services.go              # agrégats services
│   │   ├── equipements.go           # agrégats équipements
│   │   ├── rh.go                    # agrégats RH
│   │   └── commodites.go            # agrégats commodités
│   ├── sync/
│   │   └── sync.go                  # RunSync() : orchestration pull → parse → rules → usage → persist
│   ├── api/
│   │   ├── router.go                # setup Gin routes + middleware
│   │   ├── middleware.go            # auth admin (X-Admin-Token), CORS, dashboard public
│   │   ├── admin_handlers.go        # POST /sync, GET /sync/status
│   │   ├── summary_handlers.go      # GET /summary
│   │   ├── quality_handlers.go      # GET /quality/*
│   │   ├── usage_handlers.go        # GET /usage/*
│   │   └── meta_handlers.go         # GET /meta/filters
│   └── scheduler/
│       └── scheduler.go             # robfig/cron wrapper
```

---

## 3. Contrat des endpoints API

### Admin (`X-Admin-Token` requis)

| Méthode | Route | Description | Réponse |
|---------|-------|-------------|---------|
| `POST` | `/api/admin/sync` | Lance RunSync() | `{sync_run_id, status: "running"}` |
| `GET` | `/api/admin/sync/status` | Dernière synchro + en cours | `{current: SyncRun?, last: SyncRun, history: SyncRun[]}` |

### Lecture (publique si `DASHBOARD_PUBLIC=true`)

| Méthode | Route | Params | Réponse |
|---------|-------|--------|---------|
| `GET` | `/api/summary` | — | `{n_structures, n_operationnel, avg_score, n_error, n_warning, n_info, last_sync: {date, duration_ms, status}}` |
| `GET` | `/api/quality/summary` | `?by=district\|region\|statut` | `[{key, label, avg_score, n_error, n_warning, n_info, n_structures}]` |
| `GET` | `/api/quality/issues` | `?severity=&rule=&district=&search=&page=1&pageSize=20` | `{data: [{event_uid, org_unit_name, district, region, worst_severity, score, n_error, n_warning, n_info, issues: [{rule_code, severity, message}]}], total, page, pageSize}` |
| `GET` | `/api/quality/event/:uid` | — | `{event, values: [{de_code, de_name, value}], issues: [{rule_code, severity, rule_name, message}], quality: {score, n_error, n_warning, n_info}}` |
| `GET` | `/api/usage/recensement` | `?by=district\|region\|statut` | `[{key, label, n_structures, n_operationnel, n_non_operationnel, n_ferme_temp}]` |
| `GET` | `/api/usage/services` | `?district=` | `[{service_code, service_label, n_oui, n_total, pct_fonctionnel}]` |
| `GET` | `/api/usage/equipements` | `?focus=chaine_froid\|imagerie\|all&district=` | `[{equip_root, label, sum_total, sum_fonct, pct_fonct, category}]` |
| `GET` | `/api/usage/rh` | `?by=profil\|statut&district=` | `[{profil_code, label, effectif_fonc, effectif_contr, effectif_benev, effectif_total}]` |
| `GET` | `/api/usage/commodites` | `?district=` | `[{indicator, label, n_oui, n_total, pct}]` |
| `GET` | `/api/meta/filters` | — | `{districts: [], regions: [], rules: [], services: [], statuts: []}` |

---

## 4. Détection dynamique des couples équipement

Algorithme au chargement des métadonnées :
1. Collecter tous les DE dont le code finit par `_TOTAL_DE` ou `_TOTAL`
2. Pour chaque TOTAL, chercher un FONC correspondant en remplaçant `_TOTAL_DE` → `_FONC_DE` et `_TOTAL` → `_FONC`
3. Cas spéciaux détectés dans les données réelles :
   - `ISS_EQUI_TABLE_PAN_TOTAL_DE` ↔ `ISS_EQUI_TABLE__PAN_OP_FONC_DE` (double underscore + `_OP`)
   - `ISS_NB_LIT_HOSP_TOTAL` ↔ `ISS_NB_LIT_HOSP_FONC` (pas de suffixe `_DE`)
   - `ISS_NB_PESE_BEBE` ↔ `ISS_NB_PESE_BEBE_FONC_DE` (TOTAL sans suffixe)
4. Fallback : si le match direct échoue, chercher par racine commune (tout sauf TOTAL*/FONC*) parmi les DE de section `ISS_EQ`

---

## 5. Moteur de règles — interface

```go
type Issue struct {
    RuleCode string   // "R1", "R2", etc.
    Severity string   // "error", "warning", "info"
    RuleName string   // "Champs obligatoires", "Cohérence total/fonctionnel"...
    Message  string   // texte précis
}

type QualityContext struct {
    Metadata       map[string]DataElement  // de_uid → DE
    CodeToUID      map[string]string       // de_code → de_uid
    EquipPairs     []EquipPair             // couples {Root, TotalUID, FoncUID, Label}
    OptionCodes    map[string][]Option     // option_set_id → options
    Medians        map[string]MedianStat   // de_uid → {median, mad}
    OrgUnitCounts  map[string]int          // org_unit_uid → nb events
    AllEvents      []Event                 // pour les règles globales (R5, R6)
}

type Rule func(event Event, ctx *QualityContext) []Issue

// Registry
var Rules = []struct {
    Code string
    Name string
    Fn   Rule
}{
    {"R1", "Champs obligatoires", CheckRequiredFields},
    {"R2", "Cohérence total/fonctionnel", CheckTotalFonctionnel},
    // ...
}
```

---

## 6. Score qualité

```
score = max(0, 100 - 15*n_error - 5*n_warning - 1*n_info)
```

---

## 7. Arbre de fichiers frontend

```
frontend/
├── Dockerfile
├── nginx.conf
├── package.json
├── vite.config.ts
├── index.html
├── src/
│   ├── main.tsx
│   ├── App.tsx                      # router (react-router-dom)
│   ├── api/
│   │   └── client.ts                # fetch wrapper, base URL
│   ├── pages/
│   │   ├── Dashboard.tsx            # vue d'ensemble (KPIs + graphes)
│   │   ├── Quality.tsx              # tableau issues + panneau détail
│   │   ├── Usage.tsx                # onglets recensement/services/equip/rh/commo
│   │   └── Admin.tsx                # sync button + historique
│   ├── components/
│   │   ├── Layout.tsx               # sidebar + header
│   │   ├── KpiCard.tsx
│   │   ├── SeverityBadge.tsx
│   │   ├── DataTable.tsx            # tableau générique paginé
│   │   ├── ScoreBar.tsx             # barre de score colorée
│   │   ├── charts/
│   │   │   ├── ScoreByDistrict.tsx
│   │   │   ├── IssuesByRule.tsx
│   │   │   ├── ServiceMatrix.tsx
│   │   │   └── EquipmentChart.tsx
│   │   └── filters/
│   │       └── FilterBar.tsx
│   └── types/
│       └── index.ts                 # types TS miroir des réponses API
```

Libs front : `react-router-dom`, `recharts`, `lucide-react` (icônes), `tailwindcss`.

---

## 8. Docker / Traefik

```yaml
# docker-compose.yml
services:
  backend:
    build: ./backend
    volumes:
      - iss-data:/data
    env_file: .env
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.iss-api.rule=Host(`api.iss.example.com`)"
      - "traefik.http.services.iss-api.loadbalancer.server.port=8080"

  frontend:
    build: ./frontend
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.iss-front.rule=Host(`iss.example.com`)"
      - "traefik.http.services.iss-front.loadbalancer.server.port=80"

volumes:
  iss-data:
```

---

## 9. Ordre d'implémentation

1. **Backend** : config → models → store (migrations) → dhis2 client → quality engine (+ tests) → usage aggregators → sync orchestrator → API handlers → scheduler → main.go
2. **Frontend** : scaffold Vite + Tailwind → API client → Layout → Dashboard → Quality → Usage → Admin
3. **Docker** : Dockerfiles + docker-compose + nginx.conf + .env.example + README
