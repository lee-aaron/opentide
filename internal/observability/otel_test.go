package observability

import (
	"context"
	"testing"
)

func TestInit(t *testing.T) {
	shutdown, err := Init(context.Background(), "opentide-test")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer shutdown()

	if Tracer == nil {
		t.Error("Tracer should not be nil after Init")
	}
	if Meter == nil {
		t.Error("Meter should not be nil after Init")
	}
	if Metrics == nil {
		t.Fatal("Metrics should not be nil after Init")
	}

	// Verify all metric instruments are registered
	if Metrics.MessageCount == nil {
		t.Error("MessageCount metric not registered")
	}
	if Metrics.SkillInvocations == nil {
		t.Error("SkillInvocations metric not registered")
	}
	if Metrics.ApprovalRequests == nil {
		t.Error("ApprovalRequests metric not registered")
	}
	if Metrics.RateLimitHits == nil {
		t.Error("RateLimitHits metric not registered")
	}
}

func TestMetricsRecording(t *testing.T) {
	shutdown, err := Init(context.Background(), "opentide-test")
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown()

	ctx := context.Background()

	// These should not panic
	Metrics.MessageCount.Add(ctx, 1)
	Metrics.SkillInvocations.Add(ctx, 1)
	Metrics.SkillErrors.Add(ctx, 1)
	Metrics.ApprovalRequests.Add(ctx, 1)
	Metrics.ApprovalDenials.Add(ctx, 1)
	Metrics.RateLimitHits.Add(ctx, 1)
	Metrics.ActiveSessions.Add(ctx, 1)
	Metrics.ActiveSessions.Add(ctx, -1)
	Metrics.MessageLatency.Record(ctx, 0.5)
	Metrics.SkillLatency.Record(ctx, 1.2)
}

func TestAttributeKeys(t *testing.T) {
	if string(AttrUserID) == "" {
		t.Error("AttrUserID key is empty")
	}
	if string(AttrSkillName) == "" {
		t.Error("AttrSkillName key is empty")
	}
}
