package app

import (
	"testing"
)

func TestFilterPolicy_IsSatisfiedBy(t *testing.T) {
	var tests = []struct {
		filterPolicy      *FilterPolicy
		messageAttributes map[string]*MessageAttributeValue
		expected          bool
	}{
		{
			&FilterPolicy{"foo": {"bar"}},
			map[string]*MessageAttributeValue{"foo": {DataType: "String", StringValue: "bar"}},
			true,
		},
		{
			&FilterPolicy{"foo": {"bar", "xyz"}},
			map[string]*MessageAttributeValue{"foo": {DataType: "String", StringValue: "xyz"}},
			true,
		},
		{
			&FilterPolicy{"foo": {"bar", "xyz"}, "abc": {"def"}},
			map[string]*MessageAttributeValue{"foo": {DataType: "String", StringValue: "xyz"},
				"abc": {DataType: "String", StringValue: "def"}},
			true,
		},
		{
			&FilterPolicy{"foo": {"bar"}},
			map[string]*MessageAttributeValue{"foo": {DataType: "String", StringValue: "baz"}},
			false,
		},
		{
			&FilterPolicy{"foo": {"bar"}},
			map[string]*MessageAttributeValue{},
			false,
		},
		{
			&FilterPolicy{"foo": {"bar"}, "abc": {"def"}},
			map[string]*MessageAttributeValue{"foo": {DataType: "String", StringValue: "bar"}},
			false,
		},
		{
			&FilterPolicy{"foo": {"bar"}},
			map[string]*MessageAttributeValue{"foo": {DataType: "Binary", StringValue: "bar"}},
			false,
		},
	}

	for i, tt := range tests {
		actual := tt.filterPolicy.IsSatisfiedBy(tt.messageAttributes)
		if tt.filterPolicy.IsSatisfiedBy(tt.messageAttributes) != tt.expected {
			t.Errorf("#%d FilterPolicy: expected %t, actual %t", i, tt.expected, actual)
		}
	}

}
