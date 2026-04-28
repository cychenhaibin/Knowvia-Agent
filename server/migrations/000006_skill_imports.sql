CREATE TABLE IF NOT EXISTS skill_artifacts (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    definition_id TEXT NOT NULL DEFAULT '',
    revision_id TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL,
    file_name TEXT NOT NULL,
    media_type TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    sha256 TEXT NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    entry_path TEXT NOT NULL DEFAULT '',
    manifest_path TEXT NOT NULL DEFAULT '',
    instructions_path TEXT NOT NULL DEFAULT '',
    archive_bytes BYTEA NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS skill_import_jobs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    status TEXT NOT NULL,
    artifact_id TEXT NOT NULL DEFAULT '',
    definition_id TEXT NOT NULL DEFAULT '',
    revision_id TEXT NOT NULL DEFAULT '',
    installation_id TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    request_json TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_skill_artifacts_user_created
    ON skill_artifacts(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_skill_import_jobs_user_updated
    ON skill_import_jobs(user_id, updated_at DESC);
