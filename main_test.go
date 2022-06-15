package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPluckUrls(t *testing.T) {
	messageText := `:eyes: would love some reviews pls:
	• <https://gitlab.com/org/namespace/proj1/-/merge_requests/11>
	• <https://gitlab.com/org/namespace/proj2/-/merge_requests/251>`
	expected := []*GitLabMR{
		{"namespace", "proj1", 11},
		{"namespace", "proj2", 251},
	}
	actual, err := pluckUrls(messageText)
	assert.Nil(t, err, "expect error to be nil")
	assert.Equal(t, expected, actual, "should parse the MRs correctly")
}
