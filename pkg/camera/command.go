package camera

import (
	"context"
	"flag"
	"log/slog"
	"time"
)

type Options struct {
	Save       string
	DeviceID   string
	Frequency  time.Duration
	NotifyAddr string
}

func Run(ctx context.Context, args []string) error {

	fs := flag.NewFlagSet("camera", flag.ExitOnError)
	opt := Options{}

	fs.StringVar(&opt.Save, "s", "file:///tmp", "place to save pictures")
	fs.StringVar(&opt.DeviceID, "d", "0", "camera device ID")
	fs.DurationVar(&opt.Frequency, "f", 5*time.Second, "picture interval")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return cameraLoop(ctx, opt)
}

func cameraLoop(ctx context.Context, opt Options) error {
	cam, err := NewCamera(ctx, opt.DeviceID, opt.Save)
	if err != nil {
		return err
	}
	if err := cam.Open(); err != nil {
		slog.Warn("failed to open camera", "err", err)
		return err
	}
	defer cam.Close()

	ticker := time.NewTicker(opt.Frequency)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := cam.TakePicture(); err != nil {
				slog.Warn("failed to take picture", "err", err)
			} else {
				slog.Debug("took picture", "device", opt.DeviceID)
			}
		case <-ctx.Done():
			return nil
		}
	}
}
