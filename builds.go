package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/buildkite/go-buildkite/buildkite"
	"github.com/mrmod/gerrit-buildkite/backend"
	"github.com/rs/zerolog/log"
)

// Pipeline represents a Buildkite pipeline
type Pipeline struct {
	OrgSlug, PipelineSlug string
	ApiUrl, GraphQLApiUrl *url.URL
	ApiClient             *http.Client
}

// CreateBuild creates a build on a pipeline for a Review
func (p *Pipeline) CreateBuild(data *buildkite.CreateBuild) (int, error) {
	build, response, err := p.createBuild(p.ApiClient, data)
	if err != nil {
		return 0, err
	}
	log.Debug().
		Str("status", response.Status).
		Int("statusCode", response.StatusCode).
		Msgf("Created Buildkite build")
	if response.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("failed to create build: %d", response.StatusCode)
	}
	log.Trace().Any("build", build).Msg("Build created")
	return *build.Number, nil
}

func (p *Pipeline) createBuild(c *http.Client, request *buildkite.CreateBuild) (*buildkite.Build, *buildkite.Response, error) {
	bk := buildkite.NewClient(c)
	bk.BaseURL = p.ApiUrl
	return bk.Builds.Create(p.OrgSlug, p.PipelineSlug, request)
}

func (p *Pipeline) CancelBuild(buildNumber int) error {
	err := p.cancelBuild(p.ApiClient, &backend.PatchBuild{BuildNumber: buildNumber})
	if err != nil {
		return err
	}
	log.Debug().Int("buildNumber", buildNumber).Msg("Build cancelled")
	return nil
}

// CancelBuild cancels a build on a pipeline
func (p *Pipeline) cancelBuild(c *http.Client, pb *backend.PatchBuild) error {

	req, err := http.NewRequest(
		http.MethodPut,
		p.ApiUrl.JoinPath([]string{
			"organizations",
			p.OrgSlug,
			"pipelines",
			p.PipelineSlug,
			"builds",
			fmt.Sprintf("%d", pb.BuildNumber),
		}...).String(),
		nil,
	)
	if err != nil {
		return err
	}
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		return nil
	}
	data := map[string]string{}
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		log.Err(err).Msg("Failed to decode CancelBuild response body")
		return err
	}

	log.Warn().
		Str("message", data["message"]).
		Str("orgSlug", p.OrgSlug).
		Str("pipelineSlug", p.PipelineSlug).
		Int("buildNumber", pb.BuildNumber).
		Int("statusCode", res.StatusCode).
		Msg("Failed to cancel build")
	return fmt.Errorf("failed to cancel build %d: %d", pb.BuildNumber, res.StatusCode)

	return nil
}
