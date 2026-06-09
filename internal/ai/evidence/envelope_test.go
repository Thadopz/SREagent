package evidence

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNewDataCreatesUsableEvidence(t *testing.T) {
	raw := Marshal(NewData([]string{"row"}, "found rows"))
	var env Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if !env.Success || env.Status != StatusSuccess {
		t.Fatalf("expected success envelope, got %#v", env)
	}
	if !env.CanUseAsEvidence || !strings.HasPrefix(env.EvidenceID, "ev_") {
		t.Fatalf("expected usable evidence id, got %#v", env)
	}
	if env.DataStatus != DataPresent {
		t.Fatalf("expected present data, got %q", env.DataStatus)
	}
}

func TestNewErrorCreatesFailedQuery(t *testing.T) {
	env := NewError(errors.New("prometheus request timed out"), "query timeout")
	if env.Success || env.Status != StatusTimeout {
		t.Fatalf("expected timeout envelope, got %#v", env)
	}
	if env.CanUseAsEvidence || env.EvidenceID != "" || !strings.HasPrefix(env.FailedQueryID, "fq_") {
		t.Fatalf("expected failed query id only, got %#v", env)
	}
	if env.DataStatus != DataUnknown {
		t.Fatalf("expected unknown data status, got %q", env.DataStatus)
	}
}

func TestParseLegacyFailureIsNotEvidence(t *testing.T) {
	parsed := Parse(`{"success":false,"error":"boom","message":"query failed"}`)
	if parsed.Envelope.Status != StatusError {
		t.Fatalf("expected error status, got %#v", parsed.Envelope)
	}
	if parsed.Envelope.CanUseAsEvidence || parsed.Envelope.EvidenceID != "" {
		t.Fatalf("legacy failure must not be evidence, got %#v", parsed.Envelope)
	}
	if !strings.HasPrefix(parsed.Envelope.FailedQueryID, "fq_") {
		t.Fatalf("expected generated failed query id, got %#v", parsed.Envelope)
	}
}
