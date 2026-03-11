package aiverify_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/aiverify"
)

func TestClassifyAction_AutoVerify(t *testing.T) {
	action := aiverify.ClassifyAction(0.97, 0.95, 0.30)
	if action != aiverify.ActionAutoVerify {
		t.Errorf("expected AutoVerify, got %s", action)
	}
}

func TestClassifyAction_AutoReject(t *testing.T) {
	action := aiverify.ClassifyAction(0.15, 0.95, 0.30)
	if action != aiverify.ActionAutoReject {
		t.Errorf("expected AutoReject, got %s", action)
	}
}

func TestClassifyAction_QueueForReview(t *testing.T) {
	action := aiverify.ClassifyAction(0.60, 0.95, 0.30)
	if action != aiverify.ActionQueue {
		t.Errorf("expected Queue, got %s", action)
	}
}

func TestClassifyAction_ExactVerifyThreshold(t *testing.T) {
	action := aiverify.ClassifyAction(0.95, 0.95, 0.30)
	if action != aiverify.ActionAutoVerify {
		t.Errorf("expected AutoVerify at exact threshold, got %s", action)
	}
}

func TestClassifyAction_ExactRejectThreshold(t *testing.T) {
	action := aiverify.ClassifyAction(0.30, 0.95, 0.30)
	if action != aiverify.ActionQueue {
		t.Errorf("expected Queue at exact reject threshold, got %s", action)
	}
}
