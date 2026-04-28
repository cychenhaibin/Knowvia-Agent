CREATE TABLE IF NOT EXISTS skill_artifact_files (
    id TEXT PRIMARY KEY,
    artifact_id TEXT NOT NULL REFERENCES skill_artifacts(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    media_type TEXT NOT NULL DEFAULT '',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    sha256 TEXT NOT NULL DEFAULT '',
    is_manifest BOOLEAN NOT NULL DEFAULT FALSE,
    is_instructions BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_skill_artifact_files_artifact_path
    ON skill_artifact_files(artifact_id, path ASC);
