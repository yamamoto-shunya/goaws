package app

type Attributes map[string]string

// ListQueuesInput imitates sqs.ListQueuesInput
type ListQueuesInput struct {
	MaxResults      int64  `json:"MaxResults,omitempty"`
	NextToken       string `json:"NextToken,omitempty"`
	QueueNamePrefix string `json:"QueueNamePrefix,omitempty"`
}

// ListQueuesOutput imitates sqs.ListQueuesOutput
type ListQueuesOutput struct {
	//TODO: unused
	NextToken string   `json:"NextToken,omitempty"`
	QueueUrls []string `json:"QueueUrls,omitempty"`
}

// CreateQueueInput imitates sqs.CreateQueueInput
type CreateQueueInput struct {
	Attributes Attributes        `json:"Attributes,omitempty"`
	QueueName  string            `json:"QueueName"`
	Tags       map[string]string `json:"Tags,omitempty"`
}

// CreateQueueOutput imitates sqs.CreateQueueOutput
type CreateQueueOutput struct {
	QueueUrl string `json:"QueueUrl,omitempty"`
}

// SendMessageInput imitates sqs.SendMessageInput
type SendMessageInput struct {
	DelaySeconds            *int64                            `json:"DelaySeconds,omitempty"`
	MessageAttributes       map[string]*MessageAttributeValue `json:"MessageAttributes,omitempty"`
	MessageBody             string                            `json:"MessageBody"`
	MessageDeduplicationId  string                            `json:"MessageDeduplicationId,omitempty"`
	MessageGroupId          string                            `json:"MessageGroupId,omitempty"`
	MessageSystemAttributes map[string]*MessageAttributeValue `json:"MessageSystemAttributes,omitempty"`
	QueueUrl                string                            `json:"QueueUrl"`
}

// SendMessageOutput imitates sqs.SendMessageOutput
type SendMessageOutput struct {
	MD5OfMessageAttributes       string `json:"MD5OfMessageAttributes,omitempty"`
	MD5OfMessageBody             string `json:"MD5OfMessageBody,omitempty"`
	MD5OfMessageSystemAttributes string `json:"MD5OfMessageSystemAttributes,omitempty"`
	MessageId                    string `json:"MessageId,omitempty"`
	SequenceNumber               string `json:"SequenceNumber,omitempty"`
}

// ReceiveMessageInput imitates sqs.ReceiveMessageInput
type ReceiveMessageInput struct {
	// This field is deprecated
	AttributeNames        []string `json:"AttributeNames,omitempty"`
	MaxNumberOfMessages   int64    `json:"MaxNumberOfMessages,omitempty"`
	MessageAttributeNames []string `json:"MessageAttributeNames,omitempty"`
	QueueUrl              string   `json:"QueueUrl"`
	//TODO: it's for FIFO
	ReceiveRequestAttemptId string `json:"ReceiveRequestAttemptId,omitempty"`
	VisibilityTimeout       *int64 `json:"VisibilityTimeout,omitempty"`
	WaitTimeSeconds         *int64 `json:"WaitTimeSeconds,omitempty"`
}

// ReceiveMessageOutput imitates sqs.ReceiveMessageOutput
type ReceiveMessageOutput struct {
	Messages []*MessageOutput `json:"Messages,omitempty"`
}

// MessageOutput imitates sqs.Message
type MessageOutput struct {
	Attributes             Attributes                        `json:"Attribute,omitempty"`
	Body                   string                            `json:"Body,omitempty"`
	MD5OfBody              string                            `json:"MD5OfBody,omitempty"`
	MD5OfMessageAttributes string                            `json:"MD5OfMessageAttributes,omitempty"`
	MessageAttributes      map[string]*MessageAttributeValue `json:"MessageAttribute,omitempty"`
	MessageId              string                            `json:"MessageId,omitempty"`
	ReceiptHandle          string                            `json:"ReceiptHandle,omitempty"`
}

// ChangeMessageVisibilityInput imitates sqs.ChangeMessageVisibilityInput
type ChangeMessageVisibilityInput struct {
	QueueUrl          string `json:"QueueUrl"`
	ReceiptHandle     string `json:"ReceiptHandle"`
	VisibilityTimeout int64  `json:"VisibilityTimeout"`
}

// DeleteMessageInput imitates sqs.DeleteMessageInput
type DeleteMessageInput struct {
	QueueUrl      string `json:"QueueUrl"`
	ReceiptHandle string `json:"ReceiptHandle"`
}

// DeleteQueueInput imitates sqs.DeleteQueueInput
type DeleteQueueInput struct {
	QueueUrl string `json:"QueueUrl"`
}

// DeleteMessageBatchInput imitates sqs.DeleteMessageBatchInput
type DeleteMessageBatchInput struct {
	Entries  []*DeleteMessageBatchRequestEntry `json:"Entries"`
	QueueUrl string                            `json:"QueueUrl"`
}

// DeleteMessageBatchRequestEntry imitates sqs.DeleteMessageBatchRequestEntry
type DeleteMessageBatchRequestEntry struct {
	Id            string `json:"Id"`
	ReceiptHandle string `json:"ReceiptHandle"`
	// It's used in the process, not included in output.
	Deleted bool `json:"-"`
}

// DeleteMessageBatchOutput imitates sqs.DeleteMessageBatchOutput
type DeleteMessageBatchOutput struct {
	Failed     []*BatchResultErrorEntry         `json:"Failed"`
	Successful []*DeleteMessageBatchResultEntry `json:"Successful"`
}

// BatchResultErrorEntry imitates sqs.BatchResultErrorEntry
type BatchResultErrorEntry struct {
	Code        string `json:"Code"`
	Id          string `json:"Id"`
	Message     string `json:"Message,omitempty"`
	SenderFault bool   `json:"SenderFault"`
}

// DeleteMessageBatchResultEntry imitates sqs.DeleteMessageBatchResultEntry
type DeleteMessageBatchResultEntry struct {
	Id string `json:"Id"`
}

// SendMessageBatchInput imitates sqs.SendMessageBatchInput
type SendMessageBatchInput struct {
	Entries  []*SendMessageBatchRequestEntry `json:"Entries"`
	QueueUrl string                          `json:"QueueUrl"`
}

// SendMessageBatchRequestEntry imitates sqs.SendMessageBatchRequestEntry
type SendMessageBatchRequestEntry struct {
	DelaySeconds            int64                             `json:"DelaySeconds,omitempty"`
	Id                      string                            `json:"Id"`
	MessageAttributes       map[string]*MessageAttributeValue `json:"MessageAttributes,omitempty"`
	MessageBody             string                            `json:"MessageBody"`
	MessageDeduplicationId  string                            `json:"MessageDeduplicationId,omitempty"`
	MessageGroupId          string                            `json:"MessageGroupId,omitempty"`
	MessageSystemAttributes map[string]*MessageAttributeValue `json:"MessageSystemAttributes,omitempty"`
}

// SendMessageBatchOutput imitates sqs.SendMessageBatchOutput
type SendMessageBatchOutput struct {
	Failed     []*BatchResultErrorEntry       `json:"Failed,omitempty"`
	Successful []*SendMessageBatchResultEntry `json:"Successful"`
}

// SendMessageBatchResultEntry imitates sqs.SendMessageBatchResultEntry
type SendMessageBatchResultEntry struct {
	Id                           string `json:"Id"`
	MD5OfMessageAttributes       string `json:"MD5OfMessageAttributes,omitempty"`
	MD5OfMessageBody             string `json:"MD5OfMessageBody"`
	MD5OfMessageSystemAttributes string `json:"MD5OfMessageSystemAttributes,omitempty"`
	MessageId                    string `json:"MessageId"`
	SequenceNumber               string `json:"SequenceNumber,omitempty"`
}

// PurgeQueueInput imitates sqs.PurgeQueueInput
type PurgeQueueInput struct {
	QueueUrl string `json:"QueueUrl"`
}

// GetQueueUrlInput imitates sqs.GetQueueUrlInput
type GetQueueUrlInput struct {
	QueueName              string `json:"QueueName"`
	QueueOwnerAWSAccountId string `json:"QueueOwnerAWSAccountId,omitempty"`
}

// GetQueueUrlOutput imitates sqs.GetQueueUrlOutput
type GetQueueUrlOutput struct {
	QueueUrl string `json:"QueueUrl,omitempty"`
}

// GetQueueAttributesInput imitates sqs.GetQueueAttributesInput
type GetQueueAttributesInput struct {
	AttributeNames []string `json:"AttributeNames,omitempty"`
	QueueUrl       string   `json:"QueueUrl"`
}

// GetQueueAttributesOutput imitates sqs.GetQueueAttributesOutput
type GetQueueAttributesOutput struct {
	/* VisibilityTimeout, DelaySeconds, ReceiveMessageWaitTimeSeconds, ApproximateNumberOfMessages
	   ApproximateNumberOfMessagesNotVisible, CreatedTimestamp, LastModifiedTimestamp, QueueArn */
	Attrs Attributes `json:"Attributes,omitempty"`
}

// SetQueueAttributesInput imitates sqs.SetQueueAttributesInput
type SetQueueAttributesInput struct {
	Attributes Attributes `json:"Attributes"`
	QueueUrl   string     `json:"QueueUrl"`
}
