package gosqs

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/google/go-cmp/cmp"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Admiral-Piett/goaws/app"
)

func TestListQueues_POST_NoQueues(t *testing.T) {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("POST", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ListQueue")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ListQueues)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := `{}`
	if !strings.HasPrefix(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestListQueues_POST_Success(t *testing.T) {
	// Prepare for test Queues
	app.SyncQueues.Queues["foo"] = &app.Queue{Name: "foo", URL: "http://:/queue/foo"}
	app.SyncQueues.Queues["bar"] = &app.Queue{Name: "bar", URL: "http://:/queue/bar"}
	app.SyncQueues.Queues["foobar"] = &app.Queue{Name: "foobar", URL: "http://:/queue/foobar"}

	req, err := http.NewRequest("POST", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ListQueue")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ListQueues)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	var got app.ListQueuesOutput
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	sort.Strings(got.QueueUrls)
	expected := app.ListQueuesOutput{
		QueueUrls: []string{
			"http://:/queue/bar",
			"http://:/queue/foo",
			"http://:/queue/foobar",
		},
	}
	if diff := cmp.Diff(got, expected); diff != "" {
		t.Errorf("handler returned unexpected body: %v", diff)
	}

	// Filter lists by the given QueueNamePrefix
	b, err := json.Marshal(app.ListQueuesInput{
		QueueNamePrefix: "fo",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ListQueue")

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	sort.Strings(got.QueueUrls)
	expected = app.ListQueuesOutput{
		QueueUrls: []string{
			"http://:/queue/foo",
			"http://:/queue/foobar",
		},
	}
	if diff := cmp.Diff(got, expected); diff != "" {
		t.Errorf("handler returned unexpected body: %v", diff)
	}
}

func TestCreateQueuehandler_POST_CreateQueue(t *testing.T) {
	b, err := json.Marshal(app.CreateQueueInput{
		Attributes: app.Attributes{
			"VisibilityTimeout":  "60",
			"MaximumMessageSize": "2048",
		},
		QueueName: "UnitTestQueue1",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	queueName := "UnitTestQueue1"

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateQueue)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := queueName
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
	expectedQueue := &app.Queue{
		Name:               queueName,
		URL:                "http://://" + queueName,
		Arn:                "arn:aws:sqs:::" + queueName,
		TimeoutSecs:        60,
		MaximumMessageSize: 2048,
		Duplicates:         make(map[string]time.Time),
	}
	actualQueue := app.SyncQueues.Queues[queueName]
	if diff := cmp.Diff(expectedQueue, actualQueue); diff != "" {
		t.Fatalf("diff: %v", diff)
	}
}

func TestCreateFIFOQueuehandler_POST_CreateQueue(t *testing.T) {
	b, err := json.Marshal(app.CreateQueueInput{
		Attributes: app.Attributes{
			"VisibilityTimeout": "60",
		},
		QueueName: "UnitTestQueue1.fifo",
	})
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	queueName := "UnitTestQueue1.fifo"

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateQueue)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := queueName
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
	expectedQueue := &app.Queue{
		Name:        queueName,
		URL:         "http://://" + queueName,
		Arn:         "arn:aws:sqs:::" + queueName,
		TimeoutSecs: 60,
		IsFIFO:      true,
		Duplicates:  make(map[string]time.Time),
	}
	actualQueue := app.SyncQueues.Queues[queueName]
	if diff := cmp.Diff(expectedQueue, actualQueue); diff != "" {
		t.Fatalf("diff: %v", diff)
	}
}

func TestSendMessage_MaximumMessageSize_Success(t *testing.T) {
	app.SyncQueues.Queues["test_max_message_size"] = &app.Queue{Name: "test_max_message_size", MaximumMessageSize: 100}

	b, err := json.Marshal(app.SendMessageInput{
		MessageBody:            "test%20message%20body%201",
		MessageDeduplicationId: "123",
		QueueUrl:               "http://localhost:4100/queue/test_max_message_size",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SendMessage)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := "MD5OfMessageBody"
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestSendMessage_MaximumMessageSize_MessageTooBig(t *testing.T) {
	b, err := json.Marshal(app.SendMessageInput{
		MessageBody: "test%20message%20body%201",
		QueueUrl:    "http://localhost:4100/queue/test_max_message_size",
	})
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	app.SyncQueues.Queues["test_max_message_size"] =
		&app.Queue{Name: "test_max_message_size", MaximumMessageSize: 10}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SendMessage)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Check the response body is what we expect.
	expected := "MessageTooBig"
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestSendQueue_POST_NonExistant(t *testing.T) {
	// Create a request to pass to our handler.
	b, err := json.Marshal(app.SendMessageInput{
		MessageBody: "Test123",
		QueueUrl:    "http://localhost:4100/queue/NON-EXISTANT",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SendMessage)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Check the response body is what we expect.
	expected := "NonExistentQueue"
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestSendMessageBatch_POST_QueueNotFound(t *testing.T) {
	b, err := json.Marshal(app.SendMessageBatchInput{
		Entries: []*app.SendMessageBatchRequestEntry{
			{
				Id:          "test_msg_001",
				MessageBody: "test%20message%20body%201",
			},
			{
				Id:          "test_msg_002",
				MessageBody: "test%20message%20body%202",
			},
		},
		QueueUrl: "http://localhost:4100/queue/testing",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessageBatch")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SendMessageBatch)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Check the response body is what we expect.
	expected := "NonExistentQueue"
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestSendMessageBatch_POST_NoEntry(t *testing.T) {
	in := app.SendMessageBatchInput{
		QueueUrl: "http://localhost:4100/queue/testing",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessageBatch")

	app.SyncQueues.Queues["testing"] = &app.Queue{Name: "testing"}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SendMessageBatch)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Check the response body is what we expect.
	expected := "EmptyBatchRequest"
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}

	// add empty request entry
	in.Entries = []*app.SendMessageBatchRequestEntry{}
	b, err = json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessageBatch")

	rr = httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestSendMessageBatch_POST_IdNotDistinct(t *testing.T) {
	b, err := json.Marshal(app.SendMessageBatchInput{
		Entries: []*app.SendMessageBatchRequestEntry{
			{
				Id:          "test_msg_001",
				MessageBody: "test%20message%20body%201",
			},
			{
				Id:          "test_msg_001",
				MessageBody: "test%20message%20body%202",
			},
		},
		QueueUrl: "http://localhost:4100/queue/testing",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessageBatch")

	app.SyncQueues.Queues["testing"] = &app.Queue{Name: "testing"}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SendMessageBatch)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Check the response body is what we expect.
	expected := "BatchEntryIdsNotDistinct"
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestSendMessageBatch_POST_TooManyEntries(t *testing.T) {
	b, err := json.Marshal(app.SendMessageBatchInput{
		Entries: []*app.SendMessageBatchRequestEntry{
			{
				Id:          "test_msg_001",
				MessageBody: "test%20message%20body%201",
			},
			{
				Id:          "test_msg_002",
				MessageBody: "test%20message%20body%202",
			},
			{
				Id:          "test_msg_003",
				MessageBody: "test%20message%20body%202",
			},
			{
				Id:          "test_msg_004",
				MessageBody: "test%20message%20body%202",
			},
			{
				Id:          "test_msg_005",
				MessageBody: "test%20message%20body%202",
			},
			{
				Id:          "test_msg_006",
				MessageBody: "test%20message%20body%202",
			},
			{
				Id:          "test_msg_007",
				MessageBody: "test%20message%20body%202",
			},
			{
				Id:          "test_msg_008",
				MessageBody: "test%20message%20body%202",
			},
			{
				Id:          "test_msg_009",
				MessageBody: "test%20message%20body%202",
			},
			{
				Id:          "test_msg_010",
				MessageBody: "test%20message%20body%202",
			},
			{
				Id:          "test_msg_011",
				MessageBody: "test%20message%20body%202",
			},
		},
		QueueUrl: "http://localhost:4100/queue/testing",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessageBatch")

	app.SyncQueues.Queues["testing"] = &app.Queue{Name: "testing"}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SendMessageBatch)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Check the response body is what we expect.
	expected := "TooManyEntriesInBatchRequest"
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestSendMessageBatch_POST_Success(t *testing.T) {
	b, err := json.Marshal(app.SendMessageBatchInput{
		Entries: []*app.SendMessageBatchRequestEntry{
			{
				Id:          "test_msg_001",
				MessageBody: "test%20message%20body%201",
			},
			{
				Id: "test_msg_002",
				MessageAttributes: map[string]*app.MessageAttributeValue{
					"test_attribute_name_1": {
						DataType:    "String",
						StringValue: "test_attribute_value_1",
					},
				},
				MessageBody: "test%20message%20body%202",
			},
		},
		QueueUrl: "http://localhost:4100/queue/testing",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessageBatch")

	app.SyncQueues.Queues["testing"] = &app.Queue{Name: "testing"}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SendMessageBatch)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := `"MD5OfMessageBody":"58bdcfd42148396616e4260421a9b4e5"`
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestSendMessageBatchToFIFOQueue_POST_Success(t *testing.T) {
	b, err := json.Marshal(app.SendMessageBatchInput{
		Entries: []*app.SendMessageBatchRequestEntry{
			{
				Id:             "test_msg_001",
				MessageBody:    "test%20message%20body%201",
				MessageGroupId: "GROUP-X",
			},
			{
				Id: "test_msg_002",
				MessageAttributes: map[string]*app.MessageAttributeValue{
					"test_attribute_name_1": {
						DataType:    "String",
						StringValue: "test_attribute_value_1",
					},
				},
				MessageBody:    "test%20message%20body%202",
				MessageGroupId: "GROUP-X",
			},
		},
		QueueUrl: "http://localhost:4100/queue/testing.fifo",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessageBatch")

	app.SyncQueues.Queues["testing.fifo"] = &app.Queue{
		Name:   "testing.fifo",
		IsFIFO: true,
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SendMessageBatch)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := `"MD5OfMessageBody":"1c538b76fce1a234bce865025c02b042"`
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestChangeMessageVisibility_POST_SUCCESS(t *testing.T) {
	b, err := json.Marshal(app.ChangeMessageVisibilityInput{
		QueueUrl:          "http://localhost:4100/queue/testing",
		ReceiptHandle:     "123",
		VisibilityTimeout: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessageBatch")

	app.SyncQueues.Queues["testing"] = &app.Queue{Name: "testing"}
	app.SyncQueues.Queues["testing"].Messages = []app.Message{{
		MessageBody:   []byte("test1"),
		ReceiptHandle: "123",
	}}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ChangeMessageVisibility)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := ""
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestRequeueing_VisibilityTimeoutExpires(t *testing.T) {
	done := make(chan struct{}, 0)
	go PeriodicTasks(1*time.Second, done)

	// create a queue
	b, err := json.Marshal(app.CreateQueueInput{
		Attributes: app.Attributes{
			"VisibilityTimeout": "1",
		},
		QueueName: "requeue",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	rr := httptest.NewRecorder()
	http.HandlerFunc(CreateQueue).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// send a message
	b, err = json.Marshal(app.SendMessageInput{
		MessageBody: "1",
		QueueUrl:    "http://localhost:4100/queue/requeue",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(SendMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// receive message
	b, err = json.Marshal(app.ReceiveMessageInput{
		QueueUrl: "http://localhost:4100/queue/requeue",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); !ok {
		t.Logf("got: %v", rr.Body.String())
		t.Fatal("handler should return a message")
	}

	// try to receive another message with same req params.
	b, err = json.Marshal(app.ReceiveMessageInput{
		QueueUrl: "http://localhost:4100/queue/requeue",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); ok {
		t.Logf("got: %v", rr.Body.String())
		t.Fatal("handler should not return a message")
	}
	time.Sleep(2 * time.Second)

	// message needs to be requeued. It uses same req params
	b, err = json.Marshal(app.ReceiveMessageInput{
		QueueUrl: "http://localhost:4100/queue/requeue",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); !ok {
		t.Logf("got: %v", rr.Body.String())
		t.Fatal("handler should return a message")
	}
	done <- struct{}{}
}

func TestRequeueing_ResetVisibilityTimeout(t *testing.T) {
	done := make(chan struct{}, 0)
	go PeriodicTasks(1*time.Second, done)

	// create a queue
	b, err := json.Marshal(app.CreateQueueInput{
		Attributes: app.Attributes{
			"VisibilityTimeout": "10",
		},
		QueueName: "requeue-reset",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	rr := httptest.NewRecorder()
	http.HandlerFunc(CreateQueue).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// send a message
	b, err = json.Marshal(app.SendMessageInput{
		MessageBody: "1",
		QueueUrl:    "http://localhost:4100/queue/requeue-reset",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(SendMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// receive message
	b, err = json.Marshal(app.ReceiveMessageInput{
		QueueUrl: "http://localhost:4100/queue/requeue-reset",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); !ok {
		t.Logf("got: %v", rr.Body.String())
		t.Fatal("handler should return a message")
	}

	resp := app.ReceiveMessageOutput{}
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("unexpected unmarshal error: %s", err)
	}
	receiptHandle := resp.Messages[0].ReceiptHandle

	// try to receive another message.
	b, err = json.Marshal(app.ReceiveMessageInput{
		QueueUrl: "http://localhost:4100/queue/requeue-reset",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); ok {
		t.Logf("got: %v", rr.Body.String())
		t.Fatal("handler should not return a message")
	}

	// reset message visibility timeout to requeue it
	b, err = json.Marshal(app.ChangeMessageVisibilityInput{
		QueueUrl:          "http://localhost:4100/queue/requeue-reset",
		ReceiptHandle:     receiptHandle,
		VisibilityTimeout: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ChangeMessageVisibility")

	rr = httptest.NewRecorder()
	http.HandlerFunc(ChangeMessageVisibility).ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// message needs to be requeued
	b, err = json.Marshal(app.ReceiveMessageInput{
		QueueUrl: "http://localhost:4100/queue/requeue-reset",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); !ok {
		t.Logf("got: %v", rr.Body.String())
		t.Fatal("handler should return a message")
	}
	done <- struct{}{}
}

func TestDeadLetterQueue(t *testing.T) {
	done := make(chan struct{}, 0)
	go PeriodicTasks(1*time.Second, done)

	// create a dead letter queue
	deadLetterQueue := &app.Queue{
		Name:     "failed-messages",
		Messages: []app.Message{},
	}
	app.SyncQueues.Lock()
	app.SyncQueues.Queues["failed-messages"] = deadLetterQueue
	app.SyncQueues.Unlock()

	// create a queue
	b, err := json.Marshal(app.CreateQueueInput{
		Attributes: app.Attributes{
			"VisibilityTimeout": "1",
			"RedrivePolicy":     `{"maxReceiveCount": 1, "deadLetterTargetArn":"arn:aws:sqs::000000000000:failed-messages"}`,
		},
		QueueName: "testing-deadletter",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	rr := httptest.NewRecorder()
	http.HandlerFunc(CreateQueue).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// send a message
	b, err = json.Marshal(app.SendMessageInput{
		MessageBody: "1",
		QueueUrl:    "http://localhost:4100/queue/testing-deadletter",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(SendMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// receive message
	b, err = json.Marshal(app.ReceiveMessageInput{
		QueueUrl: "http://localhost:4100/queue/testing-deadletter",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); !ok {
		t.Logf("got: %v", rr.Body.String())
		t.Fatal("handler should return a message")
	}

	time.Sleep(2 * time.Second)

	// receive the message one more time
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); !ok {
		t.Logf("got: %v", rr.Body.String())
		t.Fatal("handler should return a message")
	}
	time.Sleep(2 * time.Second)

	// another receive attempt
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); ok {
		t.Logf("got: %v", rr.Body.String())
		t.Fatal("handler should not return a message")
	}
	if len(deadLetterQueue.Messages) == 0 {
		t.Fatal("expected a message")
	}
}

func TestReceiveMessageWaitTimeEnforced(t *testing.T) {
	// create a queue
	b, err := json.Marshal(app.CreateQueueInput{
		Attributes: app.Attributes{
			"ReceiveMessageWaitTimeSeconds": "2",
		},
		QueueName: "waiting-queue",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	rr := httptest.NewRecorder()
	http.HandlerFunc(CreateQueue).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// receive message ensure delay
	b, err = json.Marshal(app.ReceiveMessageInput{
		QueueUrl: "http://localhost:4100/queue/waiting-queue",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()

	start := time.Now()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)
	elapsed := time.Since(start)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); ok {
		t.Logf("got: %v", rr.Body.String())
		t.Fatal("handler should not return a message")
	}
	if elapsed < 2*time.Second {
		t.Fatal("handler didn't wait ReceiveMessageWaitTimeSeconds")
	}

	// send a message
	b, err = json.Marshal(app.SendMessageInput{
		MessageBody: "1",
		QueueUrl:    "http://localhost:4100/queue/waiting-queue",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(SendMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// receive message
	b, err = json.Marshal(app.ReceiveMessageInput{
		QueueUrl: "http://localhost:4100/queue/waiting-queue",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()

	start = time.Now()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)
	elapsed = time.Since(start)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); !ok {
		t.Logf("got: %v", rr.Body.String())
		t.Fatal("handler should return a message")
	}
	if elapsed > 1*time.Second {
		t.Fatal("handler waited when message was available, expected not to wait")
	}
}

func TestReceiveMessage_CanceledByClient(t *testing.T) {
	// create a queue
	b, err := json.Marshal(app.CreateQueueInput{
		Attributes: app.Attributes{
			"ReceiveMessageWaitTimeSeconds": "20",
		},
		QueueName: "cancel-queue",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	rr := httptest.NewRecorder()
	http.HandlerFunc(CreateQueue).ServeHTTP(rr, req)

	var wg sync.WaitGroup
	ctx, cancelReceive := context.WithCancel(context.Background())

	wg.Add(1)
	go func() {
		defer wg.Done()
		// receive message (that will be canceled)
		b, err = json.Marshal(app.ReceiveMessageInput{
			QueueUrl: "http://localhost:4100/queue/cancel-queue",
		})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got \n%v want %v",
				status, http.StatusOK)
		}

		if ok := strings.Contains(rr.Body.String(), "12345"); ok {
			t.Logf("got: %v", rr.Body.String())
			t.Fatal("expecting this ReceiveMessage() to not pickup this message as it should canceled before the Send()")
		}
	}()
	time.Sleep(100 * time.Millisecond) // let enought time for the Receive go to wait mode
	cancelReceive()                    // cancel the first ReceiveMessage(), make sure it will not pickup the sent message below
	time.Sleep(5 * time.Millisecond)

	// send a message
	b, err = json.Marshal(app.SendMessageInput{
		MessageBody: "12345",
		QueueUrl:    "http://localhost:4100/queue/cancel-queue",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(SendMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// receive message
	b, err = json.Marshal(app.ReceiveMessageInput{
		QueueUrl: "http://localhost:4100/queue/cancel-queue",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()

	start := time.Now()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)
	elapsed := time.Since(start)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if ok := strings.Contains(rr.Body.String(), "12345"); !ok {
		t.Logf("got: %v", rr.Body.String())
		t.Fatal("handler should return a message")
	}
	if elapsed > 1*time.Second {
		t.Fatal("handler waited when message was available, expected not to wait")
	}

	if timedout := waitTimeout(&wg, 2*time.Second); timedout {
		t.Errorf("expected ReceiveMessage() in goroutine to exit quickly due to cancelReceive() called")
	}
}

func TestReceiveMessage_WithConcurrentDeleteQueue(t *testing.T) {
	// create a queue
	b, err := json.Marshal(app.CreateQueueInput{
		Attributes: app.Attributes{
			"ReceiveMessageWaitTimeSeconds": "1",
		},
		QueueName: "waiting-queue",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	rr := httptest.NewRecorder()
	http.HandlerFunc(CreateQueue).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		// receive message
		b, err = json.Marshal(app.ReceiveMessageInput{
			QueueUrl: "http://localhost:4100/queue/waiting-queue",
		})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

		rr := httptest.NewRecorder()

		http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)

		// Check the status code is what we expect.
		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusBadRequest)
		}

		// Check the response body is what we expect.
		expected := "QueueNotFound"
		if !strings.Contains(rr.Body.String(), "Not Found") {
			t.Errorf("handler returned unexpected body: got %v want %v",
				rr.Body.String(), expected)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond) // 10ms to let the ReceiveMessage() block
		// delete queue message
		b, err = json.Marshal(app.DeleteQueueInput{
			QueueUrl: "http://localhost:4100/queue/waiting-queue",
		})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Amz-Target", "AmazonSQS.DeleteQueue")

		rr := httptest.NewRecorder()
		http.HandlerFunc(DeleteQueue).ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got \n%v want %v",
				status, http.StatusOK)
		}
	}()

	if timedout := waitTimeout(&wg, 2*time.Second); timedout {
		t.Errorf("concurrent handlers timeout, expecting both to return within timeout")
	}
}

func TestReceiveMessageDelaySeconds(t *testing.T) {
	// create a queue
	b, err := json.Marshal(app.CreateQueueInput{
		Attributes: app.Attributes{
			"DelaySeconds": "2",
		},
		QueueName: "delay-seconds-queue",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	rr := httptest.NewRecorder()
	http.HandlerFunc(CreateQueue).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// send a message
	b, err = json.Marshal(app.SendMessageInput{
		MessageBody: "1",
		QueueUrl:    "http://localhost:4100/queue/delay-seconds-queue",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(SendMessage).ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// receive message before delay is up
	b, err = json.Marshal(app.ReceiveMessageInput{
		QueueUrl: "http://localhost:4100/queue/delay-seconds-queue",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); ok {
		t.Logf("got: %v", rr.Body.String())
		t.Fatal("handler should not return a message")
	}

	// receive message with wait should return after delay
	b, err = json.Marshal(app.ReceiveMessageInput{
		QueueUrl:        "http://localhost:4100/queue/delay-seconds-queue",
		WaitTimeSeconds: aws.Int64(10),
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()
	start := time.Now()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)
	elapsed := time.Since(start)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if elapsed < 1*time.Second {
		t.Errorf("handler didn't wait at all")
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); !ok {
		t.Logf("got: %v", rr.Body.String())
		t.Errorf("handler should return a message")
	}
	if elapsed > 4*time.Second {
		t.Errorf("handler didn't need to wait all WaitTimeSeconds=10, only DelaySeconds=2")
	}
}

func TestSetQueueAttributes_POST_QueueNotFound(t *testing.T) {
	b, err := json.Marshal(app.SetQueueAttributesInput{
		QueueUrl: "http://localhost:4100/queue/not-existing",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SetQueueAttributes")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SetQueueAttributes)

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Check the response body is what we expect.
	expected := "NonExistentQueue"
	if !strings.Contains(rr.Body.String(), expected) {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestSendingAndReceivingFromFIFOQueueReturnsSameMessageOnError(t *testing.T) {
	done := make(chan struct{}, 0)
	go PeriodicTasks(1*time.Second, done)

	// create a queue
	b, err := json.Marshal(app.CreateQueueInput{
		Attributes: app.Attributes{
			"VisibilityTimeout": "2",
		},
		QueueName: "requeue-reset.fifo",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	rr := httptest.NewRecorder()
	http.HandlerFunc(CreateQueue).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// send a message
	b, err = json.Marshal(app.SendMessageInput{
		MessageBody:    "1",
		MessageGroupId: "GROUP-X",
		QueueUrl:       "http://localhost:4100/queue/requeue-reset.fifo",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(SendMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// send a message
	b, err = json.Marshal(app.SendMessageInput{
		MessageBody:    "2",
		MessageGroupId: "GROUP-X",
		QueueUrl:       "http://localhost:4100/queue/requeue-reset.fifo",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(SendMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// receive message
	b, err = json.Marshal(app.ReceiveMessageInput{
		QueueUrl: "http://localhost:4100/queue/requeue-reset.fifo",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); !ok {
		t.Logf("got: %v", rr.Body.String())
		t.Fatal("handler should return a message")
	}

	resp := app.ReceiveMessageOutput{}
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("unexpected unmarshal error: %s", err)
	}
	receiptHandleFirst := resp.Messages[0].ReceiptHandle
	if string(resp.Messages[0].Body) != "1" {
		t.Fatalf("should have received body 1: %s", err)
	}

	// try to receive another message and we should get none
	b, err = json.Marshal(app.ReceiveMessageInput{
		QueueUrl: "http://localhost:4100/queue/requeue-reset.fifo",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); ok {
		t.Logf("got: %v", rr.Body.String())
		t.Fatal("handler should not return a message")
	}

	if len(app.SyncQueues.Queues["requeue-reset.fifo"].FIFOMessages) != 1 {
		t.Fatal("there should be only 1 group locked")
	}

	if app.SyncQueues.Queues["requeue-reset.fifo"].FIFOMessages["GROUP-X"] != 0 {
		t.Fatal("there should be GROUP-X locked")
	}

	// remove message
	b, err = json.Marshal(app.DeleteMessageInput{
		QueueUrl:      "http://localhost:4100/queue/requeue-reset.fifo",
		ReceiptHandle: receiptHandleFirst,
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.DeleteMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(DeleteMessage).ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if len(app.SyncQueues.Queues["requeue-reset.fifo"].Messages) != 1 {
		t.Fatal("there should be only 1 message in queue")
	}

	// receive message - loop until visibility timeouts
	for {
		b, err = json.Marshal(app.ReceiveMessageInput{
			QueueUrl: "http://localhost:4100/queue/requeue-reset.fifo",
		})
		if err != nil {
			t.Fatal(err)
		}
		req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

		rr = httptest.NewRecorder()
		http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got \n%v want %v",
				status, http.StatusOK)
		}
		if ok := strings.Contains(rr.Body.String(), `"Messages"`); !ok {
			continue
		}

		resp = app.ReceiveMessageOutput{}
		err = json.Unmarshal(rr.Body.Bytes(), &resp)
		if err != nil {
			t.Fatalf("unexpected unmarshal error: %s", err)
		}
		if string(resp.Messages[0].Body) != "2" {
			t.Fatalf("should have received body 2: %s", err)
		}
		break
	}

	done <- struct{}{}
}

func TestSendMessage_POST_DuplicatationNotAppliedToStandardQueue(t *testing.T) {
	done := make(chan struct{}, 0)
	go PeriodicTasks(1*time.Second, done)

	// create a queue
	b, err := json.Marshal(app.CreateQueueInput{
		QueueName: "stantdard-testing",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	rr := httptest.NewRecorder()
	http.HandlerFunc(CreateQueue).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	b, err = json.Marshal(app.SendMessageInput{
		MessageBody:            "Test1",
		MessageDeduplicationId: "123",
		QueueUrl:               "http://localhost:4100/queue/stantdard-testing",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(SendMessage).ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if len(app.SyncQueues.Queues["stantdard-testing"].Messages) == 0 {
		t.Fatal("there should be 1 message in queue")
	}

	b, err = json.Marshal(app.SendMessageInput{
		MessageBody:            "Test2",
		MessageDeduplicationId: "123",
		QueueUrl:               "http://localhost:4100/queue/stantdard-testing",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(SendMessage).ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if len(app.SyncQueues.Queues["stantdard-testing"].Messages) == 1 {
		t.Fatal("there should be 2 messages in queue")
	}
}

func TestSendMessage_POST_DuplicatationDisabledOnFifoQueue(t *testing.T) {
	done := make(chan struct{}, 0)
	go PeriodicTasks(1*time.Second, done)

	// create a queue
	b, err := json.Marshal(app.CreateQueueInput{
		QueueName: "no-dup-testing.fifo",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	rr := httptest.NewRecorder()
	http.HandlerFunc(CreateQueue).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	b, err = json.Marshal(app.SendMessageInput{
		MessageBody:            "Test1",
		MessageDeduplicationId: "123",
		QueueUrl:               "http://localhost:4100/queue/no-dup-testing.fifo",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(SendMessage).ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if len(app.SyncQueues.Queues["no-dup-testing.fifo"].Messages) == 0 {
		t.Fatal("there should be 1 message in queue")
	}

	b, err = json.Marshal(app.SendMessageInput{
		MessageBody:            "Test2",
		MessageDeduplicationId: "123",
		QueueUrl:               "http://localhost:4100/queue/no-dup-testing.fifo",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(SendMessage).ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if len(app.SyncQueues.Queues["no-dup-testing.fifo"].Messages) != 2 {
		t.Fatal("there should be 2 message in queue")
	}
}

func TestSendMessage_POST_DuplicatationEnabledOnFifoQueue(t *testing.T) {
	done := make(chan struct{}, 0)
	go PeriodicTasks(1*time.Second, done)

	// create a queue
	b, err := json.Marshal(app.CreateQueueInput{
		QueueName: "dup-testing.fifo",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	rr := httptest.NewRecorder()
	http.HandlerFunc(CreateQueue).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	app.SyncQueues.Queues["dup-testing.fifo"].EnableDuplicates = true

	b, err = json.Marshal(app.SendMessageInput{
		MessageBody:            "Test1",
		MessageDeduplicationId: "123",
		QueueUrl:               "http://localhost:4100/queue/dup-testing.fifo",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(SendMessage).ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if len(app.SyncQueues.Queues["dup-testing.fifo"].Messages) == 0 {
		t.Fatal("there should be 1 message in queue")
	}

	b, err = json.Marshal(app.SendMessageInput{
		MessageBody:            "Test2",
		MessageDeduplicationId: "123",
		QueueUrl:               "http://localhost:4100/queue/dup-testing.fifo",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(SendMessage).ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if len(app.SyncQueues.Queues["dup-testing.fifo"].Messages) != 1 {
		t.Fatal("there should be 1 message in queue")
	}
	if body := app.SyncQueues.Queues["dup-testing.fifo"].Messages[0].MessageBody; string(body) == "Test2" {
		t.Logf("got: %v", string(body))
		t.Fatal("duplicate message should not be added to queue")
	}
}

func TestSendMessage_POST_DelaySeconds(t *testing.T) {
	// create a queue
	b, err := json.Marshal(app.CreateQueueInput{
		QueueName: "sendmessage-delay",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	rr := httptest.NewRecorder()
	http.HandlerFunc(CreateQueue).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// send a message
	b, err = json.Marshal(app.SendMessageInput{
		DelaySeconds: aws.Int64(2),
		MessageBody:  "1",
		QueueUrl:     "http://localhost:4100/queue/sendmessage-delay",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(SendMessage).ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// receive message before delay is up
	b, err = json.Marshal(app.ReceiveMessageInput{
		QueueUrl: "http://localhost:4100/queue/sendmessage-delay",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); ok {
		t.Logf("got: %v", rr.Body.String())
		t.Fatal("handler should not return a message")
	}

	// receive message with wait should return after delay
	b, err = json.Marshal(app.ReceiveMessageInput{
		QueueUrl:        "http://localhost:4100/queue/sendmessage-delay",
		WaitTimeSeconds: aws.Int64(10),
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ReceiveMessage")

	rr = httptest.NewRecorder()
	start := time.Now()
	http.HandlerFunc(ReceiveMessage).ServeHTTP(rr, req)
	elapsed := time.Since(start)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}
	if elapsed < 1*time.Second {
		t.Errorf("handler didn't wait at all")
	}
	if ok := strings.Contains(rr.Body.String(), `"Messages"`); !ok {
		t.Logf("got: %v", rr.Body.String())
		t.Errorf("handler should return a message")
	}
	if elapsed > 4*time.Second {
		t.Errorf("handler didn't need to wait all WaitTimeSeconds=10, only DelaySeconds=2")
	}
}

func TestGetQueueAttributes_GetAllAttributes(t *testing.T) {
	done := make(chan struct{}, 0)
	go PeriodicTasks(1*time.Second, done)

	// create a queue
	b, err := json.Marshal(app.CreateQueueInput{
		QueueName: "get-queue-attributes",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	rr := httptest.NewRecorder()
	http.HandlerFunc(CreateQueue).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// get queue attributes
	b, err = json.Marshal(app.GetQueueAttributesInput{
		AttributeNames: []string{"All"},
		QueueUrl:       "http://localhost:4100/queue/get-queue-attributes",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.GetQueueAtrributes")

	rr = httptest.NewRecorder()
	http.HandlerFunc(GetQueueAttributes).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	resp := app.GetQueueAttributesOutput{}
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("unexpected unmarshal error: %s", err)
	}

	hasAttribute := func(attrs app.Attributes, name string) bool {
		if _, ok := attrs[name]; ok {
			return true
		}
		return false
	}

	ok := hasAttribute(resp.Attrs, "VisibilityTimeout") &&
		hasAttribute(resp.Attrs, "DelaySeconds") &&
		hasAttribute(resp.Attrs, "ReceiveMessageWaitTimeSeconds") &&
		hasAttribute(resp.Attrs, "ApproximateNumberOfMessages") &&
		hasAttribute(resp.Attrs, "ApproximateNumberOfMessagesNotVisible") &&
		hasAttribute(resp.Attrs, "CreatedTimestamp") &&
		hasAttribute(resp.Attrs, "LastModifiedTimestamp") &&
		hasAttribute(resp.Attrs, "QueueArn") &&
		hasAttribute(resp.Attrs, "RedrivePolicy")

	if !ok {
		t.Logf("got: %v", resp.Attrs)
		t.Fatal("handler should return all attributes")
	}

	done <- struct{}{}
}

func TestGetQueueAttributes_GetSelectedAttributes(t *testing.T) {
	done := make(chan struct{}, 0)
	go PeriodicTasks(1*time.Second, done)

	// create a queue
	b, err := json.Marshal(app.CreateQueueInput{
		QueueName: "get-queue-attributes",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	rr := httptest.NewRecorder()
	http.HandlerFunc(CreateQueue).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	// get queue attributes
	b, err = json.Marshal(app.GetQueueAttributesInput{
		AttributeNames: []string{"ApproximateNumberOfMessages", "ApproximateNumberOfMessagesNotVisible"},
		QueueUrl:       "http://localhost:4100/queue/get-queue-attributes",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err = http.NewRequest("GET", "/", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.GetQueueAttributes")

	rr = httptest.NewRecorder()
	http.HandlerFunc(GetQueueAttributes).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got \n%v want %v",
			status, http.StatusOK)
	}

	resp := app.GetQueueAttributesOutput{}
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("unexpected unmarshal error: %s", err)
	}

	hasAttribute := func(attrs app.Attributes, name string) bool {
		if _, ok := attrs[name]; ok {
			return true
		}
		return false
	}

	ok := hasAttribute(resp.Attrs, "ApproximateNumberOfMessages") &&
		hasAttribute(resp.Attrs, "ApproximateNumberOfMessagesNotVisible")

	if !ok {
		t.Logf("got: %v", resp.Attrs)
		t.Fatal("handler should return requested attributes")
	}

	ok = !(hasAttribute(resp.Attrs, "VisibilityTimeout") ||
		hasAttribute(resp.Attrs, "DelaySeconds") ||
		hasAttribute(resp.Attrs, "ReceiveMessageWaitTimeSeconds") ||
		hasAttribute(resp.Attrs, "CreatedTimestamp") ||
		hasAttribute(resp.Attrs, "LastModifiedTimestamp") ||
		hasAttribute(resp.Attrs, "QueueArn") ||
		hasAttribute(resp.Attrs, "RedrivePolicy"))

	if !ok {
		t.Logf("got: %v", resp.Attrs)
		t.Fatal("handler should return only requested attributes")
	}

	done <- struct{}{}
}

// waitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
// credits: https://stackoverflow.com/questions/32840687/timeout-for-waitgroup-wait
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}
