-- name: CreateSkillDefinition :execrows
INSERT INTO skill_definitions (
	id, user_id, slug, kind, source, repo_url, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: CreateSkillRevision :execrows
INSERT INTO skill_revisions (
	id, definition_id, version, title, description, prompt, mode, manifest_json, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: CreateSkillInstallation :execrows
INSERT INTO skill_installations (
	id, user_id, definition_id, current_revision_id, name, is_default, enabled, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: GetInstallationRevisionTitle :one
SELECT sr.title
FROM skill_definitions sd
JOIN skill_revisions sr ON sr.id = $2 AND sr.definition_id = sd.id
WHERE sd.id = $1 AND sd.user_id = $3;

-- name: CountDefinitionInstallations :one
SELECT COUNT(1)
FROM skill_installations
WHERE user_id = $1 AND definition_id = $2;

-- name: ClearDefinitionDefaultInstallations :execrows
UPDATE skill_installations
SET is_default = FALSE
WHERE user_id = $1 AND definition_id = $2 AND is_default = TRUE;

-- name: UpdateSkillDefinition :execrows
UPDATE skill_definitions
SET slug = $2,
    kind = $3,
    source = $4,
    repo_url = $5,
    updated_at = $6
WHERE id = $1 AND user_id = $7;

-- name: UpdateSkillInstallationRevision :execrows
UPDATE skill_installations
SET current_revision_id = $2,
    enabled = $3,
    updated_at = $4
WHERE id = $1 AND user_id = $5;

-- name: GetInstallationUpdateContext :one
SELECT si.definition_id, sr.title, si.is_default
FROM skill_installations si
JOIN skill_revisions sr ON sr.id = $2 AND sr.definition_id = si.definition_id
WHERE si.id = $1 AND si.user_id = $3;

-- name: SelectPromoteSiblingInstallation :one
SELECT id
FROM skill_installations
WHERE user_id = $1 AND definition_id = $2 AND id <> $3
ORDER BY updated_at DESC, created_at ASC, id ASC
LIMIT 1;

-- name: ClearOtherDefaultInstallations :execrows
UPDATE skill_installations
SET is_default = FALSE
WHERE user_id = $1 AND definition_id = $2 AND id <> $3 AND is_default = TRUE;

-- name: UpdateSkillInstallation :execrows
UPDATE skill_installations si
SET current_revision_id = $2,
    name = $3,
    is_default = $4,
    enabled = $5,
    updated_at = $6
WHERE si.id = $1
  AND si.user_id = $7
  AND EXISTS (
	SELECT 1
	FROM skill_revisions sr
	WHERE sr.id = $2
	  AND sr.definition_id = si.definition_id
  );

-- name: PromoteSkillInstallationDefault :execrows
UPDATE skill_installations
SET is_default = TRUE
WHERE id = $1;

-- name: GetDeletedSkillContext :one
SELECT definition_id, is_default
FROM skill_installations
WHERE id = $1 AND user_id = $2;

-- name: DeleteSkillInstallation :execrows
DELETE FROM skill_installations
WHERE id = $1 AND user_id = $2;

-- name: PromoteLatestSkillInstallation :execrows
UPDATE skill_installations
SET is_default = TRUE
WHERE id = (
	SELECT id
	FROM skill_installations
	WHERE skill_installations.user_id = $1 AND skill_installations.definition_id = $2
	ORDER BY updated_at DESC, created_at ASC, id ASC
	LIMIT 1
);

-- name: GetSkill :one
SELECT si.id, si.user_id, sd.id, sr.id, sr.version, sd.slug, sd.kind,
       sr.title, sr.description, sr.prompt, sr.mode, sr.manifest_json, sd.source, si.enabled,
       sd.repo_url, si.created_at, si.updated_at
FROM skill_installations si
JOIN skill_definitions sd ON sd.id = si.definition_id
JOIN skill_revisions sr ON sr.id = si.current_revision_id
WHERE si.id = $1 AND si.user_id = $2;

-- name: ListSkills :many
SELECT si.id, si.user_id, sd.id, sr.id, sr.version, sd.slug, sd.kind,
       sr.title, sr.description, sr.prompt, sr.mode, sr.manifest_json, sd.source, si.enabled,
       sd.repo_url, si.created_at, si.updated_at
FROM skill_installations si
JOIN skill_definitions sd ON sd.id = si.definition_id
JOIN skill_revisions sr ON sr.id = si.current_revision_id
WHERE si.user_id = $1
ORDER BY si.updated_at DESC;

-- name: GetSkillInstallationRecord :one
SELECT si.id, si.user_id, si.definition_id, si.current_revision_id, si.name, si.is_default, si.enabled, si.created_at, si.updated_at,
       sd.id, sd.user_id, sd.slug, sd.kind, sd.source, sd.repo_url, sd.created_at, sd.updated_at,
       sr.id, sr.definition_id, sr.version, sr.title, sr.description, sr.prompt, sr.mode, sr.manifest_json, sr.created_at
FROM skill_installations si
JOIN skill_definitions sd ON sd.id = si.definition_id
JOIN skill_revisions sr ON sr.id = si.current_revision_id
WHERE si.id = $1 AND si.user_id = $2;

-- name: GetDefaultSkillInstallationRecord :one
SELECT si.id, si.user_id, si.definition_id, si.current_revision_id, si.name, si.is_default, si.enabled, si.created_at, si.updated_at,
       sd.id, sd.user_id, sd.slug, sd.kind, sd.source, sd.repo_url, sd.created_at, sd.updated_at,
       sr.id, sr.definition_id, sr.version, sr.title, sr.description, sr.prompt, sr.mode, sr.manifest_json, sr.created_at
FROM skill_installations si
JOIN skill_definitions sd ON sd.id = si.definition_id
JOIN skill_revisions sr ON sr.id = si.current_revision_id
WHERE si.user_id = $1 AND si.definition_id = $2
ORDER BY si.is_default DESC, si.updated_at DESC
LIMIT 1;

-- name: ListSkillInstallations :many
SELECT si.id, si.user_id, si.definition_id, si.current_revision_id, si.name, si.is_default, si.enabled, si.created_at, si.updated_at,
       sd.id, sd.user_id, sd.slug, sd.kind, sd.source, sd.repo_url, sd.created_at, sd.updated_at,
       sr.id, sr.definition_id, sr.version, sr.title, sr.description, sr.prompt, sr.mode, sr.manifest_json, sr.created_at
FROM skill_installations si
JOIN skill_definitions sd ON sd.id = si.definition_id
JOIN skill_revisions sr ON sr.id = si.current_revision_id
WHERE si.user_id = $1
ORDER BY si.updated_at DESC;

-- name: ListSkillDefinitions :many
SELECT id, user_id, slug, kind, source, repo_url, created_at, updated_at
FROM skill_definitions
WHERE user_id = $1
ORDER BY updated_at DESC;

-- name: GetSkillDefinition :one
SELECT id, user_id, slug, kind, source, repo_url, created_at, updated_at
FROM skill_definitions
WHERE id = $1 AND user_id = $2;

-- name: ListDefinitionInstallations :many
SELECT id, user_id, definition_id, current_revision_id, name, is_default, enabled, created_at, updated_at
FROM skill_installations
WHERE user_id = $1 AND definition_id = $2
ORDER BY is_default DESC, updated_at DESC;

-- name: EnsureSkillDefinitionExists :one
SELECT 1
FROM skill_definitions
WHERE id = $1 AND user_id = $2;

-- name: ListSkillRevisions :many
SELECT sr.id, sr.definition_id, sr.version, sr.title, sr.description, sr.prompt, sr.mode, sr.manifest_json, sr.created_at
FROM skill_revisions sr
JOIN skill_definitions sd ON sd.id = sr.definition_id
WHERE sr.definition_id = $1 AND sd.user_id = $2
ORDER BY sr.version DESC, sr.created_at DESC;

-- name: CreateSkillRuntimeSnapshot :execrows
INSERT INTO skill_runtime_snapshots (
	id, scope, scope_id, user_id, installation_id, definition_id, revision_id,
	kind, title, description, mode, prompt, runtime_spec_json, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);

-- name: CreateSkillArtifact :execrows
INSERT INTO skill_artifacts (
	id, user_id, definition_id, revision_id, source, file_name, media_type, source_url,
	sha256, size_bytes, entry_path, manifest_path, instructions_path, archive_bytes, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15);

-- name: GetSkillArtifact :one
SELECT id, user_id, definition_id, revision_id, source, file_name, media_type, source_url,
       sha256, size_bytes, entry_path, manifest_path, instructions_path, archive_bytes, created_at
FROM skill_artifacts
WHERE id = $1 AND user_id = $2;

-- name: GetSkillArtifactUserId :one
SELECT user_id
FROM skill_artifacts
WHERE id = $1;

-- name: DeleteSkillArtifactFiles :execrows
DELETE FROM skill_artifact_files
WHERE artifact_id = $1;

-- name: InsertSkillArtifactFile :execrows
INSERT INTO skill_artifact_files (
	id, artifact_id, user_id, path, media_type, size_bytes, sha256,
	is_manifest, is_instructions, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ListSkillArtifactFiles :many
SELECT saff.id, saff.artifact_id, saff.user_id, saff.path, saff.media_type, saff.size_bytes,
       saff.sha256, saff.is_manifest, saff.is_instructions, saff.created_at
FROM skill_artifact_files saff
JOIN skill_artifacts sa ON sa.id = saff.artifact_id
WHERE saff.artifact_id = $1 AND sa.user_id = $2
ORDER BY saff.path ASC;

-- name: CreateSkillImportJob :execrows
INSERT INTO skill_import_jobs (
	id, user_id, source, status, artifact_id, definition_id, revision_id, installation_id,
	error_message, request_json, created_at, updated_at, completed_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: UpdateSkillImportJob :execrows
UPDATE skill_import_jobs
SET source = $2,
    status = $3,
    artifact_id = $4,
    definition_id = $5,
    revision_id = $6,
    installation_id = $7,
    error_message = $8,
    request_json = $9,
    updated_at = $10,
    completed_at = $11
WHERE id = $1 AND user_id = $12;

-- name: GetSkillImportJob :one
SELECT id, user_id, source, status, artifact_id, definition_id, revision_id, installation_id,
       error_message, request_json, created_at, updated_at, completed_at
FROM skill_import_jobs
WHERE id = $1 AND user_id = $2;

-- name: ListSkillImportJobs :many
SELECT id, user_id, source, status, artifact_id, definition_id, revision_id, installation_id,
       error_message, request_json, created_at, updated_at, completed_at
FROM skill_import_jobs
WHERE user_id = $1
ORDER BY updated_at DESC, created_at DESC;
