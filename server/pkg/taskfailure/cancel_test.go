package taskfailure

import "testing"

func TestCancelSummary_knownReasons(t *testing.T) {
	cases := map[string]string{
		ReasonCancelUser.String():           "Cancelled by user",
		ReasonCancelTerminalFence.String():  "Cancelled: issue already terminal",
		ReasonCancelPrematureStage.String(): "Cancelled: premature stage barrier",
		ReasonCancelPlatform.String():       "Cancelled by platform",
	}
	for reason, want := range cases {
		if got := CancelSummary(reason); got != want {
			t.Errorf("CancelSummary(%q) = %q, want %q", reason, got, want)
		}
	}
}

func TestCancelSummary_unknownNonEmpty(t *testing.T) {
	if got := CancelSummary("cancel.custom"); got != "Cancelled: cancel.custom" {
		t.Fatalf("got %q", got)
	}
}

func TestCancelSummary_empty(t *testing.T) {
	if got := CancelSummary(""); got != "Cancelled (reason unknown)" {
		t.Fatalf("got %q", got)
	}
}
