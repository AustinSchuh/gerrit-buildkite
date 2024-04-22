package backend

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type RedisBackend struct {
	*redis.Client
}

const (
	RedisNeverExpireTTL = 0
)

var (
	ErrBuildNotFound = fmt.Errorf("build not found")
	envRedisAddress  = "localhost:6379"
	envRedisPassword = ""
	envRedisDB       = "0"
)

func NewRedisBackend() *RedisBackend {
	redisDB, err := strconv.Atoi(envRedisDB)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse REDIS_DB")
	}
	return &RedisBackend{
		redis.NewClient(&redis.Options{
			Addr:     envRedisAddress,
			Password: envRedisPassword,
			DB:       redisDB,
		}),
	}
}

// SaveBuild saves a build and patch information to the backend
func (b *RedisBackend) SaveBuild(ctx context.Context, pb *PatchBuild) error {
	log.Debug().
		Int("patchNumber", pb.Number).
		Int("change", pb.Change).
		Int("buildNumber", pb.BuildNumber).
		Str("patchSlug", pb.PatchSlug()).
		Msg("Saving build patchChange and build number to redis")
	// SET patchNumber_patchChange buildNumber
	key := fmt.Sprintf("patchChange:%s", pb.PatchSlug())
	if err := b.Set(ctx, key, pb.BuildNumber, RedisNeverExpireTTL).Err(); err != nil {
		return err
	}
	key = fmt.Sprintf("buildNumber:%d", pb.BuildNumber)
	if err := b.Set(ctx, key, pb.PatchSlug(), RedisNeverExpireTTL).Err(); err != nil {
		return err
	}
	return nil
}

// GetBuild retrieves a build and patch information from the backend
func (b *RedisBackend) GetBuild(ctx context.Context, buildNumber int) (*PatchBuild, error) {
	// GET build:BuildNumber
	key := fmt.Sprintf("buildNumber:%d", buildNumber)
	patchChangeSlug, err := b.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, ErrBuildNotFound
	}
	patch, err := NewPatch(patchChangeSlug)
	if err != nil {
		return nil, err
	}

	return &PatchBuild{
		BuildNumber: buildNumber,
		Patch:       patch,
	}, nil
}

// GetPatch retrieves a build by patch and change number from the backend
func (b *RedisBackend) GetPatch(ctx context.Context, p *Patch) (*PatchBuild, error) {
	key := fmt.Sprintf("patchChange:%s", p.PatchSlug())
	buildNumber, err := b.Get(ctx, key).Int()
	if err == redis.Nil {
		return nil, ErrBuildNotFound
	}
	if err != nil {
		return nil, err
	}
	return &PatchBuild{
		BuildNumber: buildNumber,
		Patch:       p,
	}, nil
}
