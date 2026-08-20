package daemon

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/daemon/transportretry"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

func TestMarshalTransportRetryReceipt_TopLevelFields(t *testing.T) {
	t.Parallel()

	stats := transportretry.Stats{
		PolicyID:           "cursor_writable_iterable",
		Attempts:           3,
		RecoveredOnAttempt: 3,
		SessionModes:       []transportretry.SessionRetryMode{transportretry.SessionRetrySame, transportretry.SessionRetrySame},
		SurfacedToServer:   false,
	}
	raw := marshalTransportRetryReceipt(stats)
	if len(raw) == 0 {
		t.Fatal("expected receipt JSON")
	}

	var receipt map[string]any
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatalf("unmarshal receipt: %v", err)
	}
	if _, nested := receipt["transport_retry"]; nested {
		t.Fatalf("receipt must not double-wrap transport_retry, got %s", raw)
	}
	if receipt["policy_id"] != "cursor_writable_iterable" {
		t.Fatalf("policy_id = %v, want cursor_writable_iterable", receipt["policy_id"])
	}
	if attempts, _ := receipt["attempts"].(float64); attempts != 3 {
		t.Fatalf("attempts = %v, want 3", receipt["attempts"])
	}
}

func TestHandleTask_PersistsTransportRetryReceiptToGCMeta(t *testing.T) {
	t.Parallel()

	workspacesRoot := t.TempDir()
	workspaceID := "ws-transport-retry-gcmeta"
	taskID := "task-transport-retry-gcmeta"
	counterFile := filepath.Join(t.TempDir(), "cursor-launch-count")
	expectedEnvRoot := execenv.PredictRootDir(workspacesRoot, workspaceID, taskID)

	fakeBin := filepath.Join(t.TempDir(), "cursor-agent")
	script := `#!/bin/sh
cat > /dev/null
countfile="$COUNTER_FILE"
n=0
[ -f "$countfile" ] && n=$(cat "$countfile")
n=$((n + 1))
echo "$n" > "$countfile"
if [ "$n" -lt 3 ]; then
	printf '%s\n' '{"type":"thinking","subtype":"delta"}'
	echo "RetriableError: WritableIterable is closed" >&2
	exit 1
fi
printf '%s\n' '{"type":"system","subtype":"init","session_id":"sess-transport-retry"}'
printf '%s\n' '{"type":"result","subtype":"success","is_error":false,"result":"ok","session_id":"sess-transport-retry"}'
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake cursor-agent: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	d := &Daemon{
		client:             NewClient(srv.URL),
		logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		workspaces:         make(map[string]*workspaceState),
		runtimeIndex:       map[string]Runtime{"rt-cursor": {ID: "rt-cursor", Provider: "cursor"}},
		activeEnvRoots:     make(map[string]int),
		cancelPollInterval: time.Hour,
		cfg: Config{
			WorkspacesRoot: workspacesRoot,
			AgentTimeout:   10 * time.Second,
			ServerBaseURL:  srv.URL,
			Agents: map[string]AgentEntry{
				"cursor": {Path: fakeBin, Model: ""},
			},
		},
	}
	d.runner = taskRunnerFunc(d.runTask)

	retryConfig := `{"policies":[{"id":"cursor_writable_iterable","delays_ms":[0,0,0]}]}`
	task := Task{
		ID:          taskID,
		WorkspaceID: workspaceID,
		RuntimeID:   "rt-cursor",
		IssueID:     "issue-transport-retry-gcmeta",
		AuthToken:   "mat_transport_retry_gcmeta",
		Agent: &AgentData{
			ID:   "agent-transport-retry-gcmeta",
			Name: "test-agent",
			CustomEnv: map[string]string{
				"COUNTER_FILE":                   counterFile,
				"MULTICA_TRANSPORT_RETRY_CONFIG": retryConfig,
			},
		},
	}

	d.handleTask(context.Background(), task, 0)

	meta, err := execenv.ReadGCMeta(expectedEnvRoot)
	if err != nil {
		t.Fatalf("ReadGCMeta: %v", err)
	}
	if len(meta.TransportRetry) == 0 {
		t.Fatal("expected transport_retry receipt in .gc_meta.json")
	}

	var receipt map[string]any
	if err := json.Unmarshal(meta.TransportRetry, &receipt); err != nil {
		t.Fatalf("unmarshal gc meta transport_retry: %v", err)
	}
	if _, nested := receipt["transport_retry"]; nested {
		t.Fatalf("gc meta receipt must not double-wrap transport_retry, got %s", meta.TransportRetry)
	}
	if receipt["policy_id"] != "cursor_writable_iterable" {
		t.Fatalf("policy_id = %v, want cursor_writable_iterable", receipt["policy_id"])
	}
	if attempts, _ := receipt["attempts"].(float64); attempts < 2 {
		t.Fatalf("attempts = %v, want >= 2 after in-turn recovery", receipt["attempts"])
	}
}

func TestAgentResultViewRoundTripPreservesBillingFields(t *testing.T) {
	t.Parallel()

	original := agent.Result{
		Status:         "failed",
		Output:         "out",
		Error:          "WritableIterable is closed",
		DurationMs:     1200,
		SessionID:      "sess-1",
		ResumeRejected: true,
		Usage: map[string]agent.TokenUsage{
			"m": {
				InputTokens:      10,
				OutputTokens:     5,
				CacheReadTokens:  3,
				CacheWriteTokens: 2,
				CostUSDTicks:     900,
			},
		},
	}

	view := agentResultView(original)
	roundTrip := agentResultFromView(view)
	if roundTrip.Status != original.Status || roundTrip.Output != original.Output ||
		roundTrip.Error != original.Error || roundTrip.DurationMs != original.DurationMs ||
		roundTrip.SessionID != original.SessionID || roundTrip.ResumeRejected != original.ResumeRejected {
		t.Fatalf("round trip metadata = %+v, want %+v", roundTrip, original)
	}
	if len(roundTrip.Usage) != len(original.Usage) {
		t.Fatalf("usage len = %d, want %d", len(roundTrip.Usage), len(original.Usage))
	}
	for model, want := range original.Usage {
		got := roundTrip.Usage[model]
		if got != want {
			t.Fatalf("usage[%s] = %+v, want %+v", model, got, want)
		}
	}
}

func TestClassifyAgentFailureReasonAfterToolsSkipsProviderNetworkAutoRetry(t *testing.T) {
	t.Parallel()

	errMsg := "RetriableError: WritableIterable is closed (result_seen=false)"
	stats := transportretry.Stats{
		PolicyID:         "cursor_writable_iterable",
		SkippedDueToTools: true,
	}
	reason := classifyAgentFailureReason(errMsg, stats)
	if reason != taskfailure.ReasonAgentUnknown.String() {
		t.Fatalf("reason = %q, want %q", reason, taskfailure.ReasonAgentUnknown)
	}
	if service.AutoRetryEligible(reason, 1, 2) {
		t.Fatal("transport failure after tool use must not be server auto-retry eligible")
	}

	networkReason := classifyAgentFailureReason(errMsg, transportretry.Stats{})
	if networkReason != taskfailure.ReasonAgentProviderNetwork.String() {
		t.Fatalf("plain classify = %q, want provider_network", networkReason)
	}
	if !service.AutoRetryEligible(networkReason, 1, 2) {
		t.Fatal("provider_network without tools should stay auto-retry eligible")
	}
}
