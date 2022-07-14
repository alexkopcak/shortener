// Package handlergrpc implements shortener grpc server.
package handlersgrpc

import (
	"context"
	"errors"
	"fmt"
	"net"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/alexkopcak/shortener/internal/config"
	handlershelper "github.com/alexkopcak/shortener/internal/handlers"
	pb "github.com/alexkopcak/shortener/internal/handlers/grpchandlers/proto"
	"github.com/alexkopcak/shortener/internal/storage"
)

type (
	GRPCHandler struct {
		pb.UnimplementedShortenerServer
		trustedNet *net.IPNet
		dChannel   chan *storage.DeletedShortURLValues
		repo       storage.Storage
		cfg        *config.Config
	}

	key uint64
)

const (
	keyPrincipalID key = iota
)

// NewGRPCHandler create handler object.
func NewGRPCHandler(store *storage.Storage, conf config.Config, dChan chan *storage.DeletedShortURLValues) *GRPCHandler {
	return &GRPCHandler{
		cfg:        &conf,
		repo:       *store,
		trustedNet: handlershelper.SetTrustedSubnet(conf.TrustedSubnet),
		dChannel:   dChan,
	}
}

// Login obtains JW Token
func (g *GRPCHandler) Login(ctx context.Context, in *pb.Empty) (*pb.Token, error) {
	token, _, err := handlershelper.GenerateJWT(g.cfg.SecretKey)
	return &pb.Token{
		Value: token,
	}, err
}

// GetURL obtains OriginalURL for ShortURL value
func (g *GRPCHandler) GetURL(ctx context.Context, in *pb.URLRequest) (*pb.URLResponse, error) {
	longURLValue, err := g.repo.GetURL(ctx, in.Value)
	if err != nil {
		if errors.Is(err, storage.ErrNotExistRecord) {
			return nil, status.Errorf(codes.NotFound, "url %s not found", in.Value)
		}
		return nil, status.Errorf(codes.Unimplemented, "")
	}

	return &pb.URLResponse{
		Value: longURLValue,
	}, nil
}

// GetAllURL obtains all URLs saved by the user in the format of pairs of OriginURL and ShortURL
func (g *GRPCHandler) GetAllURL(ctx context.Context, in *pb.Empty) (*pb.AnyURLResponse, error) {
	userID, ok := ctx.Value(keyPrincipalID).(int32)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "unknown user")
	}

	result, err := g.repo.GetUserURL(ctx, g.cfg.BaseURL, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error GetUserURL func %v", err)
	}

	items := make([]*pb.AnyURLResponse_ShortOriginalURLPairs, 0, len(result))
	if len(result) > 0 {
		for _, v := range result {
			items = append(items, &pb.AnyURLResponse_ShortOriginalURLPairs{
				ShortURL:    v.ShortURL,
				OriginalURL: v.OriginalURL,
			})
		}
	}

	return &pb.AnyURLResponse{
		Count:  int32(len(result)),
		Values: items,
	}, nil
}

// PostURL obtain ShortURL value for OriginalURL and save it at storage
func (g *GRPCHandler) PostURL(ctx context.Context, in *pb.URLRequest) (*pb.URLResponse, error) {
	userID, ok := ctx.Value(keyPrincipalID).(int32)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "unknown user")
	}

	result, err := g.repo.AddURL(ctx, in.Value, storage.ShortURLGenerator(), userID)
	if err != nil {
		if errors.Is(err, storage.ErrDuplicateRecord) {
			return &pb.URLResponse{
				Value: fmt.Sprintf("%s/%s", g.cfg.BaseURL, result),
			}, status.Errorf(codes.AlreadyExists, "duplicated value")

		}
		return nil, status.Errorf(codes.Internal, "internal error: %v", err)
	}

	return &pb.URLResponse{
		Value: fmt.Sprintf("%s/%s", g.cfg.BaseURL, result),
	}, nil
}

// PostAPIurl obtain ShortURL value for OriginalURL and save it at storage
func (g *GRPCHandler) PostAPIurl(ctx context.Context, in *pb.URLRequest) (*pb.URLResponse, error) {
	return g.PostURL(ctx, in)
}

// PostAPIBatch obtain ShortURL values for OriginalURL at batch request and save values at storage
func (g *GRPCHandler) PostAPIBatch(ctx context.Context, in *pb.BatchRequestArray) (*pb.BatchResponseArray, error) {
	userID, ok := ctx.Value(keyPrincipalID).(int32)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "unknown user")
	}

	batchReqArray := make(storage.BatchRequestArray, 0, in.Count)
	for _, val := range in.OriginalUrls {
		item := storage.BatchRequest{
			CorrelationID: val.CorrelationId,
			OriginalURL:   val.OriginalUrl,
		}
		batchReqArray = append(batchReqArray, item)
	}

	result, err := g.repo.PostAPIBatch(ctx, &batchReqArray, g.cfg.BaseURL, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "internal error: %v", err)
	}

	responseArray := make([]*pb.BatchResponseArray_BatchResponse, 0, len(*result))
	for _, v := range *result {
		item := pb.BatchResponseArray_BatchResponse{
			CorrelationId: v.CorrelationID,
			ShortUrl:      v.ShortURL,
		}
		responseArray = append(responseArray, &item)
	}

	return &pb.BatchResponseArray{
		Count:     int32(len(responseArray)),
		ShortUrls: responseArray,
	}, nil
}

// DeleteURLs delete stored URL values by ShortURL
func (g *GRPCHandler) DeleteURLs(ctx context.Context, in *pb.AnyURLRequest) (*pb.Empty, error) {
	userID, ok := ctx.Value(keyPrincipalID).(int32)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "unknown user")
	}

	deletedURLs := &storage.DeletedShortURLValues{
		ShortURLValues: in.Values,
		UserIDValue:    userID,
	}

	g.dChannel <- deletedURLs

	return &pb.Empty{}, nil
}

// GetInternalStats  get stats URLs and Users count
func (g *GRPCHandler) GetInternalStats(ctx context.Context, in *pb.Empty) (*pb.InternalStatsResponse, error) {
	if g.trustedNet == nil {
		return nil, status.Errorf(codes.PermissionDenied, "forbidden")
	}

	if ctx.Value("X-Real-IP") == nil {
		return nil, status.Errorf(codes.PermissionDenied, "where are no X-Real-IP")
	}

	xRealIp, ok := ctx.Value("X-Real-IP").(string)
	if !ok {
		return nil, status.Errorf(codes.PermissionDenied, "bad ip value")
	}

	if !g.trustedNet.Contains(net.ParseIP(xRealIp)) {
		return nil, status.Errorf(codes.PermissionDenied, "forbidden")
	}

	stats, err := g.repo.GetInternalStats(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "something went wrong %v", err)
	}

	return &pb.InternalStatsResponse{
		UrlsCount:  int32(stats.URLs),
		UsersCount: int32(stats.Users),
	}, nil
}
