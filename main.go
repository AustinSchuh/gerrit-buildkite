package main

import (
	"flag"
	"net/http"
	"net/url"

	"github.com/buildkite/go-buildkite/buildkite"
	"github.com/mrmod/gerrit-buildkite/backend"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	flagStreamType       = flag.String("stream-type", "ssh", "Stream type to use")
	flagGerritSshUrl     = flag.String("gerrit-ssh-url", "ssh://gerrit:29418/project", "Gerrit SSH URL")
	flagGerritSshKeyPath = flag.String("gerrit-ssh-key-path", "/path/to/credentials", "File with ssh private key authorized to Gerrit")

	flagBuildkiteOrgSlug                = flag.String("buildkite-org-slug", "org-slug", "Buildkite organization slug")
	flagBuildkitePipelineSlug           = flag.String("buildkite-pipeline-slug", "pipeline-slug", "Buildkite pipeline slug")
	flagBuildkiteApiUrl                 = flag.String("buildkite-api-url", "https://api.buildkite.com/v2", "Buildkite API URL")
	flagBuildkiteApiTokenPath           = flag.String("buildkite-api-token-path", "/path/to/credentials", "File with an API token for Buildkite. Token should have write_builds permission")
	flagBuildkiteWebhookHandlerDisabled = flag.Bool("disable-buildkite-webhook-handler", true, "Disable Buildkite webhook handler when passed")

	flagWebhookHandlerPort = flag.String("webhook-handler-port", "10005", "Port to listen for Buildkite webhook events. Ex: 8080")

	flagLoggingTraceEnabled = flag.Bool("enable-trace-logging", false, "Enable trace logging")
	flagLoggingDebugEnabled = flag.Bool("enable-debug-logging", false, "Enable debug logging")
)

func handleSSHEventStream() {
	// Buffer up to 16 events in the stream
	eventStream := make(chan Event, 16)

	_backend := backend.NewRedisBackend()
	client, err := NewGerritSSHClient(*flagGerritSshUrl, *flagGerritSshKeyPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Gerrit SSH client")
	}

	if !*flagBuildkiteWebhookHandlerDisabled {
		log.Debug().
			Str("webhookHandlerPort", *flagWebhookHandlerPort).
			Msg("Starting Buildkite webhook handler")
		webhookStream := make(chan BuildkiteWebhook, 16)
		webhookHandler, err := NewBuildkiteWebhookHandler(*flagBuildkiteOrgSlug, *flagBuildkitePipelineSlug, *flagBuildkiteApiUrl, *flagBuildkiteApiTokenPath)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create Buildkite webhook handler")
		}
		webhookHandler.HookEvents = webhookStream

		go func() {
			log.Debug().Str("port", *flagWebhookHandlerPort).Msg("Listening for Buildkite webhook events")
			http.ListenAndServe(":"+*flagWebhookHandlerPort, webhookHandler)
		}()

		webhookHandler.Backend = _backend
		log.Info().Msg("Started Webhook event handler")
		go HandleWebhookEvents(webhookStream, client, _backend)
	}

	apiUrl, err := url.Parse(*flagBuildkiteApiUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse Buildkite Api Url")
	}
	apiToken, err := readToken(*flagBuildkiteApiTokenPath)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to read api token: %s", *flagBuildkiteApiTokenPath)
	}

	apiTransport, err := buildkite.NewTokenConfig(apiToken, *flagLoggingTraceEnabled)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Buildkite Api client")
	}

	log.Debug().Str("host", apiUrl.Host).Msgf("Setting API host")

	pipeline := &Pipeline{
		OrgSlug:      *flagBuildkiteOrgSlug,
		PipelineSlug: *flagBuildkitePipelineSlug,
		ApiUrl:       apiUrl,
		ApiClient:    apiTransport.Client(),
	}

	go client.Handle(eventStream, pipeline, _backend)
	log.Info().Msg("Listening for Gerrit events")
	client.Listen(eventStream)
}

func initFlags() {
	flag.Parse()
}
func main() {
	initFlags()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	if *flagLoggingDebugEnabled {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	if *flagLoggingTraceEnabled {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}

	switch *flagStreamType {
	case "ssh":
		handleSSHEventStream()
	default:
		log.Fatal().Str("streamType", *flagStreamType).Msg("Unknown stream type")
	}

}
