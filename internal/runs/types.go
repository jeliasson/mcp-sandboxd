package runs

import (
	"time"
)

type State string

const (
	StateQueued    State = "queued"
	StateRunning   State = "running"
	StateCompleted State = "completed"
	StateFailed    State = "failed"
	StateCanceled  State = "canceled"
)

type CommandStatus string

const (
	CommandQueued    CommandStatus = "queued"
	CommandCompleted CommandStatus = "completed"
	CommandFailed    CommandStatus = "failed"
	CommandTimedOut  CommandStatus = "timed_out"
)

type Run struct {
	RunID      string    `json:"run_id"`
	Identifier string    `json:"identifier"`
	State      State     `json:"state"`
	CreatedAt  time.Time `json:"created_at"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	EndedAt    time.Time `json:"ended_at,omitempty"`

	SandboxContainerID string `json:"sandbox_container_id,omitempty"`
	SandboxName        string `json:"sandbox_name,omitempty"`

	Commands  []*CommandResult `json:"commands"`
	Artifacts []ArtifactFile   `json:"artifacts"`

	ArtifactsError string `json:"artifacts_error,omitempty"`
	Error          string `json:"error,omitempty"`

	Done chan struct{} `json:"-"`
}

type CommandResult struct {
	Index int `json:"index"`

	Shell string   `json:"shell,omitempty"`
	Argv  []string `json:"argv,omitempty"`

	Cwd string            `json:"cwd"`
	Env map[string]string `json:"env,omitempty"`

	Status   CommandStatus `json:"status"`
	ExitCode int           `json:"exit_code"`

	StartedAt time.Time `json:"started_at,omitempty"`
	EndedAt   time.Time `json:"ended_at,omitempty"`

	StdoutBytes     int    `json:"stdout_bytes"`
	StderrBytes     int    `json:"stderr_bytes"`
	StdoutTruncated bool   `json:"stdout_truncated"`
	StderrTruncated bool   `json:"stderr_truncated"`
	Stdout          string `json:"stdout,omitempty"`
	Stderr          string `json:"stderr,omitempty"`
}

type ArtifactFile struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type RunSandboxArgs struct {
	Identifier string         `json:"identifier"`
	Commands   []CommandInput `json:"commands"`
	Options    RunOptions     `json:"options"`
}

type CommandInput struct {
	Shell OptionalString `json:"shell,omitempty"`
	Argv  []string       `json:"argv,omitempty"`
	Cwd   string         `json:"cwd,omitempty"`
	Env   EnvVars        `json:"env,omitempty"`

	TimeoutMS    int  `json:"timeout_ms,omitempty"`
	AllowFailure bool `json:"allow_failure,omitempty"`
}

type RunOptions struct {
	TTLSeconds int `json:"ttl_seconds,omitempty"`

	DefaultCwd string  `json:"default_cwd,omitempty"`
	DefaultEnv EnvVars `json:"default_env,omitempty"`

	AsUser string `json:"as_user,omitempty"` // "root" or "sandbox"

	ContinueOnError bool `json:"continue_on_error,omitempty"`
	LockSandbox     bool `json:"lock_sandbox,omitempty"`

	AwaitCompletion *bool `json:"await_completion,omitempty"`
	AwaitTimeoutMS  *int  `json:"await_timeout_ms,omitempty"`
}
