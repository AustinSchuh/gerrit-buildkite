# Buildkite Gerrit bridge

Buildkite doesn't support gerrit integration out of the box but provides all
the hooks to implement it pretty easily.

The basic flow is as follow:
 1) gerrit-buildkite uses gerrit stream-events over ssh to watch for events
 2) When an event which should trigger a verification is found, a build is started in Buildkite using the REST API.
 3) The gerrit event and the newly created Buildkite build event are tracked together in a map so the results can be correlated back to the review.
 4) gerrit-buildkite runs a small webserver which listens for the webhooks back from Buildkite
 5) When a response comes back, we look in the map, and if there is an associated review, we publish the results back to gerrit.

If you reply to a review in gerrit with 'retest' on a line, it will re-trigger a verification.
