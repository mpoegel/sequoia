package camera

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	schema "github.com/mpoegel/sequoia/pkg/schema"
	gocv "gocv.io/x/gocv"
	grpc "google.golang.org/grpc"
	insecure "google.golang.org/grpc/credentials/insecure"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

const (
	TimeFormat = "2006-01-02T15:04:05.000"
)

type Camera struct {
	deviceID string
	cam      *gocv.VideoCapture
	saver    ImageSaver
}

type ImageSaver interface {
	Save(img gocv.Mat) error
	Close()
}

func NewCamera(ctx context.Context, deviceID, destination string) (*Camera, error) {
	c := &Camera{
		deviceID: deviceID,
		cam:      nil,
	}

	splitKey := "://"
	splitIndex := strings.Index(destination, splitKey)
	if splitIndex == -1 {
		return nil, fmt.Errorf("invalid destination: %s", destination)
	}
	saveType := destination[:splitIndex]
	saveDest := destination[splitIndex+len(splitKey):]

	switch saveType {
	case "file":
		c.saver = &FileImageSaver{Dir: saveDest}
	case "tcp":
		fallthrough
	case "unix":
		c.saver = &RemoteImageSaver{Network: saveType, Address: saveDest, Ctx: ctx, DeviceID: deviceID}
	default:
		return nil, errors.New("invalid destination types")
	}

	return c, nil
}

func (c *Camera) Open() error {
	cam, err := gocv.OpenVideoCapture(c.deviceID)
	if err != nil {
		return err
	}
	c.cam = cam
	return nil
}

func (c *Camera) Close() {
	if c.cam != nil {
		c.cam.Close()
	}
}

func (c *Camera) TakePicture() error {
	img := gocv.NewMat()
	defer img.Close()

	if ok := c.cam.Read(&img); !ok {
		return errors.New("cannot read device")
	}

	if img.Empty() {
		return errors.New("no image on device")
	}

	return c.saver.Save(img)
}

type FileImageSaver struct {
	Dir string
}

func (s *FileImageSaver) Save(img gocv.Mat) error {
	now := time.Now()
	filename := fmt.Sprintf("%s/%s.jpg", s.Dir, now.Format(TimeFormat))

	if ok := gocv.IMWrite(filename, img); !ok {
		return errors.New("could not save image")
	}
	return nil
}

func (s *FileImageSaver) Close() {}

type RemoteImageSaver struct {
	Network  string
	Address  string
	Ctx      context.Context
	DeviceID string

	conn   *grpc.ClientConn
	client schema.ImageServiceClient
}

func (s *RemoteImageSaver) Save(img gocv.Mat) error {
	if s.client == nil {
		if err := s.connect(); err != nil {
			return err
		}
	}

	now := time.Now()
	req := &schema.StoreRawImageRequest{
		Image: &schema.RawImage{
			RawImage:  img.ToBytes(),
			NumRows:   int32(img.Rows()),
			NumCols:   int32(img.Cols()),
			ImageType: int32(img.Type()),
		},
		Timestamp: timestamppb.New(now),
		Id:        fmt.Sprintf("device%s.%d", s.DeviceID, now.Unix()),
	}

	ctx, cancel := context.WithTimeout(s.Ctx, 1*time.Second)
	defer cancel()

	resp, err := s.client.StoreRawImage(ctx, req)
	if err != nil {
		return err
	}

	slog.Debug("image stored", "resp", resp)
	return nil
}

func (c *RemoteImageSaver) connect() error {
	conn, err := grpc.NewClient(c.getTarget(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	c.conn = conn
	c.client = schema.NewImageServiceClient(c.conn)

	return nil
}

func (c *RemoteImageSaver) getTarget() string {
	if c.Network == "unix" {
		return fmt.Sprintf("unix://%s", c.Address)
	}
	return c.Address
}

func (c *RemoteImageSaver) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
