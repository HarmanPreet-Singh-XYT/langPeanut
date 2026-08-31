-- 008_seo_growth_studio.sql
-- Autonomous Multilingual SEO & Market Growth Studio Schema

CREATE TABLE IF NOT EXISTS repo_seo_strategies (
    repo_id              INTEGER PRIMARY KEY REFERENCES repos(id) ON DELETE CASCADE,
    project_name         TEXT    NOT NULL DEFAULT '',
    category             TEXT    NOT NULL DEFAULT 'Software Platform',
    product_description  TEXT    NOT NULL DEFAULT '',
    target_locales_json  TEXT    NOT NULL DEFAULT '["ja", "de", "es"]',
    goal                 TEXT    NOT NULL DEFAULT 'traffic',
    scope_tier           TEXT    NOT NULL DEFAULT 'high_impact',
    competitor_urls_json TEXT    NOT NULL DEFAULT '[]',
    updated_at           DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS repo_seo_competitors (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id              INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    locale               TEXT    NOT NULL,
    domain               TEXT    NOT NULL,
    rank                 INTEGER NOT NULL DEFAULT 1,
    url                  TEXT    NOT NULL DEFAULT '',
    title                TEXT    NOT NULL DEFAULT '',
    meta_description     TEXT    NOT NULL DEFAULT '',
    h1s_json             TEXT    NOT NULL DEFAULT '[]',
    h2s_json             TEXT    NOT NULL DEFAULT '[]',
    keywords_json        TEXT    NOT NULL DEFAULT '[]',
    value_props_json     TEXT    NOT NULL DEFAULT '[]',
    is_discovered        INTEGER NOT NULL DEFAULT 1,
    updated_at           DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE(repo_id, locale, domain)
);

CREATE TABLE IF NOT EXISTS repo_seo_keywords (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id              INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    locale               TEXT    NOT NULL,
    keyword              TEXT    NOT NULL,
    intent               TEXT    NOT NULL DEFAULT 'commercial',
    volume_tier          TEXT    NOT NULL DEFAULT 'medium',
    est_monthly_volume   INTEGER NOT NULL DEFAULT 1000,
    difficulty           INTEGER NOT NULL DEFAULT 35,
    relevance            INTEGER NOT NULL DEFAULT 90,
    is_primary           INTEGER NOT NULL DEFAULT 0,
    is_locked            INTEGER NOT NULL DEFAULT 0,
    updated_at           DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE(repo_id, locale, keyword)
);

CREATE TABLE IF NOT EXISTS repo_seo_optimizations (
    repo_id                 INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    locale                  TEXT    NOT NULL,
    translation_key         TEXT    NOT NULL,
    source_en               TEXT    NOT NULL DEFAULT '',
    baseline_translation    TEXT    NOT NULL DEFAULT '',
    optimized_translation   TEXT    NOT NULL DEFAULT '',
    injected_keywords_json  TEXT    NOT NULL DEFAULT '[]',
    rationale               TEXT    NOT NULL DEFAULT '',
    impact_tier             TEXT    NOT NULL DEFAULT 'high',
    character_length        INTEGER NOT NULL DEFAULT 0,
    pixel_width_desktop     INTEGER NOT NULL DEFAULT 0,
    is_title_truncated      INTEGER NOT NULL DEFAULT 0,
    icu_variables_matched   INTEGER NOT NULL DEFAULT 1,
    updated_at              DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY(repo_id, locale, translation_key)
);

CREATE TABLE IF NOT EXISTS repo_seo_metrics (
    repo_id                 INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    locale                  TEXT    NOT NULL,
    search_volume_baseline  INTEGER NOT NULL DEFAULT 200,
    search_volume_optimized INTEGER NOT NULL DEFAULT 10000,
    search_volume_uplift_pct REAL   NOT NULL DEFAULT 0.0,
    projected_ctr_baseline  REAL    NOT NULL DEFAULT 1.8,
    projected_ctr_optimized REAL    NOT NULL DEFAULT 4.6,
    projected_ctr_uplift_pct REAL   NOT NULL DEFAULT 0.0,
    avg_keyword_difficulty  INTEGER NOT NULL DEFAULT 35,
    readability_score       INTEGER NOT NULL DEFAULT 94,
    local_trust_score       INTEGER NOT NULL DEFAULT 90,
    keyword_density_pct     REAL    NOT NULL DEFAULT 2.4,
    density_safe            INTEGER NOT NULL DEFAULT 1,
    estimated_ranking_days  INTEGER NOT NULL DEFAULT 45,
    updated_at              DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY(repo_id, locale)
);
