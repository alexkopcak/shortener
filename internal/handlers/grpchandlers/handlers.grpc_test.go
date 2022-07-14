package handlersgrpc

import (
	"context"
	"log"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/alexkopcak/shortener/client"
	"github.com/alexkopcak/shortener/internal/config"
	pb "github.com/alexkopcak/shortener/internal/handlers/grpchandlers/proto"
	"github.com/alexkopcak/shortener/internal/storage"
)

const bufSize = 1024 * 1024

var (
	lis *bufconn.Listener
	s   *grpc.Server
	cfg = config.Config{
		BaseURL:        "http://localhost:8080",
		SecretKey:      "secret key",
		CookieAuthName: "id",
		TrustedSubnet:  "10.0.0.0/8",
		GrpcAddr:       ":8181",
	}
	xRealIp = "10.0.12.3"
)

func init() {
	lis = bufconn.Listen(bufSize)

	interceptor := NewAuthServerInterceptor(&cfg)

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(interceptor.Unary()),
	}
	s = grpc.NewServer(opts...)

	dChan := make(chan *storage.DeletedShortURLValues)

	repo, err := storage.NewDictionary(cfg, &sync.WaitGroup{}, dChan)

	if err != nil {
		log.Fatal(err)
	}

	pb.RegisterShortenerServer(s, NewGRPCHandler(&repo, cfg, dChan))
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
}

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestGRPC(t *testing.T) {
	ctx := context.Background()

	transportOption := grpc.WithTransportCredentials(insecure.NewCredentials())

	conn1, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer), transportOption)
	require.NoError(t, err)
	defer conn1.Close()

	authClient := client.NewAuthClient(conn1)
	interceptor, err := client.NewAuthClientInterceptor(authClient, cfg.CookieAuthName, xRealIp)
	require.NoError(t, err)

	conn2, err := grpc.DialContext(ctx,
		"bufnet",
		grpc.WithContextDialer(bufDialer),
		transportOption,
		grpc.WithUnaryInterceptor(interceptor.Unary()),
		grpc.WithStreamInterceptor(interceptor.Stream()))

	require.NoError(t, err)
	defer conn2.Close()

	client := pb.NewShortenerClient(conn2)

	originalURL1 := "http://original.value.test"
	originalURL2 := "http://original.test.value"

	// generate shortURL for OriginalURL value
	shortURLRaw, err := client.PostURL(ctx, &pb.URLRequest{
		Value: originalURL1,
	})
	require.NoError(t, err)

	shortURL := strings.Replace(shortURLRaw.Value, cfg.BaseURL+"/", "", -1)

	// get originalURL value by ShortURL value
	originalURLRaw, err := client.GetURL(ctx, &pb.URLRequest{
		Value: shortURL,
	})
	require.NoError(t, err)
	require.Equal(t, originalURL1, originalURLRaw.Value)

	shortURLRaw2, err := client.PostAPIurl(ctx, &pb.URLRequest{
		Value: originalURL2,
	})
	require.NoError(t, err)
	require.NotEqual(t, shortURLRaw, shortURLRaw2)
	shortURL2 := strings.Replace(shortURLRaw2.Value, cfg.BaseURL+"/", "", -1)

	respRaw, err := client.GetAllURL(ctx, &pb.Empty{})
	require.NoError(t, err)
	require.EqualValues(t, 2, respRaw.Count)

	postBatchRaw, err := client.PostAPIBatch(ctx, &pb.BatchRequestArray{
		Count: 2,
		OriginalUrls: []*pb.BatchRequestArray_BatchRequest{
			{
				CorrelationId: "1",
				OriginalUrl:   originalURL1,
			},
			{
				CorrelationId: "2",
				OriginalUrl:   originalURL2,
			},
		},
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, postBatchRaw.Count)

	_, err = client.DeleteURLs(ctx, &pb.AnyURLRequest{
		Count:  1,
		Values: []string{shortURL2},
	})
	require.NoError(t, err)

	respRaw2, err := client.GetAllURL(ctx, &pb.Empty{})
	require.NoError(t, err)
	require.EqualValues(t, 1, respRaw2.Count)

	require.NoError(t, err)

	stats, err := client.GetInternalStats(ctx, &pb.Empty{})
	require.NoError(t, err)
	require.EqualValues(t, 2, stats.UrlsCount)
	require.EqualValues(t, 1, stats.UsersCount)
	s.GracefulStop()
}
