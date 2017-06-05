package image

import (
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/dnephin/dobi/config"
	"github.com/dnephin/dobi/tasks/context"
	"github.com/dnephin/dobi/utils/fs"
	"github.com/docker/cli/cli/command/image/build"
	"github.com/docker/docker/pkg/archive"
	docker "github.com/fsouza/go-dockerclient"
)

// RunBuild builds an image if it is out of date
func RunBuild(ctx *context.ExecuteContext, t *Task, hasModifiedDeps bool) (bool, error) {
	if !hasModifiedDeps {
		stale, err := buildIsStale(ctx, t)
		switch {
		case err != nil:
			return false, err
		case !stale:
			t.logger().Info("is fresh")
			return false, nil
		}
	}
	t.logger().Debug("is stale")
	// TODO: check if required fields are set (dockerfile, or steps, and context)

	if err := buildImage(ctx, t); err != nil {
		return false, err
	}

	image, err := GetImage(ctx, t.config)
	if err != nil {
		return false, err
	}

	record := imageModifiedRecord{ImageID: image.ID}
	if err := updateImageRecord(recordPath(ctx, t.config), record); err != nil {
		t.logger().Warnf("Failed to update image record: %s", err)
	}
	t.logger().Info("Created")
	return true, nil
}

// TODO: this cyclo problem should be fixed
// nolint: gocyclo
func buildIsStale(ctx *context.ExecuteContext, t *Task) (bool, error) {
	image, err := GetImage(ctx, t.config)
	switch err {
	case docker.ErrNoSuchImage:
		t.logger().Debug("Image does not exist")
		return true, nil
	case nil:
	default:
		return true, err
	}

	paths := []string{t.config.Context}
	// TODO: polymorphic config for different types of images
	if t.config.Steps != "" && ctx.ConfigFile != "" {
		paths = append(paths, ctx.ConfigFile)
	}
	mtime, err := fs.LastModified(paths...)
	if err != nil {
		t.logger().Warnf("Failed to get last modified time of context.")
		return true, err
	}

	record, err := getImageRecord(recordPath(ctx, t.config))
	if err != nil {
		t.logger().Warnf("Failed to get image record: %s", err)
		if image.Created.Before(mtime) {
			t.logger().Debug("Image older than context")
			return true, nil
		}
		return false, nil
	}

	if image.ID != record.ImageID || record.Info.ModTime().Before(mtime) {
		t.logger().Debug("Image record older than context")
		return true, nil
	}
	return false, nil
}

func buildImage(ctx *context.ExecuteContext, t *Task) error {
	var err error
	if t.config.Steps != "" {
		err = t.buildImageFromSteps(ctx)
	} else {
		err = t.buildImageFromDockerfile(ctx)
	}
	if err != nil {
		return err
	}
	image, err := GetImage(ctx, t.config)
	if err != nil {
		return err
	}
	record := imageModifiedRecord{ImageID: image.ID}
	return updateImageRecord(recordPath(ctx, t.config), record)
}

func (t *Task) buildImageFromDockerfile(ctx *context.ExecuteContext) error {
	return Stream(os.Stdout, func(out io.Writer) error {
		opts := t.commonBuildImageOptions(ctx, out)
		opts.Dockerfile = t.config.Dockerfile
		opts.ContextDir = t.config.Context
		return ctx.Client.BuildImage(opts)
	})
}

func (t *Task) commonBuildImageOptions(
	ctx *context.ExecuteContext,
	out io.Writer,
) docker.BuildImageOptions {
	return docker.BuildImageOptions{
		Name:           GetImageName(ctx, t.config),
		BuildArgs:      buildArgs(t.config.Args),
		Pull:           t.config.PullBaseImageOnBuild,
		RmTmpContainer: true,
		OutputStream:   out,
		RawJSONStream:  true,
		SuppressOutput: ctx.Quiet,
		AuthConfigs:    ctx.GetAuthConfigs(),
	}
}

func buildArgs(args map[string]string) []docker.BuildArg {
	out := []docker.BuildArg{}
	for key, value := range args {
		out = append(out, docker.BuildArg{Name: key, Value: value})
	}
	return out
}

func (t *Task) buildImageFromSteps(ctx *context.ExecuteContext) error {
	buildContext, dockerfile, err := getBuildContext(t.config)
	if err != nil {
		return err
	}
	return Stream(os.Stdout, func(out io.Writer) error {
		opts := t.commonBuildImageOptions(ctx, out)
		opts.InputStream = buildContext
		opts.Dockerfile = dockerfile
		return ctx.Client.BuildImage(opts)
	})
}

func getBuildContext(config *config.ImageConfig) (io.Reader, string, error) {
	contextDir := config.Context
	excludes, err := build.ReadDockerignore(contextDir)
	if err != nil {
		return nil, "", err
	}
	if err := build.ValidateContextDirectory(contextDir, excludes); err != nil {
		return nil, "", err

	}
	buildCtx, err := archive.TarWithOptions(contextDir, &archive.TarOptions{
		ExcludePatterns: excludes,
	})
	if err != nil {
		return nil, "", err
	}
	dockerfileCtx := ioutil.NopCloser(strings.NewReader(config.Steps))
	return build.AddDockerfileToBuildContext(dockerfileCtx, buildCtx)
}
