// Package k6provider implements a library for providing custom k6 binaries
// using a k6build service
package k6provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	k6Module             = "k6"
	defaultPruneInterval = time.Hour

	// K6ModPath is the Go module path for k6 v1
	K6ModPath = "go.k6.io/k6"
	// K6ModPathV2 is the Go module path for k6 v2
	K6ModPathV2 = "go.k6.io/k6/v2"
)

var (
	// ErrBinary indicates an error creating local binary
	ErrBinary = errors.New("creating binary")
	// ErrBuild indicates an error building binary
	ErrBuild = errors.New("building binary")
	// ErrConfig is produced by invalid configuration
	ErrConfig = errors.New("invalid configuration")
	// ErrDownload indicates an error downloading binary
	ErrDownload = errors.New("downloading binary")
	// ErrInvalidParameters is produced by invalid build parameters
	ErrInvalidParameters = errors.New("invalid build parameters")
	// ErrPruningCache indicates an error pruning the binary cache
	ErrPruningCache = errors.New("pruning cache")
)

// K6Binary defines the attributes of a k6 binary
type K6Binary struct {
	// Path to the binary
	Path string
	// Dependencies as a map of name: version
	// e.g. {"k6": "v0.50.0", "k6/x/kubernetes": "v0.9.0"}
	Dependencies map[string]string
	// Checksum of the binary
	Checksum string
	// Indicates if the artifact is retrieved from cache
	Cached bool
	// Source of the artifact (if not cached)
	DownloadURL string
}

// UnmarshalDeps returns the dependencies as a list of name:version pairs separated by ";"
func (b K6Binary) UnmarshalDeps() string {
	buffer := &bytes.Buffer{}
	for dep, version := range b.Dependencies {
		fmt.Fprintf(buffer, "%s:%q;", dep, version)
	}
	return buffer.String()
}

// Config defines the configuration of the Provider.
type Config struct {
	// Platform for the binaries. Defaults to the current platform
	Platform string
	// BuildServiceURL URL of the k6 build service
	// If not specified the value from K6_BUILD_SERVICE_URL environment variable is used
	BuildServiceURL string
	// BuildServiceAuthType type of passed in the header "Authorization: <type> <auth>".
	// Can be used to set the type as "Basic", "Token" or any custom type. Default to "Bearer"
	BuildServiceAuthType string
	// BuildServiceAuth contain authorization credentials for BuildService requests
	// Passed in the "Authorization <type> <credentials" (see BuildServiceAuthType for the meaning of <type>)
	// If not specified the value of K6_BUILD_SERVICE_AUTH is used.
	// If no value is defined, the Authentication header is not passed (except is passed as a custom header
	// see BuildServiceHeaders)
	BuildServiceAuth string
	// BuildServiceHeaders HTTP headers for the k6 build service
	BuildServiceHeaders map[string]string
	// BinDir deprecated use BinaryCacheDir
	BinDir string
	// BinaryCacheDir path to binary cache directory. If not set the environment variable K6_BINARY_CACHE is used.
	// If not set, the OS-specific cache directory is used. If not set, a temporary directory is used.
	BinaryCacheDir string
	// HighWaterMark deprecated use BinaryCacheSize
	HighWaterMark int64
	// BinaryCacheSize is the upper limit of cache size to trigger a prune of least recently used binary.
	// If 0 (default) is specified, the cache is not pruned.
	// If not set K6_BINARY_CACHE_SIZE environment variable is used. This variable defines the size in bytes,
	// kilobytes (Kb), megabytes (Mb) or gigabytes (Gb). Ex: "100Kb", "1Gb"
	BinaryCacheSize int64
	// PruneInterval minimum time between prune attempts. Defaults to 1h
	PruneInterval time.Duration
	// Download configuration
	DownloadConfig DownloadConfig
	// K6ModPath is the Go module path for k6. Defaults to K6ModPath (go.k6.io/k6).
	// Set to K6ModPathV2 (go.k6.io/k6/v2) for k6 v2 builds.
	K6ModPath string
}

// Dependencies defines a group of dependencies with their version constrains
// For example, {"k6": "*", "k6/x/sql": ">v0.4.0"}
type Dependencies map[string]string

// Provider implements an interface for providing custom k6 binaries
// from a [k6build] service.
//
// [k6build]: https://github.com/grafana/k6build
type Provider struct {
	client     *http.Client
	downloader *downloader
	binDir     string
	buildSrv   *buildClient
	platform   string
	k6ModPath  string
	pruner     *Pruner
	logger     *slog.Logger
}

// NewDefaultProvider returns a Provider with default settings
//
// Expects the K6_BUILD_SERVICE_URL environment variable to be set
// with the URL to the k6build service
func NewDefaultProvider() (*Provider, error) {
	return NewProvider(Config{})
}

// NewProvider returns a [Provider] with the given Config
func NewProvider(config Config) (*Provider, error) {
	return NewProviderWithLogger(config, nil)
}

// NewProviderWithLogger returns a [Provider] with the given Config and logger.
// If logger is nil, a discard logger is used.
func NewProviderWithLogger(config Config, logger *slog.Logger) (*Provider, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	var err error

	// try first deprecated BinDir
	binDir := config.BinDir
	if binDir == "" {
		binDir = config.BinaryCacheDir
	}
	if binDir == "" {
		binDir = os.Getenv("K6_BINARY_CACHE") //nolint:forbidigo
	}
	if binDir == "" {
		cacheDir, err := os.UserCacheDir() //nolint:forbidigo
		if err != nil {
			cacheDir = os.TempDir() //nolint:forbidigo
		}
		binDir = filepath.Join(cacheDir, "k6provider")
	}

	// try first the deprecated HighWaterMark
	cacheSize := config.HighWaterMark
	if cacheSize == 0 {
		cacheSize = config.BinaryCacheSize
	}
	if cacheSize == 0 {
		cacheSize, err = parseSize(os.Getenv("K6_BINARY_CACHE_SIZE")) //nolint:forbidigo
		if err != nil {
			return nil, NewWrappedError(ErrConfig, err)
		}
	}

	httpClient := http.DefaultClient

	buildSrvURL := config.BuildServiceURL
	if buildSrvURL == "" {
		buildSrvURL = os.Getenv("K6_BUILD_SERVICE_URL") //nolint:forbidigo
	}
	if buildSrvURL == "" {
		return nil, NewWrappedError(ErrConfig, fmt.Errorf("build service URL is required"))
	}

	buildSrvAuth := config.BuildServiceAuth
	if buildSrvAuth == "" {
		buildSrvAuth = os.Getenv("K6_BUILD_SERVICE_AUTH") //nolint:forbidigo
	}

	buildSrv, err := newBuildServiceClient(
		buildSrvURL, buildSrvAuth, config.BuildServiceAuthType, config.BuildServiceHeaders)
	if err != nil {
		return nil, NewWrappedError(ErrConfig, err)
	}

	platform := config.Platform
	if platform == "" {
		platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	}

	pruneInterval := config.PruneInterval
	if cacheSize > 0 && pruneInterval == 0 {
		pruneInterval = defaultPruneInterval
	}

	downloader, err := newDownloader(config.DownloadConfig, logger)
	if err != nil {
		return nil, NewWrappedError(ErrConfig, err)
	}

	pruner := NewPrunerWithLogger(binDir, cacheSize, pruneInterval, logger)

	k6ModPath := config.K6ModPath
	if k6ModPath == "" {
		k6ModPath = K6ModPath
	}

	return &Provider{
		client:     httpClient,
		downloader: downloader,
		binDir:     binDir,
		buildSrv:   buildSrv,
		platform:   platform,
		k6ModPath:  k6ModPath,
		pruner:     pruner,
		logger:     logger,
	}, nil
}

// Artifact defines the artifact returned by the build service
type Artifact struct {
	// Unique id. Binaries satisfying the same set of dependencies have the same ID
	ID string
	// URL to fetch the artifact's binary
	URL string
	// List of dependencies that the artifact provides
	Dependencies map[string]string
	// platform
	Platform string
	// binary checksum (sha256)
	Checksum string
}

// GetArtifact returns a custom k6 artifact that satisfies the given a set of dependencies.
// from the configured build service.
// it's useful if you want to get the artifact without downloading the binary.
func (p *Provider) GetArtifact(
	ctx context.Context,
	deps Dependencies,
) (Artifact, error) {
	k6Constrains, buildDeps := buildDeps(deps)

	p.logger.Debug("Resolving k6 artifact",
		"deps", deps,
		"platform", p.platform,
	)
	artifact, err := p.buildSrv.Build(ctx, p.platform, p.k6ModPath, k6Constrains, buildDeps)
	if err != nil {
		if !errors.Is(err, ErrInvalidParameters) {
			return Artifact{}, NewWrappedError(ErrBuild, err)
		}

		// it is an invalid build parameters, we are interested in the
		// root cause
		cause := errors.Unwrap(err)
		for errors.Unwrap(cause) != nil {
			cause = errors.Unwrap(cause)
		}
		return Artifact{}, NewWrappedError(ErrInvalidParameters, cause)
	}

	// the checksum can be base64 encoded or not depending on the source
	// if the length is not 64 we assume it is encoded and we try to decode it
	// see https://github.com/grafana/k6build/issues/140
	checksum := artifact.Checksum
	if len(checksum) < 64 {
		var decoded []byte
		decoded, err = base64.StdEncoding.DecodeString(checksum)
		if err != nil {
			return Artifact{}, NewWrappedError(ErrBuild, fmt.Errorf("invalid checksum: %w", err))
		}
		checksum = fmt.Sprintf("%x", decoded)
	}

	resolved := Artifact{
		ID:           artifact.ID,
		URL:          artifact.URL,
		Dependencies: artifact.Dependencies,
		Platform:     artifact.Platform,
		Checksum:     checksum,
	}
	p.logger.Debug("Artifact resolved",
		"artifact_id", resolved.ID,
		"deps", resolved.Dependencies,
		"url", resolved.URL,
	)
	return resolved, nil
}

// GetBinary returns a custom k6 binary that satisfies the given a set of dependencies.
//
// If the k6 version constrains are not specified, "*" is used as default.
//
// If the binary for the given dependencies does not exist, it will be built
// using the configured build service and stored in the cache directory.
//
// If the binary exists, it will be returned from the cache.
//
// The returned K6Binary has the path to the custom k6 binary, the list of
// dependencies and the checksum of the binary.
//
// If any error occurs while building, downloading or checking the binary,
// an [WrappedError] will be returned. This error will be one of the errors
// defined in the k6provider packaged. Using errors.Unwrap will return its cause.
func (p *Provider) GetBinary(ctx context.Context, constrains Dependencies) (K6Binary, error) {
	artifact, err := p.GetArtifact(ctx, constrains)
	if err != nil {
		return K6Binary{}, err
	}

	// ensure the binary's directory always exists
	// this is slightly inefficient but simplifies logic
	artifactDir := filepath.Join(p.binDir, artifact.ID)
	err = os.MkdirAll(artifactDir, 0o700) //nolint:forbidigo
	if err != nil {
		return K6Binary{}, NewWrappedError(ErrBinary, err)
	}

	// lock the directory to prevent concurrent access while downloading
	lock := newDirLock(artifactDir)
	err = lock.lock(0)
	if err != nil {
		return K6Binary{}, NewWrappedError(ErrBinary, err)
	}
	defer lock.unlock() //nolint:errcheck

	binPath := filepath.Join(artifactDir, k6Binary)
	_, err = os.Stat(binPath) //nolint:forbidigo

	// binary already exists and is valid
	if err == nil {
		p.logger.Info("Using cached k6 binary",
			"path", binPath,
			"artifact_id", artifact.ID,
			"deps", artifact.Dependencies,
		)
		go p.pruner.Touch(binPath)

		return K6Binary{
			Path:         binPath,
			Dependencies: artifact.Dependencies,
			Checksum:     artifact.Checksum,
			Cached:       true,
		}, nil
	}

	// if there's other error)
	if !os.IsNotExist(err) { //nolint:forbidigo
		return K6Binary{}, NewWrappedError(ErrBinary, err)
	}

	p.logger.Info("Downloading custom k6 binary",
		"artifact_id", artifact.ID,
		"deps", artifact.Dependencies,
	)
	p.logger.Debug("Downloading custom k6 binary",
		"url", artifact.URL,
	)
	err = p.downloader.download(ctx, artifact.URL, binPath, artifact.Checksum)
	if err != nil {
		_ = os.RemoveAll(artifactDir) //nolint:forbidigo
		return K6Binary{}, NewWrappedError(ErrDownload, err)
	}

	p.logger.Info("Custom k6 binary ready",
		"path", binPath,
		"artifact_id", artifact.ID,
		"deps", artifact.Dependencies,
	)

	// start pruning in background
	// TODO: handle case the calling process is cancelled
	go p.pruner.Prune() //nolint:errcheck

	return K6Binary{
		Path:         binPath,
		Dependencies: artifact.Dependencies,
		Checksum:     artifact.Checksum,
		Cached:       false,
		DownloadURL:  artifact.URL,
	}, nil
}

func buildDeps(deps Dependencies) (string, []dependency) {
	bdeps := make([]dependency, 0, len(deps))
	k6constraint := "*"

	for dep, constraints := range deps {
		if dep == k6Module {
			k6constraint = constraints
			continue
		}

		bdeps = append(bdeps, dependency{
			Name:        dep,
			Constraints: constraints,
		})
	}

	return k6constraint, bdeps
}
