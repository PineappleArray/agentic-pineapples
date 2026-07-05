package types

import "time"

// Normalized data is here

type Source string

const (
	SourceAppStoreReview Source = "appstore_review"
	SourceEmail          Source = "email"
	SourceSlack          Source = "slack"        // Slack not included in MVP
	SourceSocialMedia    Source = "social_media" // Social Media not included in MVP
	// SourceAppleNotification Source = "apple_notification" // stretch: CONSUMPTION_REQUEST webhook
)

// Event is the normalized unit of work produced by ingestion.
type Event struct {
	ID         string            // internal UUID, assigned at ingestion
	Source     string            // "app_store" | "gmail" | "slack"
	ExternalID string            // dedup key, unique per source (see below)
	Author     string            // best-available identity
	Text       string            // the content rules/extraction care about
	Timestamp  time.Time         // when it happened at the source (not ingestion time)
	Rating     *int              // nil for non-review sources
	Metadata   map[string]string // source-specific, preserved for audit/UI
	Derived    map[string]any    // enrichment layer writes here (counts, flags)
	Extracted  *Extraction       // LLM output, attached later; nil = not yet / failed
}

// ~~~~~~~~~~~~~~~~~~~~~~~Extraction: the LLM's structured output

type Sentiment string

const (
	SentimentPositive Sentiment = "positive"
	SentimentNeutral  Sentiment = "neutral"
	SentimentNegative Sentiment = "negative"
)

type Category string

const (
	CategoryBug            Category = "bug"
	CategoryBilling        Category = "billing"
	CategoryFeatureRequest Category = "feature_request"
	CategoryAccount        Category = "account"
	CategoryPraise         Category = "praise"
	CategoryOther          Category = "other"
)

// Extraction is what any Extractor backend (API, Ollama, mock) must return.
// Every field is validated against these enums before an Event reaches the
// engine — a hallucinated category is a validation error, not a new value.
type Extraction struct {
	Sentiment       Sentiment `json:"sentiment"`
	Category        Category  `json:"category"`
	Severity        int       `json:"severity"` // 1 (low) .. 5 (critical)
	RefundRequested bool      `json:"refund_requested"`
	Features        []string  `json:"features,omitempty"` // mentioned feature names
	Language        string    `json:"language"`           // BCP 47, e.g. "en"
	Confidence      float64   `json:"confidence"`         // 0..1, model self-report
	Raw             string    `json:"raw"`                // exact model output, logged for replay
	Model           string    `json:"model"`              // which backend/model produced this
}

// ~~~~~~~~~~~~~~~~~~~~~~~~~Rules Engine and Decision struct

type Outcome string

// Severity-ordered lattice: Escalate > Draft > AutoRespond > Ignore.
const (
	OutcomeEscalate    Outcome = "escalate"
	OutcomeDraft       Outcome = "draft"
	OutcomeAutoRespond Outcome = "auto_respond"
	OutcomeIgnore      Outcome = "ignore"
)

// OutcomeRank returns the lattice position (higher = more conservative wins).
func OutcomeRank(o Outcome) int {
	switch o {
	case OutcomeEscalate:
		return 3
	case OutcomeDraft:
		return 2
	case OutcomeAutoRespond:
		return 1
	default:
		return 0
	}
}

type Rule struct {
	ID        string  `yaml:"id" json:"id"`
	Priority  int     `yaml:"priority" json:"priority"`
	Condition string  `yaml:"condition" json:"condition"` // CEL source, compiled at load
	Outcome   Outcome `yaml:"outcome" json:"outcome"`
	// Escalation urgency (p1-p4)
	// Priority above, which orders rule evaluation, not humans
	Urgency Priority `yaml:"urgency,omitempty" json:"urgency,omitempty"`
	Version string   `yaml:"version,omitempty" json:"version,omitempty"`
}

// Decision is the engine's output. It records enough to answer
// "why did this happen" forever: the matched rules and the exact ruleset.
type Decision struct {
	EventID      string    `json:"event_id"`
	Outcome      Outcome   `json:"outcome"`
	Urgency      Priority  `json:"urgency,omitempty"`     // set when Outcome == escalate; most urgent among matched rules wins
	MatchedRules []string  `json:"matched_rules"`         // every rule that fired, sorted
	RulesetHash  string    `json:"ruleset_hash"`          // content hash of the loaded ruleset
	RuleErrors   []string  `json:"rule_errors,omitempty"` // rules whose CEL eval errored (treated as no-match)
	DecidedAt    time.Time `json:"decided_at"`            // metadata only — never an input to evaluation
}

// Escalation Queue (Triage of the most important events to address)

// Priority is human-facing urgency, distinct from Rule.Priority (which
// orders rule evaluation). Converted to a concrete Deadline at enqueue
// time so there is no starvation
type Priority string

const (
	PriorityP1 Priority = "p1" // critical: data loss, legal, security
	PriorityP2 Priority = "p2"
	PriorityP3 Priority = "p3"
	PriorityP4 Priority = "p4" // low: mild feature requests
)

// SLA maps urgency to a response time promise. Takes into account the time when user
// is away from the screen to respond
func SLA(p Priority) time.Duration {
	switch p {
	case PriorityP1:
		return 4 * time.Hour
	case PriorityP2:
		return 24 * time.Hour
	case PriorityP3:
		return 48 * time.Hour
	default:
		return 72 * time.Hour
	}
}

type QueueStatus string

const (
	QueuePending  QueueStatus = "pending"
	QueueClaimed  QueueStatus = "claimed" // atomically claimed by one user
	QueueResolved QueueStatus = "resolved"
)

// QueueItem is written in the SAME SQLite transaction as its Decision when
// Outcome == escalate — a crash must never lose an escalation. Claiming is
// atomic: UPDATE ... WHERE id=? AND status='pending', check rows affected.
type QueueItem struct {
	ID        string      `json:"id"`
	EventID   string      `json:"event_id"`
	Urgency   Priority    `json:"urgency"` // from the Decision, recorded for audit
	Status    QueueStatus `json:"status"`
	Deadline  time.Time   `json:"deadline"` // CreatedAt + SLA(Urgency); queue orders by this, ascending
	ClaimedBy string      `json:"claimed_by,omitempty"`
	ClaimedAt *time.Time  `json:"claimed_at,omitempty"` // pointer: unclaimed ≠ claimed-at-zero-time
	CreatedAt time.Time   `json:"created_at"`
}

// Outcome of the decision

type ActionKind string

const (
	ActionPostReviewResponse ActionKind = "post_review_response"
	ActionSendEmail          ActionKind = "send_email"
	ActionNotifyHuman        ActionKind = "notify_human"
)

type ActionStatus string

const (
	ActionPending          ActionStatus = "pending"           // in the outbox, not yet executed
	ActionAwaitingApproval ActionStatus = "awaiting_approval" // draft tier, human must approve
	ActionExecuted         ActionStatus = "executed"
	ActionRejected         ActionStatus = "rejected"
	ActionFailed           ActionStatus = "failed"
)

// Action is an outbox row: written in the same SQLite transaction as its
// Decision, executed by a worker keyed on IdempotencyKey.
type Action struct {
	ID             string       `json:"id"`
	EventID        string       `json:"event_id"`
	Kind           ActionKind   `json:"kind"`
	Body           string       `json:"body"` // drafted reply text, notification payload
	Status         ActionStatus `json:"status"`
	IdempotencyKey string       `json:"idempotency_key"`
	CreatedAt      time.Time    `json:"created_at"`
	ExecutedAt     *time.Time   `json:"executed_at,omitempty"`
}

type DeadLetter struct {
	RawPayload []byte
	Error      string
	Attempts   int
	ResolvedAt *time.Time
}
