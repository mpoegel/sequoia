package cleanup

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	camera "github.com/mpoegel/sequoia/pkg/camera"
)

type Options struct {
	SaveDir   string
	OlderThan time.Duration
}

func Run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("cleanup", flag.ExitOnError)
	opt := Options{}

	fs.StringVar(&opt.SaveDir, "d", "/tmp", "directory in which to delete saved pictures")
	fs.DurationVar(&opt.OlderThan, "s", 24*7*time.Hour, "delete pictures older than this duration from now")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return cleanup(opt)
}

func cleanup(opt Options) error {
	matches, err := filepath.Glob(fmt.Sprintf("%s/*.jpg", opt.SaveDir))
	if err != nil {
		return err
	}

	cutoffTime := time.Now().Add(-1 * opt.OlderThan)
	for _, match := range matches {
		slog.Debug("found file", "file", match)
		if info, err := os.Stat(match); err != nil {
			slog.Error("failed to stat file", "file", match, "err", err)
		} else {
			if ts, err := time.Parse(camera.TimeFormat, strings.Trim(info.Name(), ".jpg")); err != nil {
				slog.Warn("file name does not match expected format", "file", match, "err", err)
			} else {
				if cutoffTime.After(ts) {
					if err := os.Remove(match); err != nil {
						slog.Error("failed to remove file", "file", match, "err", err)
					} else {
						slog.Info("file removed", "file", match)
					}
				}
			}
		}
	}

	return nil
}
