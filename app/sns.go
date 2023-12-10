package app

import (
	"sync"
)

type SnsErrorType struct {
	HttpError int
	Type      string
	Code      string
	Message   string
}

var SnsErrors map[string]SnsErrorType

type MsgAttr struct {
	Type  string
	Value string
}
type SNSMessage struct {
	Type              string
	Token             string `json:"Token",omitempty`
	MessageId         string
	TopicArn          string
	Subject           string
	Message           string
	Timestamp         string
	SignatureVersion  string
	Signature         string `json:"Signature",omitempty`
	SigningCertURL    string
	UnsubscribeURL    string
	SubscribeURL      string                            `json:"SubscribeURL",omitempty`
	MessageAttributes map[string]*MessageAttributeValue `json:"MessageAttributes",omitempty`
}

type Subscription struct {
	TopicArn        string
	Protocol        string
	SubscriptionArn string
	EndPoint        string
	Raw             bool
	FilterPolicy    *FilterPolicy
}

// only simple "ExactMatch" string policy is supported at the moment
type FilterPolicy map[string][]string

// IsSatisfiedBy checks if MessageAttributes passed to Topic satisfy FilterPolicy set by subscription
func (fp *FilterPolicy) IsSatisfiedBy(msgAttrs map[string]*MessageAttributeValue) bool {
	for policyAttrName, policyAttrValues := range *fp {
		attrValue, ok := msgAttrs[policyAttrName]
		if !ok {
			return false // the attribute has to be present in the message
		}

		var exists bool
		switch attrValue.DataType {
		case "String", "Number":
			exists = stringInSlice(policyAttrValues, attrValue.StringValue)
		case "String.Array":
			exists = stringInSlice(policyAttrValues, attrValue.StringListValues...)
		case "Binary":
			exists = stringInSlice(policyAttrValues, string(attrValue.BinaryValue))
		}

		if !exists {
			return false // the attribute value has to be among filtered ones
		}
	}

	return true
}

func stringInSlice(list []string, args ...string) bool {
	for _, l := range list {
		for _, a := range args {
			if l == a {
				return true
			}
		}
	}
	return false
}

type Topic struct {
	Name          string
	Arn           string
	Subscriptions []*Subscription
}

type (
	Protocol         string
	MessageStructure string
)

const (
	ProtocolHTTP    Protocol = "http"
	ProtocolHTTPS   Protocol = "https"
	ProtocolSQS     Protocol = "sqs"
	ProtocolDefault Protocol = "default"
)

const (
	MessageStructureJSON MessageStructure = "json"
)

// Predefined errors
const (
	ErrNoDefaultElementInJSON = "Invalid parameter: Message Structure - No default entry in JSON message body"
)

var SyncTopics = struct {
	sync.RWMutex
	Topics map[string]*Topic
}{Topics: make(map[string]*Topic)}
