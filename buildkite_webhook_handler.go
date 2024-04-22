package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/mrmod/gerrit-buildkite/backend"
	"github.com/rs/zerolog/log"
)

type BuildkiteWebhookHandler struct {
	token      string
	HookEvents chan<- BuildkiteWebhook
	*Pipeline
	Backend backend.Backend
}

func readToken(tokenPath string) (string, error) {
	fh, err := os.Open(tokenPath)
	if err != nil {
		return "", err
	}
	defer fh.Close()
	b, err := io.ReadAll(fh)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(b), "\n"), nil
}

// NewBuildkiteWebhookHandler creates a new Buildkite webhook handler
func NewBuildkiteWebhookHandler(orgSlug, pipelineSlug, apiUrl, tokenPath string) (*BuildkiteWebhookHandler, error) {
	log.Debug().
		Str("orgSlug", orgSlug).
		Str("pipelineSlug", pipelineSlug).
		Str("apiUrl", apiUrl).
		Str("tokenPath", tokenPath).
		Msg("Creating Buildkite webhook handler")
	_apiUrl, err := url.Parse(apiUrl)
	if err != nil {
		return nil, err
	}
	token, err := readToken(tokenPath)
	if err != nil {
		return nil, err
	}
	return &BuildkiteWebhookHandler{
		Pipeline: &Pipeline{
			OrgSlug:      orgSlug,
			PipelineSlug: pipelineSlug,
			ApiUrl:       _apiUrl,
		},
		token: token,
	}, nil
}

func (h *BuildkiteWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debug().Msg("Handling webhook")
	if r.URL.Path != "/" {
		log.Warn().Str("path", r.URL.Path).Msg("Path not found")
		http.Error(w, "Path not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodPost {
		log.Warn().Str("method", r.Method).Msg("Method not allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if hookToken := r.Header.Get("X-Buildkite-Token"); hookToken != h.token {
		log.Warn().Msgf("Unauthorized, invalid or missing token: %s != %s", h.token, hookToken)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	webhook := BuildkiteWebhook{}
	defer r.Body.Close()
	bodyData, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read webhook body")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	log.Trace().Str("body", string(bodyData)).Msg("Webhook body")
	if err := json.NewDecoder(bytes.NewBuffer(bodyData)).Decode(&webhook); err != nil {
		log.Error().Err(err).Msg("Failed to decode webhook")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	h.HookEvents <- webhook
	w.WriteHeader(http.StatusOK)
}

func HandleWebhookEvents(events chan BuildkiteWebhook, r GerritReviewWriter, b backend.Backend) {
	for webhook := range events {
		log.Debug().Str("event", webhook.Event).Msg("Handling webhook event dispatch")
		switch webhook.Event {
		case "build.running":
			log.Info().Str("event", webhook.Event).Msg("Build running")
			ctx := context.TODO()
			pb, err := b.GetBuild(ctx, webhook.Build.Number)
			if err != nil {
				log.Err(err).Msg("Failed to get build")
				return
			}
			if err := r.SetReviewState(&Review{
				Patch:   pb.Patch,
				Message: fmt.Sprintf("Build %d is running", pb.BuildNumber),
				State:   ReviewStateUnverified,
			}); err != nil {
				log.Err(err).
					Str("webhookEvent", webhook.Event).
					Int("buildNumber", pb.BuildNumber).
					Int("patch", pb.Patch.Number).
					Int("change", pb.Patch.Change).
					Int("webhookBuildNumber", webhook.Build.Number).
					Str("webhookBuildState", webhook.Build.State).
					Msg("Failed to set review state")
			}

		case "build.finished":
			log.Info().Str("event", webhook.Event).Msg("Build finished")
			ctx := context.TODO()
			pb, err := b.GetBuild(ctx, webhook.Build.Number)
			if err != nil {
				log.Err(err).Msg("Failed to get build")
				return
			}
			patchMessage := fmt.Sprintf("for Change %d Patch %d", pb.Patch.Change, pb.Patch.Number)

			message := fmt.Sprintf("[Build %d Passed](%s) %s", pb.BuildNumber, webhook.Build.WebURL, patchMessage)
			reviewState := ReviewStateVerified
			if webhook.Build.State == "failed" {
				message = fmt.Sprintf("[Build %d Failed](%s) %s", pb.BuildNumber, webhook.Build.WebURL, patchMessage)
				reviewState = ReviewStateRejected
			}
			if err := r.SetReviewState(&Review{
				Patch:   pb.Patch,
				Message: message,
				State:   reviewState,
			}); err != nil {
				log.Err(err).
					Str("webhookEvent", webhook.Event).
					Int("buildNumber", pb.BuildNumber).
					Int("patch", pb.Patch.Number).
					Int("change", pb.Patch.Change).
					Int("webhookBuildNumber", webhook.Build.Number).
					Str("webhookBuildState", webhook.Build.State).
					Msg("Failed to set review state")
			}
			// TODO: Notify Gerrit of build finished
		case "build.scheduled":
			log.Info().Str("event", webhook.Event).Msg("Build scheduled")
		case "build.cancelled":
			log.Info().Str("event", webhook.Event).Msg("Build cancelled")
			ctx := context.TODO()
			pb, err := b.GetBuild(ctx, webhook.Build.Number)
			if err != nil {
				log.Err(err).Msg("Failed to get build")
				return
			}
			log.Info().
				Int("change", pb.Patch.Change).
				Int("patch", pb.Patch.Number).
				Int("buildNumber", pb.BuildNumber).
				Msg("Cancelled build")

		default:
			log.Warn().Str("event", webhook.Event).Msg("Unknown event")
		}
	}
}
