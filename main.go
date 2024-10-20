package main

//go:generate protoc -I=./schema --go_out=./pkg/schema --go_opt=paths=source_relative --go-grpc_out=./pkg/schema --go-grpc_opt=paths=source_relative ./schema/image_service.proto

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	camera "github.com/mpoegel/sequoia/pkg/camera"
	cleanup "github.com/mpoegel/sequoia/pkg/cleanup"
	collect "github.com/mpoegel/sequoia/pkg/collect"
	web "github.com/mpoegel/sequoia/pkg/web"
)

func init() {
	slog.SetLogLoggerLevel(slog.LevelDebug)
}

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("missing command: [camera, cleanup, collect, web]")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	go func() {
		<-sig
		slog.Info("stopping")
		cancel()
	}()

	var err error
	switch args[0] {
	case "camera":
		err = camera.Run(ctx, args[1:])
	case "cleanup":
		err = cleanup.Run(ctx, args[1:])
	case "collect":
		err = collect.Run(ctx, args[1:])
	case "web":
		err = web.Run(ctx, args[1:])
	default:
		err = fmt.Errorf("unknown command: %s", args[0])
	}

	if err != nil {
		slog.Error("command failed", "err", err)
		os.Exit(1)
	}
}
