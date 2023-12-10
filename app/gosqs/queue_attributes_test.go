package gosqs

import (
	"testing"

	"github.com/Admiral-Piett/goaws/app"
	"github.com/google/go-cmp/cmp"
)

func TestApplyQueueAttributes(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		deadLetterQueue := &app.Queue{Name: "failed-messages"}
		app.SyncQueues.Lock()
		app.SyncQueues.Queues["failed-messages"] = deadLetterQueue
		app.SyncQueues.Unlock()
		q := &app.Queue{TimeoutSecs: 30}
		if err := validateAndSetQueueAttributes(q, app.Attributes{
			"DelaySeconds":                  "25",
			"VisibilityTimeout":             "60",
			"Policy":                        "",
			"RedrivePolicy":                 `{"maxReceiveCount": "4", "deadLetterTargetArn":"arn:aws:sqs::000000000000:failed-messages"}`,
			"ReceiveMessageWaitTimeSeconds": "20",
		}); err != nil {
			t.Fatalf("expected nil, got %s", err)
		}
		expected := &app.Queue{
			TimeoutSecs:         60,
			ReceiveWaitTimeSecs: 20,
			DelaySecs:           25,
			MaxReceiveCount:     4,
			DeadLetterQueue:     deadLetterQueue,
		}
		if diff := cmp.Diff(q, expected); diff != "" {
			t.Fatalf("diff: %v", diff)
		}
	})
	t.Run("missing_deadletter_arn", func(t *testing.T) {
		q := &app.Queue{TimeoutSecs: 30}
		if err := validateAndSetQueueAttributes(q, app.Attributes{
			"RedrivePolicy": `{"maxReceiveCount": "4"}`,
		}); err != ErrInvalidParameterValue {
			t.Fatalf("expected %s, got %s", ErrInvalidParameterValue, err)
		}
	})
	t.Run("invalid_redrive_policy", func(t *testing.T) {
		q := &app.Queue{TimeoutSecs: 30}
		if err := validateAndSetQueueAttributes(q, app.Attributes{
			"RedrivePolicy": `{invalidinput}`,
		}); err != ErrInvalidAttributeValue {
			t.Fatalf("expected %s, got %s", ErrInvalidAttributeValue, err)
		}
	})
}
