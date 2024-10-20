package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"text/template"
	"time"

	schema "github.com/mpoegel/sequoia/pkg/schema"
	grpc "google.golang.org/grpc"
	insecure "google.golang.org/grpc/credentials/insecure"
)

type Server struct {
	opt        Options
	httpServer *http.Server
}

func NewServer(opt Options) (*Server, error) {
	mux := http.NewServeMux()
	s := &Server{
		opt: opt,
		httpServer: &http.Server{
			Addr:        opt.Addr,
			ReadTimeout: 5 * time.Second,
			Handler:     mux,
		},
	}

	mux.HandleFunc("GET /{$}", s.HandleIndex)
	mux.HandleFunc("GET /feed/{feedID}", s.HandleFeed)
	mux.Handle("GET /static/", http.StripPrefix("/static", http.FileServer(http.Dir(opt.StaticDir))))
	mux.Handle("GET /image/", http.StripPrefix("/image", http.FileServer(http.Dir("/tmp"))))

	slog.Info("loaded mux", "route", mux)

	return s, nil
}

func (s *Server) Start() error {
	slog.Info("starting server", "addr", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	s.httpServer.Shutdown(ctx)
}

func (s *Server) HandleIndex(w http.ResponseWriter, r *http.Request) {
	plate, err := loadTemplates(s.opt.StaticDir)
	if err != nil {
		slog.Error("failed to load templates", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err = plate.ExecuteTemplate(w, "IndexView", nil); err != nil {
		slog.Error("failed to execute index template", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Server) HandleFeed(w http.ResponseWriter, r *http.Request) {
	plate, err := loadTemplates(s.opt.StaticDir)
	if err != nil {
		slog.Error("failed to load templates", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "text/event-stream")
	w.Header().Set("cache-control", "no-cache")
	w.Header().Set("connection", "keep-alive")
	w.(http.Flusher).Flush()

	feedID := r.PathValue("feedID")
	slog.Info("got request for feed", "feedID", feedID)

	conn, err := grpc.NewClient(s.opt.CollectionAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprint(w, "event: problem\n")
		fmt.Fprint(w, "data: ")
		if err = plate.ExecuteTemplate(w, "error", err); err != nil {
			slog.Error("failed to execute error template", "err", err)
		}
		fmt.Fprint(w, "\n\n")
	}
	defer conn.Close()

	client := schema.NewImageServiceClient(conn)
	req := &schema.LiveStreamRequest{}
	stream, err := client.LiveStream(r.Context(), req)
	if err != nil {
		fmt.Fprint(w, "event: problem\n")
		fmt.Fprint(w, "data: ")
		if err = plate.ExecuteTemplate(w, "error", err); err != nil {
			slog.Error("failed to execute error template", "err", err)
		}
		fmt.Fprint(w, "\n\n")
		return
	}

	for {
		resp, err := stream.Recv()
		if err != nil && errors.Is(err, io.EOF) {
			fmt.Fprint(w, "event: problem\n")
			fmt.Fprint(w, "data: ")
			if err = plate.ExecuteTemplate(w, "error", "feed ended"); err != nil {
				slog.Error("failed to execute error template", "err", err)

			}
			fmt.Fprint(w, "\n\n")
			return
		} else if err != nil {
			fmt.Fprint(w, "event: problem\n")
			fmt.Fprint(w, "data: ")
			if err = plate.ExecuteTemplate(w, "error", err); err != nil {
				slog.Error("failed to execute error template", "err", err)

			}
			fmt.Fprint(w, "\n\n")
			return
		} else {
			view := FeedView{
				URL:       strings.Replace(resp.ImageUrl, "file:///tmp", "/image", 1),
				Timestamp: resp.Timestamp.AsTime(),
				ID:        resp.Id,
			}
			fmt.Fprint(w, "event: feed\n")
			fmt.Fprintf(w, "data: ")
			if err = plate.ExecuteTemplate(w, "feed", view); err != nil {
				slog.Error("failed to execute error template", "err", err)
			}
			fmt.Fprint(w, "\n\n")
		}
		w.(http.Flusher).Flush()
	}
}

func loadTemplates(baseDir string) (plate *template.Template, err error) {
	plate = template.New("").Funcs(template.FuncMap{})
	plate, err = plate.ParseGlob(path.Join(baseDir, "views/*.html"))
	if err != nil {
		return
	}
	return
}
