package gosqs

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/Admiral-Piett/goaws/app"
	"github.com/Admiral-Piett/goaws/app/common"
	"github.com/gorilla/mux"
)

func init() {
	app.SyncQueues.Queues = make(map[string]*app.Queue)

	app.SqsErrors = make(map[string]app.SqsErrorType)
	err1 := app.SqsErrorType{HttpError: http.StatusBadRequest, Type: "Not Found", Code: "AWS.SimpleQueueService.NonExistentQueue", Message: "The specified queue does not exist for this wsdl version."}
	app.SqsErrors["QueueNotFound"] = err1
	err2 := app.SqsErrorType{HttpError: http.StatusBadRequest, Type: "Duplicate", Code: "AWS.SimpleQueueService.QueueExists", Message: "The specified queue already exists."}
	app.SqsErrors["QueueExists"] = err2
	err3 := app.SqsErrorType{HttpError: http.StatusNotFound, Type: "Not Found", Code: "AWS.SimpleQueueService.QueueExists", Message: "The specified queue does not contain the message specified."}
	app.SqsErrors["MessageDoesNotExist"] = err3
	err4 := app.SqsErrorType{HttpError: http.StatusBadRequest, Type: "GeneralError", Code: "AWS.SimpleQueueService.GeneralError", Message: "General Error."}
	app.SqsErrors["GeneralError"] = err4
	err5 := app.SqsErrorType{HttpError: http.StatusBadRequest, Type: "TooManyEntriesInBatchRequest", Code: "AWS.SimpleQueueService.TooManyEntriesInBatchRequest", Message: "Maximum number of entries per request are 10."}
	app.SqsErrors["TooManyEntriesInBatchRequest"] = err5
	err6 := app.SqsErrorType{HttpError: http.StatusBadRequest, Type: "BatchEntryIdsNotDistinct", Code: "AWS.SimpleQueueService.BatchEntryIdsNotDistinct", Message: "Two or more batch entries in the request have the same Id."}
	app.SqsErrors["BatchEntryIdsNotDistinct"] = err6
	err7 := app.SqsErrorType{HttpError: http.StatusBadRequest, Type: "EmptyBatchRequest", Code: "AWS.SimpleQueueService.EmptyBatchRequest", Message: "The batch request doesn't contain any entries."}
	app.SqsErrors["EmptyBatchRequest"] = err7
	err8 := app.SqsErrorType{HttpError: http.StatusBadRequest, Type: "ValidationError", Code: "AWS.SimpleQueueService.ValidationError", Message: "The visibility timeout is incorrect"}
	app.SqsErrors["InvalidVisibilityTimeout"] = err8
	err9 := app.SqsErrorType{HttpError: http.StatusBadRequest, Type: "MessageNotInFlight", Code: "AWS.SimpleQueueService.MessageNotInFlight", Message: "The message referred to isn't in flight."}
	app.SqsErrors["MessageNotInFlight"] = err9
	err10 := app.SqsErrorType{HttpError: http.StatusBadRequest, Type: "MessageTooBig", Code: "InvalidMessageContents", Message: "The message size exceeds the limit."}
	app.SqsErrors["MessageTooBig"] = err10
	app.SqsErrors[ErrInvalidParameterValue.Type] = *ErrInvalidParameterValue
	app.SqsErrors[ErrInvalidAttributeValue.Type] = *ErrInvalidAttributeValue
}

func PeriodicTasks(d time.Duration, quit <-chan struct{}) {
	ticker := time.NewTicker(d)
	for {
		select {
		case <-ticker.C:
			app.SyncQueues.Lock()
			for j := range app.SyncQueues.Queues {
				queue := app.SyncQueues.Queues[j]

				log.Debugf("Queue [%s] length [%d]", queue.Name, len(queue.Messages))
				for i := 0; i < len(queue.Messages); i++ {
					msg := &queue.Messages[i]

					// Reset deduplication period
					for dedupId, startTime := range queue.Duplicates {
						if time.Now().After(startTime.Add(app.DeduplicationPeriod)) {
							log.Debugf("deduplication period for message with deduplicationId [%s] expired", dedupId)
							delete(queue.Duplicates, dedupId)
						}
					}

					if msg.ReceiptHandle != "" {
						if msg.VisibilityTimeout.Before(time.Now()) {
							log.Debugf("Making message visible again %s", msg.ReceiptHandle)
							queue.UnlockGroup(msg.GroupID)
							msg.ReceiptHandle = ""
							msg.ReceiptTime = time.Now().UTC()
							msg.Retry++
							if queue.MaxReceiveCount > 0 &&
								queue.DeadLetterQueue != nil &&
								msg.Retry > queue.MaxReceiveCount {
								queue.DeadLetterQueue.Messages = append(queue.DeadLetterQueue.Messages, *msg)
								queue.Messages = append(queue.Messages[:i], queue.Messages[i+1:]...)
								i++
							}
						}
					}
				}
			}
			app.SyncQueues.Unlock()
		case <-quit:
			ticker.Stop()
			return
		}
	}
}

// commonFunc handles common functions.
// *Note: It parses the JSON-encoded data from Body in http.Request and stores the result in the value pointed to by v.
// If v is nil or not a pointer, unmarshal returns an InvalidUnmarshalError.
func commonFunc(w http.ResponseWriter, body io.ReadCloser, v any) {
	// Sent response type
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	// Unmarshal request body
	if body != nil {
		defer body.Close()
		b, err := io.ReadAll(body)
		if e := json.Unmarshal(b, v); e != nil {
			log.Errorf("failed to unmarshal: %+v", err)
			return
		}
	}
}

func ListQueues(w http.ResponseWriter, req *http.Request) {
	var v app.ListQueuesInput
	commonFunc(w, req.Body, &v)

	queueUrls := make([]string, 0)

	log.Println("Listing Queues")
	for _, queue := range app.SyncQueues.Queues {
		app.SyncQueues.Lock()
		if strings.HasPrefix(queue.Name, v.QueueNamePrefix) {
			queueUrls = append(queueUrls, queue.URL)
		}
		app.SyncQueues.Unlock()
	}
	respStruct := app.ListQueuesOutput{
		QueueUrls: queueUrls,
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(respStruct); err != nil {
		log.Printf("error: %v\n", err)
	}
}

func CreateQueue(w http.ResponseWriter, req *http.Request) {
	var v app.CreateQueueInput
	commonFunc(w, req.Body, &v)

	queueName := v.QueueName

	queueUrl := "http://" + app.CurrentEnvironment.Host + ":" + app.CurrentEnvironment.Port +
		"/" + app.CurrentEnvironment.AccountID + "/" + queueName
	if app.CurrentEnvironment.Region != "" {
		queueUrl = "http://" + app.CurrentEnvironment.Region + "." + app.CurrentEnvironment.Host + ":" +
			app.CurrentEnvironment.Port + "/" + app.CurrentEnvironment.AccountID + "/" + queueName
	}
	queueArn := "arn:aws:sqs:" + app.CurrentEnvironment.Region + ":" + app.CurrentEnvironment.AccountID + ":" + queueName

	if _, ok := app.SyncQueues.Queues[queueName]; !ok {
		log.Println("Creating Queue:", queueName)
		queue := &app.Queue{
			Name:                   queueName,
			URL:                    queueUrl,
			Arn:                    queueArn,
			TimeoutSecs:            app.CurrentEnvironment.QueueAttributeDefaults.VisibilityTimeout,
			ReceiveWaitTimeSecs:    app.CurrentEnvironment.QueueAttributeDefaults.ReceiveMessageWaitTimeSeconds,
			MaximumMessageSize:     app.CurrentEnvironment.QueueAttributeDefaults.MaximumMessageSize,
			IsFIFO:                 app.HasFIFOQueueName(queueName),
			EnableDuplicates:       app.CurrentEnvironment.EnableDuplicates,
			Duplicates:             make(map[string]time.Time),
			QueueOwnerAWSAccountId: app.CurrentEnvironment.AccountID,
		}
		if err := validateAndSetQueueAttributes(queue, v.Attributes); err != nil {
			createErrorResponse(w, req, err.Error())
			return
		}
		app.SyncQueues.Lock()
		app.SyncQueues.Queues[queueName] = queue
		app.SyncQueues.Unlock()
	}

	respStruct := app.CreateQueueOutput{
		QueueUrl: queueUrl,
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(respStruct); err != nil {
		log.Printf("error: %v\n", err)
	}
}

func SendMessage(w http.ResponseWriter, req *http.Request) {
	var v app.SendMessageInput
	commonFunc(w, req.Body, &v)

	messageBody := v.MessageBody
	messageGroupID := v.MessageGroupId
	messageDeduplicationID := v.MessageDeduplicationId
	messageAttributes := v.MessageAttributes
	messageSystemAttributes := v.MessageSystemAttributes

	queueUrl := getQueueFromPath(v.QueueUrl, req.URL.String())

	queueName := ""
	if queueUrl == "" {
		vars := mux.Vars(req)
		queueName = vars["queueName"]
	} else {
		uriSegments := strings.Split(queueUrl, "/")
		queueName = uriSegments[len(uriSegments)-1]
	}

	if _, ok := app.SyncQueues.Queues[queueName]; !ok {
		// Queue does not exist
		createErrorResponse(w, req, "QueueNotFound")
		return
	}

	if app.SyncQueues.Queues[queueName].MaximumMessageSize > 0 &&
		len(messageBody) > app.SyncQueues.Queues[queueName].MaximumMessageSize {
		// Message size is too big
		createErrorResponse(w, req, "MessageTooBig")
		return
	}

	delaySecs := app.SyncQueues.Queues[queueName].DelaySecs
	if mv := v.DelaySeconds; mv != nil {
		delaySecs = int(*mv)
	}

	log.Println("Putting Message in Queue:", queueName)
	msg := app.Message{MessageBody: []byte(messageBody)}
	if len(messageAttributes) > 0 {
		msg.MessageAttributes = messageAttributes
		msg.MD5OfMessageAttributes = common.HashAttributes(messageAttributes)
	}
	if len(messageSystemAttributes) > 0 {
		msg.MessageSystemAttributes = messageSystemAttributes
		msg.MD5OfMessageSystemAttributes = common.HashAttributes(messageSystemAttributes)
	}
	msg.MD5OfMessageBody = common.GetMD5Hash(messageBody)
	msg.Uuid, _ = common.NewUUID()
	msg.GroupID = messageGroupID
	msg.DeduplicationID = messageDeduplicationID
	msg.SentTime = time.Now()
	msg.DelaySecs = delaySecs

	app.SyncQueues.Lock()
	fifoSeqNumber := ""
	if app.SyncQueues.Queues[queueName].IsFIFO {
		fifoSeqNumber = app.SyncQueues.Queues[queueName].NextSequenceNumber(messageGroupID)
	}

	if !app.SyncQueues.Queues[queueName].IsDuplicate(messageDeduplicationID) {
		app.SyncQueues.Queues[queueName].Messages = append(app.SyncQueues.Queues[queueName].Messages, msg)
	} else {
		log.Debugf("Message with deduplicationId [%s] in queue [%s] is duplicate ", messageDeduplicationID, queueName)
	}

	app.SyncQueues.Queues[queueName].InitDuplicatation(messageDeduplicationID)
	app.SyncQueues.Unlock()
	log.Infof("%s: Queue: %s, Message: %s\n", time.Now().Format("2006-01-02 15:04:05"), queueName, msg.MessageBody)

	respStruct := app.SendMessageOutput{
		MD5OfMessageAttributes:       msg.MD5OfMessageAttributes,
		MD5OfMessageBody:             msg.MD5OfMessageBody,
		MD5OfMessageSystemAttributes: msg.MD5OfMessageSystemAttributes,
		MessageId:                    msg.Uuid,
		SequenceNumber:               fifoSeqNumber,
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(respStruct); err != nil {
		log.Printf("error: %v\n", err)
	}
}

func SendMessageBatch(w http.ResponseWriter, req *http.Request) {
	var v app.SendMessageBatchInput
	commonFunc(w, req.Body, &v)

	queueUrl := getQueueFromPath(v.QueueUrl, req.URL.String())
	queueName := ""
	if queueUrl == "" {
		vars := mux.Vars(req)
		queueName = vars["queueName"]
	} else {
		uriSegments := strings.Split(queueUrl, "/")
		queueName = uriSegments[len(uriSegments)-1]
	}

	if _, ok := app.SyncQueues.Queues[queueName]; !ok {
		createErrorResponse(w, req, "QueueNotFound")
		return
	}

	sendEntries := v.Entries

	if len(sendEntries) == 0 {
		createErrorResponse(w, req, "EmptyBatchRequest")
		return
	}

	if len(sendEntries) > 10 {
		createErrorResponse(w, req, "TooManyEntriesInBatchRequest")
		return
	}
	ids := map[string]struct{}{}
	for _, v := range sendEntries {
		if _, ok := ids[v.Id]; ok {
			createErrorResponse(w, req, "BatchEntryIdsNotDistinct")
			return
		}
		ids[v.Id] = struct{}{}
	}

	sentEntries := make([]*app.SendMessageBatchResultEntry, len(sendEntries))
	log.Println("Putting Message in Queue:", queueName)
	for i, sendEntry := range sendEntries {
		msg := app.Message{MessageBody: []byte(sendEntry.MessageBody)}
		if len(sendEntry.MessageAttributes) > 0 {
			msg.MessageAttributes = sendEntry.MessageAttributes
			msg.MD5OfMessageAttributes = common.HashAttributes(sendEntry.MessageAttributes)
		}
		if len(sendEntry.MessageSystemAttributes) > 0 {
			msg.MessageAttributes = sendEntry.MessageAttributes
			msg.MD5OfMessageSystemAttributes = common.HashAttributes(sendEntry.MessageSystemAttributes)
		}
		msg.MD5OfMessageBody = common.GetMD5Hash(sendEntry.MessageBody)
		msg.GroupID = sendEntry.MessageGroupId
		msg.DeduplicationID = sendEntry.MessageDeduplicationId
		msg.Uuid, _ = common.NewUUID()
		msg.SentTime = time.Now()
		msg.DelaySecs = int(sendEntry.DelaySeconds)
		app.SyncQueues.Lock()
		fifoSeqNumber := ""
		if app.SyncQueues.Queues[queueName].IsFIFO {
			fifoSeqNumber = app.SyncQueues.Queues[queueName].NextSequenceNumber(sendEntry.MessageGroupId)
		}

		if !app.SyncQueues.Queues[queueName].IsDuplicate(sendEntry.MessageDeduplicationId) {
			app.SyncQueues.Queues[queueName].Messages = append(app.SyncQueues.Queues[queueName].Messages, msg)
		} else {
			log.Debugf("Message with deduplicationId [%s] in queue [%s] is duplicate ", sendEntry.MessageDeduplicationId, queueName)
		}

		app.SyncQueues.Queues[queueName].InitDuplicatation(sendEntry.MessageDeduplicationId)

		app.SyncQueues.Unlock()
		sentEntries[i] = &app.SendMessageBatchResultEntry{
			Id:                           sendEntry.Id,
			MessageId:                    msg.Uuid,
			MD5OfMessageBody:             msg.MD5OfMessageBody,
			MD5OfMessageAttributes:       msg.MD5OfMessageAttributes,
			MD5OfMessageSystemAttributes: msg.MD5OfMessageSystemAttributes,
			SequenceNumber:               fifoSeqNumber,
		}
		log.Infof("%s: Queue: %s, Message: %s\n", time.Now().Format("2006-01-02 15:04:05"), queueName, msg.MessageBody)
	}

	respStruct := app.SendMessageBatchOutput{
		Successful: sentEntries,
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(respStruct); err != nil {
		log.Printf("error: %v\n", err)
	}
}

func ReceiveMessage(w http.ResponseWriter, req *http.Request) {
	var v app.ReceiveMessageInput
	commonFunc(w, req.Body, &v)

	names := v.AttributeNames
	maxNumberOfMessages := 1
	if mom := v.MaxNumberOfMessages; mom != 0 {
		maxNumberOfMessages = int(mom)
	}
	mNames := v.MessageAttributeNames
	vt := v.VisibilityTimeout

	queueUrl := getQueueFromPath(v.QueueUrl, req.URL.String())

	queueName := ""
	if queueUrl == "" {
		vars := mux.Vars(req)
		queueName = vars["queueName"]
	} else {
		uriSegments := strings.Split(queueUrl, "/")
		queueName = uriSegments[len(uriSegments)-1]
	}

	if _, ok := app.SyncQueues.Queues[queueName]; !ok {
		createErrorResponse(w, req, "QueueNotFound")
		return
	}

	var messages []*app.MessageOutput
	//	respMsg := MessageOutput{}
	respStruct := app.ReceiveMessageOutput{}
	var waitTimeSeconds int
	if wts := v.WaitTimeSeconds; wts == nil {
		app.SyncQueues.RLock()
		waitTimeSeconds = app.SyncQueues.Queues[queueName].ReceiveWaitTimeSecs
		app.SyncQueues.RUnlock()
	} else {
		waitTimeSeconds = int(*wts)
	}

	loops := waitTimeSeconds * 10
	for loops > 0 {
		app.SyncQueues.RLock()
		_, queueFound := app.SyncQueues.Queues[queueName]
		if !queueFound {
			app.SyncQueues.RUnlock()
			createErrorResponse(w, req, "QueueNotFound")
			return
		}
		messageFound := len(app.SyncQueues.Queues[queueName].Messages)-numberOfHiddenMessagesInQueue(*app.SyncQueues.Queues[queueName]) != 0
		app.SyncQueues.RUnlock()
		if !messageFound {
			continueTimer := time.NewTimer(100 * time.Millisecond)
			select {
			case <-req.Context().Done():
				continueTimer.Stop()
				return // client gave up
			case <-continueTimer.C:
				continueTimer.Stop()
			}
			loops--
		} else {
			break
		}

	}
	log.Println("Getting Message from Queue:", queueName)

	app.SyncQueues.Lock() // Lock the Queues
	if len(app.SyncQueues.Queues[queueName].Messages) > 0 {
		numMsg := 0
		messages = make([]*app.MessageOutput, 0)
		for i := range app.SyncQueues.Queues[queueName].Messages {
			if numMsg >= maxNumberOfMessages {
				break
			}

			if app.SyncQueues.Queues[queueName].Messages[i].ReceiptHandle != "" {
				continue
			}

			uuid, _ := common.NewUUID()

			msg := &app.SyncQueues.Queues[queueName].Messages[i]
			if !msg.IsReadyForReceipt() {
				continue
			}
			msg.ReceiptHandle = msg.Uuid + "#" + uuid
			msg.ReceiptTime = time.Now().UTC()
			duration := time.Duration(app.SyncQueues.Queues[queueName].TimeoutSecs)
			if vt != nil {
				duration = time.Duration(*vt)
			}
			msg.VisibilityTimeout = time.Now().Add(duration * time.Second)

			if app.SyncQueues.Queues[queueName].IsFIFO {
				// If we got messages here it means we have not processed it yet, so get next
				if app.SyncQueues.Queues[queueName].IsLocked(msg.GroupID) {
					continue
				}
				// Otherwise lock messages for group ID
				app.SyncQueues.Queues[queueName].LockGroup(msg.GroupID)
			}

			mAttrs := make(map[string]*app.MessageAttributeValue)
			for _, n := range mNames {
				mAttrs[n] = msg.MessageAttributes[n]
			}

			messages = append(messages, &app.MessageOutput{
				MessageId:              msg.Uuid,
				Body:                   string(msg.MessageBody),
				ReceiptHandle:          msg.ReceiptHandle,
				MD5OfBody:              common.GetMD5Hash(string(msg.MessageBody)),
				MD5OfMessageAttributes: msg.MD5OfMessageAttributes,
				MessageAttributes:      mAttrs,
				Attributes:             msg.ExtractAttributes(names),
			})

			numMsg++
		}

		//		respMsg = MessageOutput{MessageId: messages.Uuid, ReceiptHandle: messages.ReceiptHandle, MD5OfBody: messages.MD5OfMessageBody, Body: messages.MessageBody, MD5OfMessageAttributes: messages.MD5OfMessageAttributes}
		respStruct = app.ReceiveMessageOutput{
			Messages: messages,
		}
	} else {
		log.Println("No messages in Queue:", queueName)
		respStruct = app.ReceiveMessageOutput{}
	}
	app.SyncQueues.Unlock() // Unlock the Queues
	enc := json.NewEncoder(w)
	if err := enc.Encode(respStruct); err != nil {
		log.Printf("error: %v\n", err)
	}
}

func numberOfHiddenMessagesInQueue(queue app.Queue) int {
	num := 0
	for _, m := range queue.Messages {
		if m.ReceiptHandle != "" || m.DelaySecs > 0 && time.Now().Before(m.SentTime.Add(time.Duration(m.DelaySecs)*time.Second)) {
			num++
		}
	}
	return num
}

func ChangeMessageVisibility(w http.ResponseWriter, req *http.Request) {
	var v app.ChangeMessageVisibilityInput
	commonFunc(w, req.Body, &v)

	visibilityTimeout := v.VisibilityTimeout
	if visibilityTimeout > 43200 {
		createErrorResponse(w, req, "ValidationError")
		return
	}

	queueUrl := getQueueFromPath(v.QueueUrl, req.URL.String())
	queueName := ""
	if queueUrl == "" {
		vars := mux.Vars(req)
		queueName = vars["queueName"]
	} else {
		uriSegments := strings.Split(queueUrl, "/")
		queueName = uriSegments[len(uriSegments)-1]
	}

	if _, ok := app.SyncQueues.Queues[queueName]; !ok {
		createErrorResponse(w, req, "QueueNotFound")
		return
	}

	app.SyncQueues.Lock()
	messageFound := false
	for i := 0; i < len(app.SyncQueues.Queues[queueName].Messages); i++ {
		queue := app.SyncQueues.Queues[queueName]
		msgs := queue.Messages
		if msgs[i].ReceiptHandle == v.ReceiptHandle {
			timeout := app.SyncQueues.Queues[queueName].TimeoutSecs
			if visibilityTimeout == 0 {
				msgs[i].ReceiptTime = time.Now().UTC()
				msgs[i].ReceiptHandle = ""
				msgs[i].VisibilityTimeout = time.Now().Add(time.Duration(timeout) * time.Second)
				msgs[i].Retry++
				if queue.MaxReceiveCount > 0 &&
					queue.DeadLetterQueue != nil &&
					msgs[i].Retry > queue.MaxReceiveCount {
					queue.DeadLetterQueue.Messages = append(queue.DeadLetterQueue.Messages, msgs[i])
					queue.Messages = append(queue.Messages[:i], queue.Messages[i+1:]...)
					i++
				}
			} else {
				msgs[i].VisibilityTimeout = time.Now().Add(time.Duration(visibilityTimeout) * time.Second)
			}
			messageFound = true
			break
		}
	}
	app.SyncQueues.Unlock()
	if !messageFound {
		createErrorResponse(w, req, "MessageNotInFlight")
		return
	}
}

func DeleteMessageBatch(w http.ResponseWriter, req *http.Request) {
	var v app.DeleteMessageBatchInput
	commonFunc(w, req.Body, &v)

	queueUrl := getQueueFromPath(v.QueueUrl, req.URL.String())
	queueName := ""
	if queueUrl == "" {
		vars := mux.Vars(req)
		queueName = vars["queueName"]
	} else {
		uriSegments := strings.Split(queueUrl, "/")
		queueName = uriSegments[len(uriSegments)-1]
	}

	deleteEntries := v.Entries

	deletedEntries := make([]*app.DeleteMessageBatchResultEntry, 0)

	app.SyncQueues.Lock()
	if _, ok := app.SyncQueues.Queues[queueName]; ok {
		for _, deleteEntry := range deleteEntries {
			for i, msg := range app.SyncQueues.Queues[queueName].Messages {
				if msg.ReceiptHandle == deleteEntry.ReceiptHandle {
					// Unlock messages for the group
					log.Printf("FIFO Queue %s unlocking group %s:", queueName, msg.GroupID)
					app.SyncQueues.Queues[queueName].UnlockGroup(msg.GroupID)
					app.SyncQueues.Queues[queueName].Messages = append(app.SyncQueues.Queues[queueName].Messages[:i], app.SyncQueues.Queues[queueName].Messages[i+1:]...)
					delete(app.SyncQueues.Queues[queueName].Duplicates, msg.DeduplicationID)

					deleteEntry.Deleted = true
					deletedEntries = append(deletedEntries, &app.DeleteMessageBatchResultEntry{
						Id: deleteEntry.Id,
					})
					break
				}
			}
		}
	}
	app.SyncQueues.Unlock()

	notFoundEntries := make([]*app.BatchResultErrorEntry, 0)
	for _, deleteEntry := range deleteEntries {
		if deleteEntry.Deleted {
			notFoundEntries = append(notFoundEntries, &app.BatchResultErrorEntry{
				Code:        "1",
				Id:          deleteEntry.Id,
				Message:     "Message not found",
				SenderFault: true})
		}
	}

	respStruct := app.DeleteMessageBatchOutput{
		Successful: deletedEntries,
		Failed:     notFoundEntries,
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(respStruct); err != nil {
		log.Printf("error: %v\n", err)
	}
}

func DeleteMessage(w http.ResponseWriter, req *http.Request) {
	var v app.DeleteMessageInput
	commonFunc(w, req.Body, &v)

	receiptHandle := v.ReceiptHandle

	queueUrl := getQueueFromPath(v.QueueUrl, req.URL.String())
	queueName := ""
	if queueUrl == "" {
		vars := mux.Vars(req)
		queueName = vars["queueName"]
	} else {
		uriSegments := strings.Split(queueUrl, "/")
		queueName = uriSegments[len(uriSegments)-1]
	}

	log.Println("Deleting Message, Queue:", queueName, ", ReceiptHandle:", receiptHandle)

	// Find queue/message with the receipt handle and delete
	app.SyncQueues.Lock()
	if _, ok := app.SyncQueues.Queues[queueName]; ok {
		for i, msg := range app.SyncQueues.Queues[queueName].Messages {
			if msg.ReceiptHandle == receiptHandle {
				// Unlock messages for the group
				log.Printf("FIFO Queue %s unlocking group %s:", queueName, msg.GroupID)
				app.SyncQueues.Queues[queueName].UnlockGroup(msg.GroupID)
				//Delete message from Q
				app.SyncQueues.Queues[queueName].Messages = append(app.SyncQueues.Queues[queueName].Messages[:i], app.SyncQueues.Queues[queueName].Messages[i+1:]...)
				delete(app.SyncQueues.Queues[queueName].Duplicates, msg.DeduplicationID)

				app.SyncQueues.Unlock()
				return
			}
		}
		log.Println("Receipt Handle not found")
	} else {
		log.Println("Queue not found")
	}
	app.SyncQueues.Unlock()

	createErrorResponse(w, req, "MessageDoesNotExist")
}

func DeleteQueue(w http.ResponseWriter, req *http.Request) {
	var v app.DeleteQueueInput
	commonFunc(w, req.Body, &v)

	queueUrl := getQueueFromPath(v.QueueUrl, req.URL.String())
	queueName := ""
	if queueUrl == "" {
		vars := mux.Vars(req)
		queueName = vars["queueName"]
	} else {
		uriSegments := strings.Split(queueUrl, "/")
		queueName = uriSegments[len(uriSegments)-1]
	}

	log.Println("Deleting Queue:", queueName)
	app.SyncQueues.Lock()
	delete(app.SyncQueues.Queues, queueName)
	app.SyncQueues.Unlock()
}

func PurgeQueue(w http.ResponseWriter, req *http.Request) {
	var v app.PurgeQueueInput
	commonFunc(w, req.Body, &v)

	queueUrl := getQueueFromPath(v.QueueUrl, req.URL.String())

	uriSegments := strings.Split(queueUrl, "/")
	queueName := uriSegments[len(uriSegments)-1]

	log.Println("Purging Queue:", queueName)

	app.SyncQueues.Lock()
	if _, ok := app.SyncQueues.Queues[queueName]; ok {
		app.SyncQueues.Queues[queueName].Messages = nil
		app.SyncQueues.Queues[queueName].Duplicates = make(map[string]time.Time)
	} else {
		log.Println("Purge Queue:", queueName, ", queue does not exist!!!")
		createErrorResponse(w, req, "QueueNotFound")
	}
	app.SyncQueues.Unlock()
}

func GetQueueUrl(w http.ResponseWriter, req *http.Request) {
	var v app.GetQueueUrlInput
	commonFunc(w, req.Body, &v)

	queueName := v.QueueName
	accountId := v.QueueOwnerAWSAccountId
	if queue, ok := app.SyncQueues.Queues[queueName]; ok && (accountId == "" || accountId == queue.QueueOwnerAWSAccountId) {
		url := queue.URL
		log.Println("Get Queue URL:", queueName)
		// Create, encode/xml and send response
		respStruct := app.GetQueueUrlOutput{QueueUrl: url}
		enc := json.NewEncoder(w)
		if err := enc.Encode(respStruct); err != nil {
			log.Printf("error: %v\n", err)
		}
	} else {
		log.Println("Get Queue URL:", queueName, ", queue does not exist!!!")
		createErrorResponse(w, req, "QueueNotFound")
	}
}

func GetQueueAttributes(w http.ResponseWriter, req *http.Request) {
	var v app.GetQueueAttributesInput
	commonFunc(w, req.Body, &v)

	attribute_names := make(map[string]bool)
	for _, value := range v.AttributeNames {
		attribute_names[value] = true
	}

	include_attr := func(a string) bool {
		if len(attribute_names) == 0 {
			return true
		}
		if _, ok := attribute_names[a]; ok {
			return true
		}
		if _, ok := attribute_names["All"]; ok {
			return true
		}
		return false
	}

	queueUrl := getQueueFromPath(v.QueueUrl, req.URL.String())

	queueName := ""
	if queueUrl == "" {
		vars := mux.Vars(req)
		queueName = vars["queueName"]
	} else {
		uriSegments := strings.Split(queueUrl, "/")
		queueName = uriSegments[len(uriSegments)-1]
	}

	log.Println("Get Queue Attributes:", queueName)
	app.SyncQueues.RLock()
	if queue, ok := app.SyncQueues.Queues[queueName]; ok {
		// Create, encode/xml and send response
		attribs := make(app.Attributes)
		if include_attr("VisibilityTimeout") {
			attribs["VisibilityTimeout"] = strconv.Itoa(queue.TimeoutSecs)
		}
		if include_attr("DelaySeconds") {
			attribs["DelaySeconds"] = strconv.Itoa(queue.DelaySecs)
		}
		if include_attr("ReceiveMessageWaitTimeSeconds") {
			attribs["ReceiveMessageWaitTimeSeconds"] = strconv.Itoa(queue.ReceiveWaitTimeSecs)
		}
		if include_attr("ApproximateNumberOfMessages") {
			attribs["ApproximateNumberOfMessages"] = strconv.Itoa(len(queue.Messages))
		}
		if include_attr("ApproximateNumberOfMessagesNotVisible") {
			attribs["ApproximateNumberOfMessagesNotVisible"] = strconv.Itoa(numberOfHiddenMessagesInQueue(*queue))
		}
		if include_attr("CreatedTimestamp") {
			attribs["CreatedTimestamp"] = "0000000000"
		}
		if include_attr("LastModifiedTimestamp") {
			attribs["LastModifiedTimestamp"] = "0000000000"
		}
		if include_attr("QueueArn") {
			attribs["QueueArn"] = queue.Arn
		}

		deadLetterTargetArn := ""
		if queue.DeadLetterQueue != nil {
			deadLetterTargetArn = queue.DeadLetterQueue.Name
		}
		if include_attr("RedrivePolicy") {
			attribs["RedrivePolicy"] = fmt.Sprintf(`{"maxReceiveCount":"%d","deadLetterTargetArn":"%s"}`, queue.MaxReceiveCount, deadLetterTargetArn)
		}

		respStruct := app.GetQueueAttributesOutput{Attrs: attribs}
		enc := json.NewEncoder(w)
		if err := enc.Encode(respStruct); err != nil {
			log.Printf("error: %v\n", err)
		}
	} else {
		log.Println("Get Queue URL:", queueName, ", queue does not exist!!!")
		createErrorResponse(w, req, "QueueNotFound")
	}
	app.SyncQueues.RUnlock()
}

func SetQueueAttributes(w http.ResponseWriter, req *http.Request) {
	var v app.SetQueueAttributesInput
	commonFunc(w, req.Body, &v)

	queueUrl := getQueueFromPath(v.QueueUrl, req.URL.String())

	queueName := ""
	if queueUrl == "" {
		vars := mux.Vars(req)
		queueName = vars["queueName"]
	} else {
		uriSegments := strings.Split(queueUrl, "/")
		queueName = uriSegments[len(uriSegments)-1]
	}

	log.Println("Set Queue Attributes:", queueName)
	app.SyncQueues.Lock()
	if queue, ok := app.SyncQueues.Queues[queueName]; ok {
		if err := validateAndSetQueueAttributes(queue, v.Attributes); err != nil {
			createErrorResponse(w, req, err.Error())
			app.SyncQueues.Unlock()
			return
		}
	} else {
		log.Println("Get Queue URL:", queueName, ", queue does not exist!!!")
		createErrorResponse(w, req, "QueueNotFound")
	}
	app.SyncQueues.Unlock()
}

func getQueueFromPath(reqUrl string, theUrl string) string {
	if reqUrl != "" {
		return reqUrl
	}
	u, err := url.Parse(theUrl)
	if err != nil {
		return ""
	}
	return u.Path
}

func createErrorResponse(w http.ResponseWriter, req *http.Request, err string) {
	er := app.SqsErrors[err]
	respStruct := app.ErrorResult{
		Type:    er.Type,
		Code:    er.Code,
		Message: er.Message,
	}

	w.WriteHeader(er.HttpError)
	enc := json.NewEncoder(w)
	if err := enc.Encode(respStruct); err != nil {
		log.Printf("error: %v\n", err)
	}
}
