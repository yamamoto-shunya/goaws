package router

import (
	"bytes"
	"encoding/json"
	"github.com/Admiral-Piett/goaws/app"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestIndexServerhandler_POST_BadRequest(t *testing.T) {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("POST", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	form := url.Values{}
	form.Add("Action", "BadRequest")
	req.PostForm = form

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	New().ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestIndexServerhandler_POST_GoodRequest(t *testing.T) {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("POST", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.ListTopics")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	New().ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestIndexServerhandler_POST_GoodRequest_With_URL(t *testing.T) {

	b, err := json.Marshal(app.CreateQueueInput{
		QueueName: "local-queue1",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", "/100010001000/local-queue1", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")

	rr := httptest.NewRecorder()
	New().ServeHTTP(rr, req)

	req, err = http.NewRequest("POST", "/100010001000/local-queue1", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Amz-Target", "AmazonSQS.GetQueueAttributes")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr = httptest.NewRecorder()

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	New().ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestIndexServerhandler_GET_GoodRequest_Pem_cert(t *testing.T) {

	req, err := http.NewRequest("GET", "/SimpleNotificationService/100010001000.pem", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	New().ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}
