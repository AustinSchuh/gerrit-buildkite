package backend

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

type Backend interface {
	// SaveBuild saves a build and patch information to the backend
	SaveBuild(context.Context, *PatchBuild) error
	// GetBuild retrieves a build and patch information from the backend
	GetBuild(ctx context.Context, buildNumber int) (*PatchBuild, error)
	// GetPatch retrieves a build by patch and change number from the backend
	GetPatch(context.Context, *Patch) (*PatchBuild, error)
}

// Patch represents a Gerrit patch revision
// Patches have one change
// Changes have many patches
type Patch struct {
	// Number of the patch in a Change
	Number int
	// Change is the change number in Gerrit
	Change int
	// Revision is the SHA of the change in git
	Revision string
}

// PatchBuild represents a Gerrit patch revision with a build number from BuildKite
type PatchBuild struct {
	BuildNumber int
	*Patch
}

// NewPatch creates a new PatchBuild from a patch revision slug
// it does not include a build number
func NewPatch(slug string) (*Patch, error) {
	patchBuild := &Patch{}
	c := strings.Split(slug, "_")
	patchNumber, err := strconv.Atoi(c[0])
	if err != nil {
		log.Error().Err(err).Str("slug", slug).Msg("Failed to parse change_patch slug")
		return patchBuild, err
	}
	changeNumber, err := strconv.Atoi(c[1])
	if err != nil {
		log.Error().Err(err).Str("slug", slug).Msg("Failed to parse change_path slug")
		return patchBuild, err
	}
	patchBuild.Number = patchNumber
	patchBuild.Change = changeNumber
	return patchBuild, nil
}

// PatchSlug returns the patch revision slug from a $Patch_$Change slug
func (pb *Patch) PatchSlug() string {
	return fmt.Sprintf("%d_%d", pb.Number, pb.Change)
}
