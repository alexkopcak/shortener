package client

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type AuthClientInterceptor struct {
	authClient       *AuthClient
	accessToken      string
	accessTokenField string
	xRealIp          string
}

func NewAuthClientInterceptor(authClient *AuthClient, accessTokenField string, xIp string) (*AuthClientInterceptor, error) {
	interceptor := &AuthClientInterceptor{
		authClient:       authClient,
		accessTokenField: accessTokenField,
		xRealIp:          xIp,
	}

	err := interceptor.getToken()
	if err != nil {
		return nil, err
	}

	return interceptor, nil
}

func (interceptor *AuthClientInterceptor) getToken() error {
	token, err := interceptor.authClient.Login()
	if err != nil {
		return err
	}

	interceptor.accessToken = token
	return nil
}

func (interceptor *AuthClientInterceptor) attachToken(ctx context.Context) context.Context {
	md := metadata.Pairs()
	md.Append(interceptor.accessTokenField, interceptor.accessToken)
	md.Append("X-Real-IP", interceptor.xRealIp)
	return metadata.NewOutgoingContext(ctx, md)
}

func (interceptor *AuthClientInterceptor) Unary() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(interceptor.attachToken(ctx), method, req, reply, cc)
	}
}

func (interceptor *AuthClientInterceptor) Stream() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(interceptor.attachToken(ctx), desc, cc, method)
	}
}
