package handlersgrpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/alexkopcak/shortener/internal/config"
	handlershelper "github.com/alexkopcak/shortener/internal/handlers"
)

type AuthServerInterceptor struct {
	cfg *config.Config
}

func NewAuthServerInterceptor(conf *config.Config) *AuthServerInterceptor {
	return &AuthServerInterceptor{cfg: conf}
}

func (inter *AuthServerInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

		cont, err := inter.authorize(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}

		return handler(cont, req)
	}
}

func (inter *AuthServerInterceptor) authorize(ctx context.Context, method string) (context.Context, error) {
	if method == "/shortener.grpc.Shortener/Login" {
		return ctx, nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx, status.Errorf(codes.Unauthenticated, "metadata is not provided")
	}

	values := md[inter.cfg.CookieAuthName]
	if len(values) == 0 {
		return ctx, status.Errorf(codes.Unauthenticated, "authorization token is not provided")
	}

	userID, err := handlershelper.DecodeJWT(inter.cfg.SecretKey, values[0])
	if err != nil {
		return ctx, status.Errorf(codes.Unauthenticated, "access token is invalid: %v", err)
	}

	ctx = context.WithValue(ctx, keyPrincipalID, userID)
	if xRealIp := md["x-real-ip"]; len(xRealIp) > 0 {
		if xRealIp[0] != "" {
			ctx = context.WithValue(ctx, "X-Real-IP", xRealIp[0])
		}
	}
	return ctx, nil
}
