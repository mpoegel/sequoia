package collect

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	camera "github.com/mpoegel/sequoia/pkg/camera"
	schema "github.com/mpoegel/sequoia/pkg/schema"
	gocv "gocv.io/x/gocv"
	grpc "google.golang.org/grpc"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

type ImageServer struct {
	schema.UnimplementedImageServiceServer

	opt Options

	fullAddr string
	network  string
	addr     string

	ln         net.Listener
	liveBroker *Broker[*ProcessedImage]
}

type ProcessedImage struct {
	URL       string
	Timestamp time.Time
	ID        string
}

func NewImageServer(opt Options) (*ImageServer, error) {
	s := &ImageServer{
		opt:        opt,
		fullAddr:   opt.Addr,
		ln:         nil,
		liveBroker: NewBroker[*ProcessedImage](),
	}

	splitKey := "://"
	splitIndex := strings.Index(opt.Addr, splitKey)
	if splitIndex == -1 {
		return nil, errors.New("invalid server address")
	}
	s.network = opt.Addr[:splitIndex]
	s.addr = opt.Addr[splitIndex+len(splitKey):]

	return s, nil
}

func (s *ImageServer) Start(ctx context.Context) error {
	lnConfig := net.ListenConfig{}

	ln, err := lnConfig.Listen(ctx, s.network, s.addr)
	if err != nil {
		return err
	}
	s.ln = ln
	slog.Info("listening", "addr", s.fullAddr)

	go s.liveBroker.Start()

	grpcServer := grpc.NewServer()
	schema.RegisterImageServiceServer(grpcServer, s)
	return grpcServer.Serve(ln)
}

func (s *ImageServer) Stop() {
	if s.ln != nil {
		s.ln.Close()
	}
	s.liveBroker.Stop()
}

func (s *ImageServer) StoreRawImage(ctx context.Context, req *schema.StoreRawImageRequest) (*schema.StoreRawImageResponse, error) {
	slog.Info("got store raw image request", "req.id", req.Id, "timestamp", req.Timestamp)
	resp := &schema.StoreRawImageResponse{}

	img, err := gocv.NewMatFromBytes(int(req.Image.NumRows), int(req.Image.NumCols), gocv.MatType(req.Image.ImageType), req.Image.RawImage)
	if err != nil {
		slog.Warn("raw image could not be parsed", "err", err)
		return resp, nil
	}
	pImg, err := s.processImage(img, req.Timestamp.AsTime(), req.Id)
	if err != nil {
		slog.Warn("could not process image", "err", err)
		return resp, nil
	}

	s.liveBroker.Broadcast(pImg)
	slog.Info("image broadcasted", "id", pImg.ID)

	return resp, nil
}

func (s *ImageServer) processImage(img gocv.Mat, ts time.Time, id string) (*ProcessedImage, error) {
	filename := fmt.Sprintf("%s.jpg", ts.Format(camera.TimeFormat))
	absoluteFilename := fmt.Sprintf("%s/%s", s.opt.ImgDir, filename)

	if ok := gocv.IMWrite(absoluteFilename, img); !ok {
		return nil, errors.New("could not save image")
	}

	pImg := &ProcessedImage{
		URL:       fmt.Sprintf("%s/%s", s.opt.ProxyAddr, filename),
		Timestamp: ts,
		ID:        id,
	}
	return pImg, nil
}

func (s *ImageServer) LiveStream(req *schema.LiveStreamRequest, stream schema.ImageService_LiveStreamServer) error {
	c := s.liveBroker.Subscribe()
	if c == nil {
		return errors.New("subscription unavailable")
	}
	defer s.liveBroker.Unsubscribe(c)

	var img *ProcessedImage
	for {
		img = <-c
		if img == nil {
			break
		}
		err := stream.Send(&schema.LiveStreamResponse{
			ImageUrl:  img.URL,
			Timestamp: timestamppb.New(img.Timestamp),
			Id:        img.ID,
		})
		if err != nil {
			break
		}
		slog.Debug("live stream updated")
	}
	return nil
}
