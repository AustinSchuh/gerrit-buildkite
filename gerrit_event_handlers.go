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
	commentCommands = map[*regexp.Regexp]commandFunc{
		regexp.MustCompile(`(?mi)^retest$`): handleRetestComment,
	}
)

type BuildPipeline interface {
	CreateBuild(*buildkite.CreateBuild) (buildNumber int, err error)
	CancelBuild(buildNumber int) error
}
type commandFunc func(event Event, p BuildPipeline, b backend.Backend) error

type EventHandlerFunc func(Event, BuildPipeline, backend.Backend) error

func createAndSaveBuild(p BuildPipeline, b backend.Backend, event Event, build *buildkite.CreateBuild) error {
	buildNumber, err := p.CreateBuild(build)
	if err != nil {
		log.Error().Err(err).
			Str("eventType", event.Type).
			Int("patch", event.PatchSet.Number).
			Int("change", event.Change.Number).
			Msg("Failed to create build")
		return err
	}
	pb := &backend.PatchBuild{
		BuildNumber: buildNumber,
		Patch: &backend.Patch{
			Number: event.PatchSet.Number,
			Change: event.Change.Number,
		},
	}
	ctx := context.TODO()
	log.Debug().
		Str("eventType", event.Type).
		Int("patch", event.PatchSet.Number).
		Int("change", event.Change.Number).
		Int("buildNumber", buildNumber).
		Msg("Saving patch build information")
	return b.SaveBuild(ctx, pb)
}

func handleRetestComment(event Event, p BuildPipeline, b backend.Backend) error {
	log.Info().
		Str("eventType", event.Type).
		Int("patch", event.PatchSet.Number).
		Int("change", event.Change.Number).
		Msg("Retesting patchset")
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
	build := &buildkite.CreateBuild{
		Commit: event.PatchSet.Revision,
		Branch: event.Change.ID,
		Author: buildkite.Author{
			Name:  event.PatchSet.Author.Name,
			Email: event.PatchSet.Author.Email,
		},
	}
	return createAndSaveBuild(p, b, event, build)
}

func HandleCommentAdded(event Event, p BuildPipeline, b backend.Backend) error {
	comment := event.Comment

	for expression, cmdFunc := range commentCommands {
		log.Debug().Str("comment", comment).Msg("Checking comment for command")
		if expression.MatchString(comment) {
			return cmdFunc(event, p, b)
		}
	}
	log.Debug().Msg("No command found in comment")
	return nil
}

func HandlePatchsetCreated(event Event, p BuildPipeline, b backend.Backend) error {
	defer func() {
		if err := recover(); err != nil {
			log.Error().
				Any("panic", err).
				Str("handler", "HandlePatchsetCreated").
				Msg("Panic recovered")
		}
	}()
	log.Trace().Any("event", event).Msg("Handling patchset created")
	log.Debug().
		Str("eventType", event.Type).
		Int("patch", event.PatchSet.Number).
		Int("change", event.Change.Number).
		Str("authorName", event.PatchSet.Author.Name).
		Str("authorEmail", event.PatchSet.Author.Email).
		Msg("Patchset created or updated")

	patch := &backend.Patch{
		Number:   event.PatchSet.Number,
		Change:   event.Change.Number,
		Revision: event.PatchSet.Revision,
	}

	// Cancel previous build if it exists
	if patch.Number > 1 {
		prevPatch := &backend.Patch{
			Number: patch.Number - 1,
			Change: patch.Change,
		}
		if pb, err := b.GetPatch(context.TODO(), prevPatch); err == nil {
			log.Debug().
				Str("eventType", event.Type).
				Str("eventType", event.Type).
				Int("patch", patch.Number).
				Int("change", patch.Change).
				Int("prevPatch", prevPatch.Number).
				Msg("Cancelling previous build")
			if err := p.CancelBuild(pb.BuildNumber); err != nil {
				log.Error().
					Err(err).
					Str("eventType", event.Type).
					Int("patch", patch.Number).
					Int("change", patch.Change).
					Int("prevPatch", prevPatch.Number).
					Msg("Failed to cancel build")
			}
		}
	}
	log.Debug().
		Str("eventType", event.Type).
		Str("patchRevision", patch.Revision).
		Int("change", patch.Change).
		Int("patchNumber", patch.Number).
		Msg("Creating build")
	build := &buildkite.CreateBuild{
		Commit: event.PatchSet.Revision,
		Branch: event.Change.ID,
		Author: buildkite.Author{
			Name:  event.PatchSet.Author.Name,
			Email: event.PatchSet.Author.Email,
		},
	}
	return createAndSaveBuild(p, b, event, build)
}

func HandleRefUpdated(event Event, p BuildPipeline, b backend.Backend) error {
	defer func() {
		if err := recover(); err != nil {
			log.Error().
				Any("panic", err).
				Str("handler", "HandleRefUpdated").
				Msg("Panic recovered")
		}
	}()
	log.Debug().
		Str("eventType", event.Type).
		Int("patch", event.PatchSet.Number).
		Int("change", event.Change.Number).
		Str("refName", event.RefUpdate.RefName).Msg("Ref updated")
	if ok, _ := regexp.MatchString(`^refs/heads/(master|main)`, event.RefUpdate.RefName); ok {
		log.Info().
			Str("eventType", event.Type).
			Int("patch", event.PatchSet.Number).
			Int("change", event.Change.Number).
			Msg("Trunk Updated")
	}
	return nil
}
