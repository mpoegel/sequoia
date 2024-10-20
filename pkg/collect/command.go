package collect

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"strings"
)

type Options struct {
	Addr      string
	ImgDir    string
	ProxyAddr string
}

func Run(ctx context.Context, args []string) error {

	fs := flag.NewFlagSet("collect", flag.ExitOnError)
	opt := Options{}
	fs.StringVar(&opt.Addr, "l", "unix:///tmp/sequoia.collector", "listen address")
	fs.StringVar(&opt.ImgDir, "d", "/tmp", "directory in which to store images")
	fs.StringVar(&opt.ProxyAddr, "proxy", "http://localhost:8000/image", "proxy that serves images; using localhost will start a local proxy")

	if err := fs.Parse(args); err != nil {
		return err
	}

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var proxyServer *ProxyServer
	if strings.HasPrefix(opt.ProxyAddr, "http://localhost") {
		proxyServer = NewProxyServer(opt)
		go func() {
			if err := proxyServer.Start(); err != nil && !errors.Is(err, net.ErrClosed) {
				slog.Error("failed to start proxy server", "err", err)
				cancel()
			}
		}()
	}

	server, err := NewImageServer(opt)
	if err != nil {
		return err
	}

	go func() {
		<-subCtx.Done()
		server.Stop()
		if proxyServer != nil {
			proxyServer.Stop()
		}
	}()

	if err := server.Start(subCtx); !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}
