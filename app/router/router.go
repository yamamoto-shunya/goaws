package router

import (
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	sns "github.com/Admiral-Piett/goaws/app/gosns"
	sqs "github.com/Admiral-Piett/goaws/app/gosqs"
	"github.com/gorilla/mux"
)

// New returns a new router
func New() http.Handler {
	r := mux.NewRouter()
	r.Use(writeHeaderMiddleware)

	r.HandleFunc("/", actionHandler).Methods("GET", "POST")
	r.HandleFunc("/health", health).Methods("GET")
	r.HandleFunc("/{account}", actionHandler).Methods("GET", "POST")
	r.HandleFunc("/queue/{queueName}", actionHandler).Methods("GET", "POST")
	r.HandleFunc("/SimpleNotificationService/{id}.pem", pemHandler).Methods("GET")
	r.HandleFunc("/{account}/{queueName}", actionHandler).Methods("GET", "POST")

	return r
}

// カスタムResponseWriterの定義
type customResponseWriter struct {
	http.ResponseWriter
	buf *bytes.Buffer
}

func (w *customResponseWriter) Write(data []byte) (int, error) {
	// バッファにデータを書き込む
	w.buf.Write(data)
	// 本来のResponseWriterにもデータを書き込む
	return w.ResponseWriter.Write(data)
}

func writeHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		crw := &customResponseWriter{w, new(bytes.Buffer)}
		next.ServeHTTP(crw, r)
		w.Header().Set("x-amzn-RequestId", "00000000-0000-0000-0000-000000000000")
		w.Header().Set("Content-Length", strconv.Itoa(crw.buf.Len()))
		w.Header().Set("Date", time.Now().Format("20060102T150405Z"))
	})
}

var routingTable = map[string]http.HandlerFunc{
	// SQS
	"ListQueues":              sqs.ListQueues,
	"CreateQueue":             sqs.CreateQueue,
	"GetQueueAttributes":      sqs.GetQueueAttributes,
	"SetQueueAttributes":      sqs.SetQueueAttributes,
	"SendMessage":             sqs.SendMessage,
	"SendMessageBatch":        sqs.SendMessageBatch,
	"ReceiveMessage":          sqs.ReceiveMessage,
	"DeleteMessage":           sqs.DeleteMessage,
	"DeleteMessageBatch":      sqs.DeleteMessageBatch,
	"GetQueueUrl":             sqs.GetQueueUrl,
	"PurgeQueue":              sqs.PurgeQueue,
	"DeleteQueue":             sqs.DeleteQueue,
	"ChangeMessageVisibility": sqs.ChangeMessageVisibility,

	// SNS
	"ListTopics":                sns.ListTopics,
	"CreateTopic":               sns.CreateTopic,
	"DeleteTopic":               sns.DeleteTopic,
	"Subscribe":                 sns.Subscribe,
	"SetSubscriptionAttributes": sns.SetSubscriptionAttributes,
	"GetSubscriptionAttributes": sns.GetSubscriptionAttributes,
	"ListSubscriptionsByTopic":  sns.ListSubscriptionsByTopic,
	"ListSubscriptions":         sns.ListSubscriptions,
	"Unsubscribe":               sns.Unsubscribe,
	"Publish":                   sns.Publish,

	// SNS Internal
	"ConfirmSubscription": sns.ConfirmSubscription,
}

func health(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(200)
	fmt.Fprint(w, "OK")
}

func actionHandler(w http.ResponseWriter, req *http.Request) {
	th := req.Header.Get("X-Amz-Target")
	resource, method, ok := strings.Cut(th, ".")
	if !ok {
		method = req.FormValue("Action")
	}

	//
	if method == "" {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			log.Println("Bad Request - failed to read Body", method)
			badRequest(w)
		}
		query, err := url.ParseQuery(string(b))
		if err != nil {
			log.Println("Bad Request - failed to parse query in Body", method)
			badRequest(w)
			return
		}
		method = query.Get("Action")
	}

	log.WithFields(
		log.Fields{
			"resource": resource,
			"action":   method,
			"url":      req.URL,
		}).Debug("Handling URL request")
	fn, ok := routingTable[method]
	if !ok {
		log.Println("Bad Request - Action:", method)
		badRequest(w)
		return
	}

	http.HandlerFunc(fn).ServeHTTP(w, req)
}

func pemHandler(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write(sns.PemKEY)
}

func badRequest(w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	io.WriteString(w, "Bad Request")
}
