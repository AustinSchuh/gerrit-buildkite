package main

import (
	"context"
	"regexp"

	"github.com/buildkite/go-buildkite/buildkite"
	"github.com/mrmod/gerrit-buildkite/backend"
	"github.com/rs/zerolog/log"
)

var (
	eventRouter = map[string]EventHandlerFunc{
		"patchset-created": HandlePatchsetCreated,
		"ref-updated":      HandleRefUpdated,
		"comment-added":    HandleCommentAdded,
	}
)

type BuildPipeline interface {
	CreateBuild(*buildkite.CreateBuild) (buildNumber int, err error)
	CancelBuild(buildNumber int) error
}

type EventHandlerFunc func(Event, BuildPipeline, backend.Backend) error

func HandleCommentAdded(event Event, p BuildPipeline, b backend.Backend) error {
	comment := event.Comment

	if ok, _ := regexp.MatchString(`(?mi)^retest$`, comment); ok {
		log.Info().
			Str("eventType", event.Type).
			Int("patchNumber", event.PatchSet.Number).
			Int("change", event.Change.Number).
			Msg("Retesting patchset")

	}
	return nil
}

func HandlePatchsetCreated(event Event, p BuildPipeline, b backend.Backend) error {
	defer func() {
		if err := recover(); err != nil {
			log.Error().
				Str("handler", "HandlePatchsetCreated").
				Msg("Panic recovered")
		}
	}()
	log.Trace().Any("event", event).Msg("Handling patchset created")
	log.Debug().
		Str("eventType", event.Type).
		Int("patchNumber", event.PatchSet.Number).
		Int("change", event.Change.Number).
		Str("authorName", event.PatchSet.Author.Name).
		Str("authorEmail", event.PatchSet.Author.Email).
		Msg("Patchset created or updated")

	patch := &backend.Patch{
		Number:   event.PatchSet.Number,
		Change:   event.Change.Number,
		Revision: event.PatchSet.Revision,
	}
	log.Debug().
		Str("eventType", event.Type).
		Str("patchRevision", patch.Revision).
		Int("change", patch.Change).
		Int("patchNumber", patch.Number).
		Msg("Creating build")
	buildNumber, err := p.CreateBuild(&buildkite.CreateBuild{
		Commit: event.PatchSet.Revision,
		Branch: event.Change.ID,
		Author: buildkite.Author{
			Name:  event.PatchSet.Author.Name,
			Email: event.PatchSet.Author.Email,
		},
	})
	if err != nil {
		log.Error().Err(err).
			Str("eventType", event.Type).
			Int("patchNumber", event.PatchSet.Number).
			Int("change", event.Change.Number).
			Msg("Failed to create build")
		return err
	}
	pb := &backend.PatchBuild{
		BuildNumber: buildNumber,
		Patch:       patch,
	}
	ctx := context.TODO()
	log.Debug().
		Str("eventType", event.Type).
		Int("patchNumber", event.PatchSet.Number).
		Int("change", event.Change.Number).
		Int("buildNumber", buildNumber).
		Msg("Saving patch build information")
	return b.SaveBuild(ctx, pb)
}

func HandleRefUpdated(event Event, p BuildPipeline, b backend.Backend) error {
	defer func() {
		if err := recover(); err != nil {
			log.Error().
				Str("handler", "HandleRefUpdated").
				Msg("Panic recovered")
		}
	}()
	log.Debug().
		Str("eventType", event.Type).
		Int("patchNumber", event.PatchSet.Number).
		Int("change", event.Change.Number).
		Str("refName", event.RefUpdate.RefName).Msg("Ref updated")
	if ok, _ := regexp.MatchString(`^refs/heads/(master|main)`, event.RefUpdate.RefName); ok {
		log.Info().
			Str("eventType", event.Type).
			Int("patchNumber", event.PatchSet.Number).
			Int("change", event.Change.Number).
			Msg("Trunk Updated")
	}
	return nil
}
