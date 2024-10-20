package collect

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type ProxyServer struct {
	opt        Options
	httpServer *http.Server
}

func NewProxyServer(opt Options) *ProxyServer {
	mux := http.NewServeMux()

	addr := strings.Replace(opt.ProxyAddr, "http://", "", 1)
	splitIndex := strings.Index(addr, "/")
	if splitIndex != -1 {
		addr = addr[:splitIndex]
	}

	s := &ProxyServer{
		opt: opt,
		httpServer: &http.Server{
			Addr:        addr,
			ReadTimeout: 5 * time.Second,
			Handler:     mux,
		},
	}

	mux.Handle("GET /image/", http.StripPrefix("/image", http.FileServer(http.Dir(opt.ImgDir))))

	return s
}

func (s *ProxyServer) Start() error {
	slog.Info("starting proxy server", "addr", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *ProxyServer) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	s.httpServer.Shutdown(ctx)
}
