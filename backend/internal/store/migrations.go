package store

const migrationSQL = `
CREATE TABLE IF NOT EXISTS sync_run (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at      TEXT NOT NULL,
    finished_at     TEXT,
    status          TEXT NOT NULL DEFAULT 'running',
    events_pulled   INTEGER DEFAULT 0,
    issues_found    INTEGER DEFAULT 0,
    duration_ms     INTEGER DEFAULT 0,
    error_text      TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS metadata_de (
    de_uid          TEXT PRIMARY KEY,
    code            TEXT DEFAULT '',
    name            TEXT NOT NULL,
    value_type      TEXT DEFAULT '',
    option_set_id   TEXT DEFAULT '',
    section_prefix  TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_metadata_de_code ON metadata_de(code);

CREATE TABLE IF NOT EXISTS option_set (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    option_set_id   TEXT NOT NULL,
    option_code     TEXT NOT NULL,
    option_name     TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_option_set_osid ON option_set(option_set_id);

CREATE TABLE IF NOT EXISTS org_unit (
    uid             TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    level           INTEGER DEFAULT 0,
    parent_uid      TEXT DEFAULT '',
    parent_name     TEXT DEFAULT '',
    closed_date     TEXT DEFAULT ''
);


CREATE TABLE IF NOT EXISTS event (
    event_uid       TEXT PRIMARY KEY,
    org_unit_uid    TEXT NOT NULL,
    org_unit_name   TEXT DEFAULT '',
    district        TEXT DEFAULT '',
    region          TEXT DEFAULT '',
    event_date      TEXT DEFAULT '',
    status          TEXT DEFAULT '',
    raw_json        TEXT DEFAULT '',
    sync_run_id     INTEGER REFERENCES sync_run(id)
);
CREATE INDEX IF NOT EXISTS idx_event_orgunit ON event(org_unit_uid);
CREATE INDEX IF NOT EXISTS idx_event_district ON event(district);

CREATE TABLE IF NOT EXISTS event_value (
    event_uid       TEXT NOT NULL REFERENCES event(event_uid),
    de_uid          TEXT NOT NULL,
    de_code         TEXT DEFAULT '',
    value           TEXT DEFAULT '',
    PRIMARY KEY (event_uid, de_uid)
);
CREATE INDEX IF NOT EXISTS idx_event_value_code ON event_value(de_code);

CREATE TABLE IF NOT EXISTS quality_issue (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    event_uid       TEXT NOT NULL REFERENCES event(event_uid),
    rule_code       TEXT NOT NULL,
    severity        TEXT NOT NULL,
    rule_name       TEXT NOT NULL,
    message         TEXT NOT NULL,
    sync_run_id     INTEGER REFERENCES sync_run(id)
);
CREATE INDEX IF NOT EXISTS idx_qi_event ON quality_issue(event_uid);
CREATE INDEX IF NOT EXISTS idx_qi_severity ON quality_issue(severity);
CREATE INDEX IF NOT EXISTS idx_qi_rule ON quality_issue(rule_code);

CREATE TABLE IF NOT EXISTS event_quality (
    event_uid       TEXT PRIMARY KEY REFERENCES event(event_uid),
    n_error         INTEGER DEFAULT 0,
    n_warning       INTEGER DEFAULT 0,
    n_info          INTEGER DEFAULT 0,
    worst_severity  TEXT DEFAULT '',
    score           INTEGER DEFAULT 100
);

CREATE TABLE IF NOT EXISTS quality_summary (
    dimension       TEXT NOT NULL,
    key             TEXT NOT NULL,
    label           TEXT DEFAULT '',
    avg_score       REAL DEFAULT 0,
    n_error         INTEGER DEFAULT 0,
    n_warning       INTEGER DEFAULT 0,
    n_info          INTEGER DEFAULT 0,
    n_structures    INTEGER DEFAULT 0,
    PRIMARY KEY (dimension, key)
);

CREATE TABLE IF NOT EXISTS usage_recensement (
    dimension       TEXT NOT NULL,
    key             TEXT NOT NULL,
    label           TEXT DEFAULT '',
    n_structures    INTEGER DEFAULT 0,
    n_operationnel  INTEGER DEFAULT 0,
    n_non_operationnel INTEGER DEFAULT 0,
    n_ferme_temp    INTEGER DEFAULT 0,
    PRIMARY KEY (dimension, key)
);

CREATE TABLE IF NOT EXISTS usage_service (
    service_code    TEXT NOT NULL,
    service_label   TEXT DEFAULT '',
    district        TEXT NOT NULL DEFAULT 'all',
    n_oui           INTEGER DEFAULT 0,
    n_oui_pas_fonc  INTEGER DEFAULT 0,
    n_non           INTEGER DEFAULT 0,
    n_total         INTEGER DEFAULT 0,
    pct_fonctionnel REAL DEFAULT 0,
    PRIMARY KEY (service_code, district)
);

CREATE TABLE IF NOT EXISTS usage_equipement (
    equip_root      TEXT NOT NULL,
    label           TEXT DEFAULT '',
    district        TEXT NOT NULL DEFAULT 'all',
    sum_total       INTEGER DEFAULT 0,
    sum_fonct       INTEGER DEFAULT 0,
    pct_fonct       REAL DEFAULT 0,
    category        TEXT DEFAULT '',
    PRIMARY KEY (equip_root, district)
);

CREATE TABLE IF NOT EXISTS usage_rh (
    profil_code     TEXT NOT NULL,
    label           TEXT DEFAULT '',
    district        TEXT NOT NULL DEFAULT 'all',
    effectif_fonc   INTEGER DEFAULT 0,
    effectif_contr  INTEGER DEFAULT 0,
    effectif_benev  INTEGER DEFAULT 0,
    effectif_total  INTEGER DEFAULT 0,
    PRIMARY KEY (profil_code, district)
);

CREATE TABLE IF NOT EXISTS usage_commodite (
    indicator       TEXT NOT NULL,
    district        TEXT NOT NULL DEFAULT 'all',
    n_oui           INTEGER DEFAULT 0,
    n_total         INTEGER DEFAULT 0,
    pct             REAL DEFAULT 0,
    PRIMARY KEY (indicator, district)
);

CREATE TABLE IF NOT EXISTS reporting_rate (
    dimension       TEXT NOT NULL,
    key             TEXT NOT NULL,
    label           TEXT DEFAULT '',
    n_expected      INTEGER DEFAULT 0,
    n_reported      INTEGER DEFAULT 0,
    pct             REAL DEFAULT 0,
    PRIMARY KEY (dimension, key)
);

CREATE TABLE IF NOT EXISTS user (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    username        TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    name            TEXT DEFAULT '',
    role            TEXT NOT NULL DEFAULT 'viewer',
    created_at      TEXT NOT NULL
);
`
