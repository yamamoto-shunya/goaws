package gosqs

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/Admiral-Piett/goaws/app"
)

var (
	ErrInvalidParameterValue = &app.SqsErrorType{
		HttpError: http.StatusBadRequest,
		Type:      "InvalidParameterValue",
		Code:      "AWS.SimpleQueueService.InvalidParameterValue",
		Message:   "An invalid or out-of-range value was supplied for the input parameter.",
	}
	ErrInvalidAttributeValue = &app.SqsErrorType{
		HttpError: http.StatusBadRequest,
		Type:      "InvalidAttributeValue",
		Code:      "AWS.SimpleQueueService.InvalidAttributeValue",
		Message:   "Invalid Value for the parameter RedrivePolicy.",
	}
)

// validateAndSetQueueAttributes applies the requested queue attributes to the given
// queue.
// TODO Currently it only supports VisibilityTimeout, MaximumMessageSize, DelaySeconds, RedrivePolicy and ReceiveMessageWaitTimeSeconds  attributes.
func validateAndSetQueueAttributes(q *app.Queue, attrs app.Attributes) error {
	visibilityTimeout, _ := strconv.Atoi(attrs["VisibilityTimeout"])
	if visibilityTimeout != 0 {
		q.TimeoutSecs = visibilityTimeout
	}
	receiveWaitTime, _ := strconv.Atoi(attrs["ReceiveMessageWaitTimeSeconds"])
	if receiveWaitTime != 0 {
		q.ReceiveWaitTimeSecs = receiveWaitTime
	}
	maximumMessageSize, _ := strconv.Atoi(attrs["MaximumMessageSize"])
	if maximumMessageSize != 0 {
		q.MaximumMessageSize = maximumMessageSize
	}
	strRedrivePolicy := attrs["RedrivePolicy"]
	if strRedrivePolicy != "" {
		// support both int and string maxReceiveCount (Amazon clients use string)
		redrivePolicy1 := struct {
			MaxReceiveCount     int    `json:"maxReceiveCount"`
			DeadLetterTargetArn string `json:"deadLetterTargetArn"`
		}{}
		redrivePolicy2 := struct {
			MaxReceiveCount     string `json:"maxReceiveCount"`
			DeadLetterTargetArn string `json:"deadLetterTargetArn"`
		}{}
		err1 := json.Unmarshal([]byte(strRedrivePolicy), &redrivePolicy1)
		err2 := json.Unmarshal([]byte(strRedrivePolicy), &redrivePolicy2)
		maxReceiveCount := redrivePolicy1.MaxReceiveCount
		deadLetterQueueArn := redrivePolicy1.DeadLetterTargetArn
		if err1 != nil && err2 != nil {
			return ErrInvalidAttributeValue
		} else if err1 != nil {
			maxReceiveCount, _ = strconv.Atoi(redrivePolicy2.MaxReceiveCount)
			deadLetterQueueArn = redrivePolicy2.DeadLetterTargetArn
		}

		if (deadLetterQueueArn != "" && maxReceiveCount == 0) ||
			(deadLetterQueueArn == "" && maxReceiveCount != 0) {
			return ErrInvalidParameterValue
		}
		dlt := strings.Split(deadLetterQueueArn, ":")
		deadLetterQueueName := dlt[len(dlt)-1]
		deadLetterQueue, ok := app.SyncQueues.Queues[deadLetterQueueName]
		if !ok {
			return ErrInvalidParameterValue
		}
		q.DeadLetterQueue = deadLetterQueue
		q.MaxReceiveCount = maxReceiveCount
	}
	delaySecs, _ := strconv.Atoi(attrs["DelaySeconds"])
	if delaySecs != 0 {
		q.DelaySecs = delaySecs
	}

	return nil
}
