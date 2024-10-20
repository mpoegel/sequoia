package web

import (
	"context"
	"flag"
)

type Options struct {
	Addr           string
	StaticDir      string
	CollectionAddr string

	Ctx context.Context
}

func Run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("web", flag.ExitOnError)
	opt := Options{
		Ctx: ctx,
	}

	fs.StringVar(&opt.Addr, "l", ":0", "listen address")
	fs.StringVar(&opt.StaticDir, "staticDir", "static", "directory containing static assets")
	fs.StringVar(&opt.CollectionAddr, "collection", "unix:///tmp/sequoia.collector", "address of the collection")

	if err := fs.Parse(args); err != nil {
		return err
	}

	server, err := NewServer(opt)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		server.Stop()
	}()

	return server.Start()
}
