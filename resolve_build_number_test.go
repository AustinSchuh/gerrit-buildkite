package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockBuildkiteGraphqlApi struct {
	response *http.Response
	err      error
}

func (m *mockBuildkiteGraphqlApi) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.response, m.err
}

func TestResolveBuildNumber(t *testing.T) {
	type testCase struct {
		expectation bool
		output      int
		input       string
		mock        *mockBuildkiteGraphqlApi
	}

	mockApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	defer mockApi.Close()

	testCases := []testCase{
		// Should find the build number for the input UUID
		{
			expectation: true,
			output:      9999,
			input:       "fake-test-fake-test-fake",
			mock: &mockBuildkiteGraphqlApi{
				response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"data": {"build": {"number": 9999}}}`)),
				},
				err: nil,
			},
		},
		// Should not find the build number for an absent UUID
		{
			expectation: false,
			output:      0,
			input:       "fake-test-fake-test-fake",
			mock: &mockBuildkiteGraphqlApi{
				response: &http.Response{
					// Remote API worked
					StatusCode: http.StatusOK,
					// GraphQL had no builds matching the "uuid"
					Body: io.NopCloser(strings.NewReader(`{"data": {"build": {"number": 0}}}`)),
				},
				err: nil,
			},
		},
	}

	for id, tc := range testCases {
		client := mockApi.Client()
		client.Transport = tc.mock
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
