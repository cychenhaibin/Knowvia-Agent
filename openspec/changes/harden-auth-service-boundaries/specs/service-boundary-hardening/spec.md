## ADDED Requirements

### Requirement: Authentication tokens have bounded and distinct lifetimes
The service SHALL issue access and refresh tokens with distinct token-use claims, unique identifiers, configured expiration times, and matching response TTLs. Access authentication MUST reject refresh tokens and revoked sessions.

#### Scenario: Login issues bounded tokens
- **WHEN** a user logs in with valid credentials
- **THEN** the access token expires at the configured access TTL, the refresh token expires at the configured refresh TTL, and the two tokens carry different token-use values and identifiers

#### Scenario: Logout revokes access authentication
- **WHEN** a user logs out with a valid refresh token and then presents the associated access token
- **THEN** authentication fails with an invalid-token error

### Requirement: Production configuration fails closed
The service SHALL reject known development JWT secrets, implicit default administrator credentials, and externally bound default listeners unless explicit development mode is enabled.

#### Scenario: Missing production secret is rejected
- **WHEN** the service loads configuration without an explicit production JWT secret
- **THEN** configuration validation returns an error before serving requests

### Requirement: File-backed scope identifiers are confined
The Python file backend SHALL reject absolute paths, traversal components, separators, and symlink escapes in user and scope identifiers before any filesystem operation.

#### Scenario: Traversal deletion is rejected
- **WHEN** a delete request supplies `..` or an absolute user/scope identifier
- **THEN** the repository raises a validation error and leaves files outside the configured base directory untouched

### Requirement: External archive imports are bounded and restricted
GitHub imports SHALL accept only HTTPS GitHub repository URLs and safe redirects. ZIP processing SHALL enforce member-count, cumulative uncompressed-size, and compression-ratio limits.

#### Scenario: Private GitHub target is rejected
- **WHEN** an import URL targets localhost, a private address, a non-HTTPS scheme, or a non-GitHub host
- **THEN** the request is rejected without making the external fetch

#### Scenario: Archive expansion limit is enforced
- **WHEN** an archive exceeds any configured member, cumulative-size, or ratio limit
- **THEN** import fails with an archive-limit error before retaining all file contents

### Requirement: Stream and run detail responses report incomplete data
The Go Python proxy SHALL fail a stream that ends without a done event, and run detail assembly SHALL return sub-resource read errors instead of silently returning partial success.

#### Scenario: Truncated stream fails
- **WHEN** the upstream closes a chat stream without a done event
- **THEN** the proxy returns an incomplete-stream error

#### Scenario: Run detail dependency failure propagates
- **WHEN** reading steps, artifacts, or sources fails
- **THEN** GetRunDetails returns that error and does not report a successful complete response

### Requirement: File-backed indexing and chunking preserve boundaries
The Python file backend SHALL serialize job/scope writes atomically, and chunking SHALL never emit a chunk larger than the configured chunk size.

#### Scenario: Concurrent job updates are preserved
- **WHEN** two indexing workers update different jobs concurrently
- **THEN** both updates remain readable and no partial JSON is observed

#### Scenario: Long paragraph is split
- **WHEN** a paragraph exceeds the configured chunk size
- **THEN** it is split into chunks whose lengths do not exceed that size

### Requirement: Redis task failure transitions are recoverable
Redis workers SHALL atomically move a failed processing task to retry or failed storage so an interruption cannot leave it in neither location.

#### Scenario: Retry move is atomic
- **WHEN** the worker handles a retryable task and Redis fails during the transition
- **THEN** the task remains in processing or appears in retry, and its deduplication state does not permanently suppress recovery
