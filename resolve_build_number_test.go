package main

import (
	"os"
	"testing"

	"github.com/buildkite/go-buildkite/buildkite"
)

func TestResolveBuildNumber(t *testing.T) {
	apiToken := os.Getenv("BUILDKITE_API_TOKEN")

	config, err := buildkite.NewTokenConfig(apiToken, true)
	if err != nil {
		t.Fatalf("failed to create Buildkite config: %s", err)
	}
	// Warning: This is a hot call
	client := config.Client()

	type testCase struct {
		expectation bool
		output      int
		input       string
	}

	// Warning: These are static values and will not
	// work for you. Maybe you want a fixture loader?
	testCases := []testCase{
		// Should find the build number for the input UUID
		{
			expectation: true,
			output:      44842,
			input:       "018e5da3-f391-49a0-bc5d-6a5f60b7ef30",
		},
		// Should not find the build number for an absent UUID
		{
			expectation: false,
			output:      9000,
			input:       "fake-test-fake-test-fake",
		},
	}

	for id, tc := range testCases {
		buildNumber, ok := resolveBuildNumber(client, tc.input)
		if ok != tc.expectation {
			t.Fatalf("expected to find build uuid %s for case %d but failed", tc.input, id)
		}
		if !tc.expectation {
			continue
		}
		if buildNumber != tc.output {
			t.Fatalf("expected buildNumber %d for case %d but got %d", tc.output, id, buildNumber)
		}
	}
}
