package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/buildkite/go-buildkite/buildkite"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Commit struct {
	Sha1         string
	ChangeId     string
	ChangeNumber int
	Patchset     int
}

type State struct {
	// This mutex needs to be locked across anything which generates a uuid or calls {Get,Add}Commit.
	mu sync.Mutex

	User string
	Key  string
	// Webhook token expected to service requests
	Token string
	// Gerrit server to connect to.
	Server string
	// Project in gerrit to only accept events from.
	Project string
	// BuildkiteProject in gerrit to only accept events from.
	BuildkiteProject string
	// Organization to use in Buildkite for the build.
	BuildkiteOrganization string

	// Database to hold commits.
	DB *sql.DB
}

func (s *State) OpenDatabase(database string) {
	db, err := sql.Open("sqlite3", database)

	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Create a build counter table with a key of "id", and a "count" column only if it doesn't exist.
	sqlStmt := `
        create table if not exists buildkite (id text not null primary key, sha1 text, changeid text, changenumber integer, patchset integer);
        `
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Fatalf("%q: %s\n", err, sqlStmt)
		return
	}

	s.DB = db
}

func (s *State) CloseDatabase() {
	if s.DB == nil {
		log.Fatalf("Closing nil database")
	}
	s.DB.Close()
	s.DB = nil
}

// Reads our commit from the database.
func (s *State) GetCommit(id string) (Commit, bool) {
	ctx := context.Background()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	defer tx.Commit()

	var commit Commit
	statement, err := tx.PrepareContext(ctx, "select sha1, changeid, changenumber, patchset from buildkite where id = ?")
	if err != nil {
		log.Fatal(err)
	}

	err = statement.QueryRow(id).Scan(&commit.Sha1, &commit.ChangeId, &commit.ChangeNumber, &commit.Patchset)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Fatalf("Failed to query: '%v'", err)
		}
		return commit, false
	} else {
		return commit, true
	}
}

// Writes our commit to the database.
func (s *State) AddCommit(id string, commit Commit) {
	ctx := context.Background()
	// Use a transaction so we do the atomic read, add, write.
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	defer tx.Commit()

	statement, err := tx.PrepareContext(ctx, "insert into buildkite (id, sha1, changeid, changenumber, patchset) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatalf("Failed to insert %s", err)
	}
	_, err = statement.Exec(id, commit.Sha1, commit.ChangeId, commit.ChangeNumber, commit.Patchset)
	if err != nil {
		log.Fatalf("Failed to exec: %s", err)
	}
}

// Simple application to poll Gerrit for events and trigger builds on buildkite when one happens.

// Handles a gerrit event and triggers buildkite accordingly.
func (s *State) handleEvent(eventInfo EventInfo, client *buildkite.Client) {
	// Only work on the desired project.
	if eventInfo.Project != s.Project {
		log.Printf("Ignoring project: '%s'\n", eventInfo.Project)
		return
	}

	// Find the change id, change number, patchset revision
	if eventInfo.Change == nil {
		log.Println("Failed to find Change")
		return
	}

	if eventInfo.PatchSet == nil {
		log.Println("Failed to find Change")
		return
	}

	log.Printf("Got a matching change of %s %s %d,%d\n",
		eventInfo.Change.ID, eventInfo.PatchSet.Revision, eventInfo.Change.Number, eventInfo.PatchSet.Number)

	for {
		var user *User
		if eventInfo.Author != nil {
			user = eventInfo.Author
		} else if eventInfo.Uploader != nil {
			user = eventInfo.Uploader
		} else {
			log.Fatalf("Failed to find Author or Uploader")
		}

		// Triggering a build creates a UUID, and we can see events back from the webhook before the command returns.  Lock across the command so nothing access commits while the new UUID is being added.
		s.mu.Lock()

		// Trigger the build.
		if build, _, err := client.Builds.Create(
			s.BuildkiteOrganization, s.BuildkiteProject, &buildkite.CreateBuild{
				Commit: eventInfo.PatchSet.Revision,
				Branch: eventInfo.Change.ID,
				Author: buildkite.Author{
					Name:  user.Name,
					Email: user.Email,
				},
				Env: map[string]string{
					"GERRIT_CHANGE_NUMBER": fmt.Sprintf("%d", eventInfo.Change.Number),
					"GERRIT_PATCH_NUMBER":  fmt.Sprintf("%d", eventInfo.PatchSet.Number),
				},
			}); err == nil {

			if build.ID != nil {
				log.Printf("Scheduled build %s\n", *build.ID)
				s.AddCommit(*build.ID, Commit{
					Sha1:         eventInfo.PatchSet.Revision,
					ChangeId:     eventInfo.Change.ID,
					ChangeNumber: eventInfo.Change.Number,
					Patchset:     eventInfo.PatchSet.Number,
				})
			}
			s.mu.Unlock()

			if data, err := json.MarshalIndent(build, "", "\t"); err != nil {
				log.Fatalf("json encode failed: %s", err)
			} else {
				log.Printf("%s\n", string(data))
			}

			// Now remove the verified from Gerrit and post the link.
			cmd := exec.Command("ssh",
				"-p",
				"29418",
				"-i",
				s.Key,
				s.User+"@"+s.Server,
				"gerrit",
				"review",
				"-m",
				fmt.Sprintf("\"Build Started: %s\"", *build.WebURL),
				// Don't email out the initial link to lower the spam.
				"-n",
				"NONE",
				"--verified",
				"0",
				fmt.Sprintf("%d,%d", eventInfo.Change.Number, eventInfo.PatchSet.Number))

			log.Printf("Running 'ssh -p 29418 -i %s %s@%s gerrit review -m '\"Build Started: %s\"' -n NONE --verified 0 %d,%d' and waiting for it to finish...",
				s.Key, s.User, s.Server,
				*build.WebURL, eventInfo.Change.Number, eventInfo.PatchSet.Number)
			if err := cmd.Run(); err != nil {
				log.Printf("Command failed with error: %v", err)
			}
			return
		} else {
			s.mu.Unlock()
			log.Fatalf("Failed to trigger build: %s", err)
			log.Printf("Trying again in 30 seconds")
			time.Sleep(30 * time.Second)
		}
	}
}

func (s *State) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	switch r.Method {
	case "POST":
		if r.Header.Get("X-Buildkite-Token") != s.Token {
			http.Error(w, "Invalid token", http.StatusBadRequest)
			return
		}

		var data []byte
		var err error
		if data, err = ioutil.ReadAll(r.Body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Println(string(data))

		var webhook BuildkiteWebhook

		if err := json.Unmarshal(data, &webhook); err != nil {
			log.Fatalf("json decode failed: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// We've successfully received the webhook.  Spawn a goroutine in case the mutex is blocked so we don't block this thread.
		f := func() {
			if webhook.Event == "build.running" {
				if webhook.Build.RebuiltFrom != nil {
					s.mu.Lock()
					if c, ok := s.GetCommit(webhook.Build.RebuiltFrom.ID); ok {
						log.Printf("Detected a rebuild of %s for build %s", webhook.Build.RebuiltFrom.ID, webhook.Build.ID)
						s.AddCommit(webhook.Build.ID, c)

						// And now remove the vote since the rebuild started.
						cmd := exec.Command("ssh",
							"-p",
							"29418",
							"-i",
							s.Key,
							s.User+"@"+s.Server,
							"gerrit",
							"review",
							"-m",
							fmt.Sprintf("\"Build Started: %s\"", webhook.Build.WebURL),
							// Don't email out the initial link to lower the spam.
							"-n",
							"NONE",
							"--verified",
							"0",
							fmt.Sprintf("%d,%d", c.ChangeNumber, c.Patchset))

						log.Printf("Running 'ssh -p 29418 -i %s %s@%s gerrit review -m '\"Build Started: %s\"' -n NONE --verified 0 %d,%d' and waiting for it to finish...",
							s.Key, s.User, s.Server,
							webhook.Build.WebURL, c.ChangeNumber, c.Patchset)
						if err := cmd.Run(); err != nil {
							log.Printf("Command failed with error: %v", err)
						}
					}
					s.mu.Unlock()
				}
			} else if webhook.Event == "build.finished" {
				var commit *Commit
				{
					s.mu.Lock()
					if c, ok := s.GetCommit(webhook.Build.ID); ok {
						commit = &c
					}
					s.mu.Unlock()
				}

				if commit == nil {
					log.Printf("Unknown commit, ID: %s", webhook.Build.ID)
				} else {
					var verify string
					var status string

					if webhook.Build.State == "passed" {
						verify = "+1"
						status = "Succeeded"
					} else {
						verify = "-1"
						status = "Failed"
					}

					cmd := exec.Command("ssh",
						"-p",
						"29418",
						"-i",
						s.Key,
						s.User+"@"+s.Server,
						"gerrit",
						"review",
						"-m",
						fmt.Sprintf("\"Build %s: %s\"", status, webhook.Build.WebURL),
						"--verified",
						verify,
						fmt.Sprintf("%d,%d", commit.ChangeNumber, commit.Patchset))

					log.Printf("Running 'ssh -p 29418 -i %s %s@%s gerrit review -m '\"Build %s: %s\"' --verified %s %d,%d' and waiting for it to finish...",
						s.Key, s.User, s.Server,
						status, webhook.Build.WebURL, verify, commit.ChangeNumber, commit.Patchset)
					if err := cmd.Run(); err != nil {
						log.Printf("Command failed with error: %v", err)
					}

				}
				if webhook.Build.State == "passed" {
					log.Printf("Passed build %s: %s", webhook.Build.ID, webhook.Build.Commit)
				} else {
					log.Printf("Failed build %s: %s", webhook.Build.ID, webhook.Build.Commit)
				}
			}
		}

		go f()

		log.Printf("%s: %s %s %s\n", webhook.Event, webhook.Build.ID, webhook.Build.Commit, webhook.Build.Branch)

		fmt.Fprintf(w, "")

	default:
		internalError := http.StatusInternalServerError
		http.Error(w, "Invalid method", internalError)
		log.Printf("Invalid method %s", r.Method)
	}
}

func main() {
	apiToken := flag.String("token", "", "API token")
	webhookToken := flag.String("webhook_token", "", "Expected webhook token")
	user := flag.String("user", "buildkite", "User to be in gerrit")
	key := flag.String("key", "~/.ssh/buildkite", "SSH key to use to connect to gerrit")
	debug := flag.Bool("debug", false, "Enable debugging")
	server := flag.String("server", "software.frc971.org", "Gerrit server to connect to")
	project := flag.String("project", "971-Robot-Code", "Project to filter events for")
	buildkiteProject := flag.String("buildkite_project", "971-Robot-Code", "Buildkite project to trigger")
	buildkiteOrganization := flag.String("organization", "spartan-robotics", "Project to filter events for")
	database := flag.String("database", "/data/buildkite/buildkite.db", "Database to store builds in.")

	flag.Parse()

	state := State{
		Key:                   *key,
		User:                  *user,
		Token:                 *webhookToken,
		Server:                *server,
		BuildkiteProject:      *buildkiteProject,
		Project:               *project,
		BuildkiteOrganization: *buildkiteOrganization,
	}

	state.OpenDatabase(*database)
	defer state.CloseDatabase()

	f := func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			state.handle(w, r)
		})
		log.Println("Starting webhook server on 10005\n")
		if err := http.ListenAndServe(":10005", nil); err != nil {
			log.Fatal(err)
		}
	}

	if *apiToken == "nope" {
		log.Println("Only starting server")
		f()
	} else {
		go f()
	}

	config, err := buildkite.NewTokenConfig(*apiToken, *debug)

	if err != nil {
		log.Fatalf("client config failed: %s", err)
	}

	client := buildkite.NewClient(config.Client())

	for {
		args := fmt.Sprintf("-o ServerAliveInterval=10 -o ServerAliveCountMax=3 -i %s -p 29418 %s@%s gerrit stream-events", state.Key, state.User, state.Server)
		cmd := exec.Command("ssh", strings.Split(args, " ")...)

		log.Printf("Command: ssh %s\n", args)

		stdout, _ := cmd.StdoutPipe()
		cmd.Start()

		scanner := bufio.NewScanner(stdout)
		maxBufferSize := 1024 * 1024
		scanner.Buffer(make([]byte, maxBufferSize), maxBufferSize)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			m := scanner.Text()

			log.Println(m)

			var eventInfo EventInfo
			dec := json.NewDecoder(bytes.NewReader([]byte(m)))
			dec.DisallowUnknownFields()

			if err := dec.Decode(&eventInfo); err != nil {
				log.Printf("Failed to parse JSON: %e\n", err)
				continue
			}

			log.Printf("Got an event of type: '%s'\n", eventInfo.Type)
			switch eventInfo.Type {
			case "assignee-changed":
			case "change-abandoned":
			case "change-deleted":
			case "change-merged":
			case "change-restored":
			case "comment-added":
				if matched, _ := regexp.MatchString(`(?m)^retest$`, eventInfo.Comment); !matched {
					continue
				}

				state.handleEvent(eventInfo, client)
			case "dropped-output":
			case "hashtags-changed":
			case "project-created":
			case "patchset-created":
				state.handleEvent(eventInfo, client)
			case "ref-updated":
				if eventInfo.RefUpdate.RefName == "refs/heads/master" {
					if build, _, err := client.Builds.Create(
						state.BuildkiteOrganization, state.BuildkiteProject, &buildkite.CreateBuild{
							Commit: eventInfo.RefUpdate.NewRev,
							Branch: "master",
							Author: buildkite.Author{
								Name:  eventInfo.Submitter.Name,
								Email: eventInfo.Submitter.Email,
							},
						}); err == nil {
						log.Printf("Scheduled master build %s\n", *build.ID)
					} else {
						log.Printf("Failed to schedule master build %v", err)
					}
				}
			case "reviewer-added":
			case "reviewer-deleted":
			case "topic-changed":
			case "wip-state-changed":
			case "private-state-changed":
			case "vote-deleted":
			case "ref-replicated":
			case "ref-replication-done":
			case "ref-replication-scheduled":
			default:
				log.Println("Unknown case")
			}
		}
		log.Println("Finished scanning, going to wait")
		cmd.Wait()
	}
}
