package main

import (
	"context"
	"testing"

	"github.com/buildkite/go-buildkite/buildkite"
	"github.com/mrmod/gerrit-buildkite/backend"
)

type MockedInterface struct {
	FunctionCallCounter map[string]int
}

func (m *MockedInterface) Reset(name string) {
	m.FunctionCallCounter[name] = 0
}
func (m *MockedInterface) ResetAll() {
	m.FunctionCallCounter = map[string]int{}
}

type MockPipeline struct {
	MockCreateBuild func(*buildkite.CreateBuild) (int, error)
	MockCancelBuild func(int) error
	*MockedInterface
}

func (m MockPipeline) CreateBuild(build *buildkite.CreateBuild) (int, error) {
	m.FunctionCallCounter["CreateBuild"]++
	return m.MockCreateBuild(build)
}
func (m MockPipeline) CancelBuild(buildNumber int) error {
	m.FunctionCallCounter["CancelBuild"]++
	return m.MockCancelBuild(buildNumber)
}

type MockBackend struct {
	MockSaveBuild func(context.Context, *backend.PatchBuild) error
	MockGetBuild  func(ctx context.Context, buildNumber int) (*backend.PatchBuild, error)
	MockGetPatch  func(context.Context, *backend.Patch) (*backend.PatchBuild, error)
	*MockedInterface
}

func (b MockBackend) SaveBuild(ctx context.Context, pb *backend.PatchBuild) error {
	b.FunctionCallCounter["SaveBuild"]++
	return b.MockSaveBuild(ctx, pb)
}
func (b MockBackend) GetBuild(ctx context.Context, buildNumber int) (*backend.PatchBuild, error) {
	b.FunctionCallCounter["GetBuild"]++
	return b.MockGetBuild(ctx, buildNumber)
}
func (b MockBackend) GetPatch(ctx context.Context, p *backend.Patch) (*backend.PatchBuild, error) {
	b.FunctionCallCounter["GetPatch"]++
	return b.MockGetPatch(ctx, p)
}
func NewMockPipeline() MockPipeline {
	return MockPipeline{
		MockedInterface: &MockedInterface{map[string]int{}},
		MockCreateBuild: func(build *buildkite.CreateBuild) (int, error) {
			return 1, nil
		},
		MockCancelBuild: func(buildNumber int) error {
			return nil
		},
	}
}
func NewMockBackend() MockBackend {
	return MockBackend{
		MockedInterface: &MockedInterface{map[string]int{}},
		MockSaveBuild: func(ctx context.Context, pb *backend.PatchBuild) error {
			return nil
		},
		MockGetBuild: func(ctx context.Context, buildNumber int) (*backend.PatchBuild, error) {
			return nil, nil
		},
		MockGetPatch: func(ctx context.Context, p *backend.Patch) (*backend.PatchBuild, error) {
			return nil, nil
		},
	}
}

func TestItCreatesABuildWhenTheCommentIsRetest(t *testing.T) {
	p := NewMockPipeline()
	b := NewMockBackend()
	event := Event{
		PatchSet: PatchSet{
			Number:   1,
			Revision: "123456",
		},
		Change: Change{
			Number: 1,
		},
		Comment: "retest",
	}

	// When it's just "retest"
	HandleCommentAdded(event, p, b)
	if p.FunctionCallCounter["CreateBuild"] != 1 {
		t.Errorf("Expected CreateBuild to be called once, but it was called %d times", p.FunctionCallCounter["CreateBuild"])
	}

	p.Reset("CreateBuild")
	// When retest is on a line by itself
	event.Comment = `
retest
		`
	HandleCommentAdded(event, p, b)
	if p.FunctionCallCounter["CreateBuild"] != 1 {
		t.Errorf("Expected CreateBuild to be called once, but it was called %d times", p.FunctionCallCounter["CreateBuild"])
	}

	p.Reset("CreateBuild")
	// When retest is on a line by itself in a larger comment
	event.Comment = `
Just to make sure
retest
When this is done the change can be merged
		`
	HandleCommentAdded(event, p, b)
	if p.FunctionCallCounter["CreateBuild"] != 1 {
		t.Errorf("Expected CreateBuild to be called once, but it was called %d times", p.FunctionCallCounter["CreateBuild"])
	}

	// Not when retest is within a line
	p.Reset("CreateBuild")
	event.Comment = "not retest"
	HandleCommentAdded(event, p, b)
	if c, ok := p.FunctionCallCounter["CreateBuild"]; ok && c != 0 {
		t.Errorf("Expected CreateBuild to be called zero times, but it was called %d times", p.FunctionCallCounter["CreateBuild"])
	}

	// Not when retest has leading whitespace
	event.Comment = " retest"
	HandleCommentAdded(event, p, b)
	if c, ok := p.FunctionCallCounter["CreateBuild"]; ok && c != 0 {
		t.Errorf("Expected CreateBuild to be called zero times, but it was called %d times", p.FunctionCallCounter["CreateBuild"])
	}
	// Not when retest has trailing whitespace
	event.Comment = "retest "
	HandleCommentAdded(event, p, b)
	if c, ok := p.FunctionCallCounter["CreateBuild"]; ok && c != 0 {
		t.Errorf("Expected CreateBuild to be called zero times, but it was called %d times", p.FunctionCallCounter["CreateBuild"])
	}
	// Note the leading whitespace on its own line
	event.Comment = `
	retest
	`
	HandleCommentAdded(event, p, b)
	if c, ok := p.FunctionCallCounter["CreateBuild"]; ok && c != 0 {
		t.Errorf("Expected CreateBuild to be called zero times, but it was called %d times", p.FunctionCallCounter["CreateBuild"])
	}
}
