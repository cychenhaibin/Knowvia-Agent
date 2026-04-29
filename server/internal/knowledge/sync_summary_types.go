package knowledge

type incrementalSyncSummary struct {
	Kind           string
	Documents      int
	Chunks         int
	Added          int
	Updated        int
	Deleted        int
	Unchanged      int
	Deferred       int
	RetryAfterSec  int
	AddedTitles    []string
	UpdatedTitles  []string
	DeletedTitles  []string
	DeferredTitles []string
	SyncedTitles   []string
}

type syncErrorSummary struct {
	Kind     string
	Provider string
	Code     string
	Message  string
	LogID    string
}
