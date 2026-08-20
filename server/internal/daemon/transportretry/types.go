package transportretry

// SessionRetryMode names how the next launch should bind to a provider session.
type SessionRetryMode string

const (
	SessionRetrySame  SessionRetryMode = "same_session"
	SessionRetryFresh SessionRetryMode = "fresh_session"
	SessionRetryCold  SessionRetryMode = "cold"
)

// Stats summarizes in-turn transport retries for logging, metrics, and optional receipts.
type Stats struct {
	PolicyID                 string             `json:"policy_id,omitempty"`
	Attempts                 int                `json:"attempts,omitempty"`
	RecoveredOnAttempt       int                `json:"recovered_on_attempt,omitempty"`
	SessionModes             []SessionRetryMode `json:"session_modes,omitempty"`
	CacheReadTokensFirst     int64              `json:"cache_read_tokens_first,omitempty"`
	CacheReadTokensRecovered int64              `json:"cache_read_tokens_recovered,omitempty"`
	SurfacedToServer         bool               `json:"surfaced_to_server,omitempty"`
	SkippedDueToTools        bool               `json:"skipped_due_to_tools,omitempty"`
	ExtraWallSeconds         float64            `json:"extra_wall_seconds,omitempty"`
}

// ExecuteFunc runs one backend launch and drain cycle.
type ExecuteFunc func(prompt string, opts ExecOptionsView) (ResultView, int32, error)

// ExecOptionsView is the slice of agent.ExecOptions the retry executor mutates.
type ExecOptionsView struct {
	ResumeSessionID string
}

// ResultView is the slice of agent.Result the retry executor reads.
type ResultView struct {
	Status         string
	Output         string
	Error          string
	DurationMs     int64
	SessionID      string
	Usage          map[string]TokenUsageView
	ResumeRejected bool
}

// TokenUsageView carries billing fields the retry executor must preserve.
type TokenUsageView struct {
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	CostUSDTicks     int64
}

// RetryHooks supplies daemon-specific fresh-session recovery.
type RetryHooks struct {
	OnFreshSession func(opts *ExecOptionsView) (newPrompt string, retiredSessionID string)
}
