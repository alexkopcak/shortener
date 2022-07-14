package client

import (
	"context"
	"time"

	pb "github.com/alexkopcak/shortener/internal/handlers/grpchandlers/proto"
	"google.golang.org/grpc"
)

type AuthClient struct {
	service pb.ShortenerClient
}

func NewAuthClient(cc *grpc.ClientConn) *AuthClient {
	service := pb.NewShortenerClient(cc)
	return &AuthClient{service: service}
}

func (client *AuthClient) Login() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := client.service.Login(ctx, &pb.Empty{})
	if err != nil {
		return "", err
	}
	return res.Value, err
}
