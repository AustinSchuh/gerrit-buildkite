package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/mrmod/gerrit-buildkite/backend"
	"github.com/rs/zerolog/log"
)

const (
	ReviewStateVerified   = 1
	ReviewStateUnverified = 0
	ReviewStateRejected   = -1
)

var (
	sshConnectionOptions = []string{
		"-o ServerAliveInterval=10",
		"-o ServerAliveCountMax=3",
	}
)

type GerritEventStreamHandler interface {
	GerritEventHandler
	GerritEventListener
	GerritReviewWriter
}

// GerritSSHClient represents a Gerrit server
type GerritSSHClient struct {
	*url.URL
	SshKeyPath string
}

// GerritEventListener is an interface for listening to Gerrit events
type GerritEventListener interface {
	Listen(chan<- Event)
}

// GerritEventHandler is an interface for handling Gerrit events
type GerritEventHandler interface {
	Handle(chan Event, BuildPipeline, backend.Backend)
}

// GerritReviewWriter is an interface for writing reviews to Gerrit
type GerritReviewWriter interface {
	SetReviewState(*Review) error
}

// Review represents a Gerrit review
type Review struct {
	*backend.Patch
	State              int
	Message            string
	NotifyEmailAddress string
}

func NewGerritSSHClient(sshUrl string, sshKeyPath string) (*GerritSSHClient, error) {
	u, err := url.Parse(sshUrl)
	if err != nil {
		return nil, err
	}
	return &GerritSSHClient{u, sshKeyPath}, nil
}

// Build the command arguemtns for an ssh connection to Gerrit
// Ex: ssh user@gerrit -p port gerrit
// Any new tail argument is a gerrit command then the arguments
// to that command
func (s *GerritSSHClient) buildSshCommand() []string {
	return []string{
		"-i",
		s.SshKeyPath,
		"-p",
		s.Port(),
		s.User.Username() + "@" + s.Hostname(),
		"gerrit",
	}
}

// SetReviewState sets the verified state of a review in Gerrit
func (s *GerritSSHClient) SetReviewState(r *Review) error {

	args := s.buildSshCommand()
	reviewArgs := []string{
		"review",
		"-m", fmt.Sprintf(`'%s'`, r.Message),
		"-n", "NONE",
		"--code-review", fmt.Sprint(r.State),
		fmt.Sprintf("%d,%d", r.Patch.Change, r.Patch.Number),
	}
	log.Debug().
		Str("patchNumber", fmt.Sprint(r.Patch.Number)).
		Int("change", r.Patch.Change).
		Int("state", r.State).
		Str("_args", strings.Join(append(args, reviewArgs...), " ")).
		Msgf("Setting review state: %d", r.State)

	return exec.Command("ssh", append(args, reviewArgs...)...).Run()
}

// GetListener returns a new unopened SSH connection to Gerrit.
func (s *GerritSSHClient) getListener() *exec.Cmd {
	log.Debug().Msgf("Creating stream connection to Gerrit at %s", s.String())
	connectionArgs := append(s.buildSshCommand(), "stream-events")

	log.Debug().
		Str("sshCommand", strings.Join(append(sshConnectionOptions, connectionArgs...), " ")).
		Msgf("Authenticating to event stream with key %s", s.SshKeyPath)
	return exec.Command("ssh", append(sshConnectionOptions, connectionArgs...)...)
}

// Listen listens for events on the Gerrit ssh event stream
func (s *GerritSSHClient) Listen(events chan<- Event) {
	listener := s.getListener()
	eventStream, err := listener.StdoutPipe()
	if err != nil {
		log.Error().Err(err).Msg("Failed to open SSH connection to Gerrit")
		return
	}
	log.Debug().Msg("Starting SSH connection to Gerrit")

	for {

		if err := listener.Start(); err != nil {
			log.Error().Err(err).Msg("Failed to start SSH connection to Gerrit")
			return
		}

		scanner := bufio.NewScanner(eventStream)
		maxBufferSize := 1024 * 1024
		scanner.Buffer(make([]byte, maxBufferSize), maxBufferSize)
		scanner.Split(bufio.ScanLines)

		for scanner.Scan() {
			text := scanner.Text()
			log.Trace().Str("event", text).Msg("Raw Event from SSH connection")
			decoder := json.NewDecoder(bytes.NewBufferString(scanner.Text()))
			event := Event{}
			if err := decoder.Decode(&event); err != nil {
				log.Error().Err(err).Msg("Failed to decode Gerrit event")
				return
			}
			log.Debug().Str("eventType", event.Type).Msgf("Dispatching received event %s", event.Type)
			events <- event
		}
		log.Debug().Msg("Closing SSH connection to Gerrit")

		if err := listener.Wait(); err != nil {
			log.Err(err).Msg("Failed to wait for SSH connection to Gerrit")
			return
		}
	}

}

// Handle dispatches events to the appropriate handler
func (s *GerritSSHClient) Handle(events chan Event, p BuildPipeline, b backend.Backend) {
	for event := range events {
		if handler, ok := eventRouter[event.Type]; ok {
			log.Trace().Any("event", event).Msg("Raw Event from Dispatch")
			log.Debug().Str("eventType", event.Type).Msgf("Handling dispatched event %s", event.Type)
			handler(event, p, b)
			continue
		}
		log.Info().Str("eventType", event.Type).Msgf("No handler for event %s", event.Type)
	}
}
