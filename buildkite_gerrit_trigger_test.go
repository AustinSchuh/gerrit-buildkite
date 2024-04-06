package main

import (
	"log"
	"os"
	"testing"

	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

var (
	cannedCreateDatabase = []string{
		"create table if not exists buildkite (id text not null primary key, sha1 text, changeid text, changenumber integer, patchset integer);",
	}
)

func setupDatabase(t *testing.T, statements ...string) (*os.File, *sql.DB) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed in setup: %s", err)
	}
	fh, err := os.CreateTemp(cwd, "gepdb")
	if err != nil {
		t.Fatalf("failed in setup: %s", err)
	}

	db, err := sql.Open("sqlite3", fh.Name())
	if err != nil {
		t.Fatalf("failed in setup: %s", err)
	}

	if len(statements) > 0 {
		for _, statement := range statements {
			if _, err := db.Exec(statement); err != nil {
				t.Fatalf("failed in setup: executing '%s': %s", statement, err)
			}
			log.Printf("setup: executed %s", statement)
		}
	}

	return fh, db
}

func TestTryLatestBuildCases(t *testing.T) {
	type testCase struct {
		statements  []string
		expectation bool
		input       int
		output      string
	}

	testCases := []testCase{
		// Should find an existing build
		{
			statements: append(
				cannedCreateDatabase,
				[]string{"insert into buildkite (id, changenumber) values ('abc-123', 1234)"}...,
			),
			expectation: true,
			input:       1234,
			output:      "abc-123",
		},
		// Should not find a missing build
		{
			statements: append(
				cannedCreateDatabase,
				[]string{"insert into buildkite (id, changenumber) values ('abc-123', 1234)"}...,
			),
			expectation: false,
			input:       9999,
			output:      "abc-123",
		},
	}
	for id, tc := range testCases {
		dbFile, db := setupDatabase(t, tc.statements...)
		defer func() {
			db.Close()
			dbFile.Close()
			os.Remove(dbFile.Name())
		}()
		state := &State{
			DB: db,
		}
		buildUUID, ok := state.TryGetLatestBuild(tc.input)
		if ok != tc.expectation {
			t.Fatalf("expected %v for case %d but got %v", tc.expectation, id, ok)
		}
		if !tc.expectation {
			continue
		}
		if buildUUID != tc.output {
			t.Fatalf("expected %s buildUUID for case %d but got %s", tc.output, id, buildUUID)
		}
	}

}
