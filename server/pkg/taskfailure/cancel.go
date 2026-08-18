package taskfailure

// Cancel reasons are written to agent_task_queue.failure_reason when the server
// flips a task to status=cancelled. They are distinct from agent-side failures
// (agent_error.*) and from the legacy daemon coarse bucket "cancelled", which
// only appears when the daemon reports a poll-driven cancellation.
//
// Wire stability: these strings are persisted and surfaced in orchestration
// audits (dispatch-accuracy.sh, orchestration-audit.py). Add new values freely;
// do not rename without a backfill.
const cancelPrefix = "cancel."

type CancelReason string

const (
	// ReasonCancelUser: explicit user/API cancellation (Stop button, cancel-task).
	ReasonCancelUser CancelReason = "cancel.user"

	// ReasonCancelSupersededRerun: manual rerun cancelled the assignee's prior attempt.
	ReasonCancelSupersededRerun CancelReason = "cancel.superseded_by_rerun"

	// ReasonCancelTerminalFence: live attempt on an already-terminal issue (driver-tick R1).
	ReasonCancelTerminalFence CancelReason = "cancel.terminal_fence"

	// ReasonCancelPrematureStage: attempt jumped ahead of an open stage barrier (driver-tick).
	ReasonCancelPrematureStage CancelReason = "cancel.premature_stage_barrier"

	// ReasonCancelIssueDeleted: issue deletion swept active tasks.
	ReasonCancelIssueDeleted CancelReason = "cancel.issue_deleted"

	// ReasonCancelAgentBulk: bulk "cancel all tasks" on an agent.
	ReasonCancelAgentBulk CancelReason = "cancel.agent_bulk"

	// ReasonCancelTriggerStale: trigger comment edited/deleted while task was active.
	ReasonCancelTriggerStale CancelReason = "cancel.trigger_stale"

	// ReasonCancelChatSession: chat session archived/deleted.
	ReasonCancelChatSession CancelReason = "cancel.chat_session"

	// ReasonCancelRuntimeTeardown: runtime unbind/revoke/offline sweep.
	ReasonCancelRuntimeTeardown CancelReason = "cancel.runtime_teardown"

	// ReasonCancelClaimInvalid: claim-time validation failed (empty chat input, etc.).
	ReasonCancelClaimInvalid CancelReason = "cancel.claim_invalid"

	// ReasonCancelPlatform: internal cancellation with no more specific classifier.
	ReasonCancelPlatform CancelReason = "cancel.platform"

	// ReasonCancelledLegacy: daemon-reported cancellation (poll / parent ctx). Kept for
	// parity with daemon/daemon.go reportTaskResult.
	ReasonCancelledLegacy CancelReason = "cancelled"
)

func (r CancelReason) String() string { return string(r) }

// CancelSummary returns a short human-readable line stored in trigger_summary when
// the row had none at enqueue time (assignment-triggered runs). Matches the shape
// admission rejections use for clustering in orchestration audits.
func CancelSummary(reason string) string {
	switch reason {
	case string(ReasonCancelUser):
		return "Cancelled by user"
	case string(ReasonCancelSupersededRerun):
		return "Cancelled: superseded by rerun"
	case string(ReasonCancelTerminalFence):
		return "Cancelled: issue already terminal"
	case string(ReasonCancelPrematureStage):
		return "Cancelled: premature stage barrier"
	case string(ReasonCancelIssueDeleted):
		return "Cancelled: issue deleted"
	case string(ReasonCancelAgentBulk):
		return "Cancelled: agent bulk cancel"
	case string(ReasonCancelTriggerStale):
		return "Cancelled: trigger comment stale"
	case string(ReasonCancelChatSession):
		return "Cancelled: chat session closed"
	case string(ReasonCancelRuntimeTeardown):
		return "Cancelled: runtime teardown"
	case string(ReasonCancelClaimInvalid):
		return "Cancelled: invalid claim"
	case string(ReasonCancelPlatform):
		return "Cancelled by platform"
	case string(ReasonCancelledLegacy):
		return "Cancelled"
	default:
		if reason != "" {
			return "Cancelled: " + reason
		}
		return "Cancelled (reason unknown)"
	}
}
