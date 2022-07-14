package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/alexkopcak/shortener/internal/config"
	handlersgrpc "github.com/alexkopcak/shortener/internal/handlers/grpchandlers"
	handlers "github.com/alexkopcak/shortener/internal/handlers/resthandlers"
	"github.com/alexkopcak/shortener/internal/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/alexkopcak/shortener/internal/handlers/grpchandlers/proto"
)

const (
	certFile = "localhost.crt"
	keyFile  = "localhost.key"
)

type App struct {
	wg         *sync.WaitGroup
	repository storage.Storage
	cfg        *config.Config
	restServer *http.Server
	grpcServer *grpc.Server
	dChannel   chan *storage.DeletedShortURLValues
}

func NewApp(conf config.Config) *App {
	return &App{
		cfg: &conf,
	}
}

func (a *App) Run() error {
	a.wg = &sync.WaitGroup{}
	a.dChannel = make(chan *storage.DeletedShortURLValues)

	// Repository
	var err error
	a.repository, err = storage.InitializeStorage(*a.cfg, a.wg, a.dChannel)
	if err != nil {
		return err
	}

	// the channel notifying about the closure of connections
	iddleConnsClosed := make(chan struct{})
	// interrupt redirection channel
	sigint := make(chan os.Signal, 1)
	// regiser signal notifications
	signal.Notify(sigint, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	go func() {
		<-sigint

		close(iddleConnsClosed)
	}()

	go func() {
		<-iddleConnsClosed
		if err = a.restServer.Shutdown(context.Background()); err != nil {
			log.Printf("HTTP server Shutdown: %v", err)
		}
	}()

	go func() {
		<-iddleConnsClosed
		if a.grpcServer != nil {
			a.grpcServer.GracefulStop()
		}
	}()

	// start servers and wait
	a.start()

	err = a.repository.Close()

	close(sigint)
	close(a.dChannel)

	log.Println("server shutdown gracefully")
	return err
}

func (a *App) start() {
	if a.cfg.EnableHTTPS {
		if err := checkKeyAndCert(); err != nil {
			log.Fatal(err)
		}
	}
	a.wg.Add(1)
	go func() {
		err := a.startREST()
		if err != http.ErrServerClosed {
			log.Fatalf("HTTP server ListenAndServe: %v", err)
		}
		a.wg.Done()
	}()
	a.wg.Add(1)
	go func() {
		err := a.startGRPC()
		if err != nil && err != grpc.ErrServerStopped {
			log.Fatalf("gRPC server ListenAndServe: %v", err)
		}
		a.wg.Done()
	}()
	a.wg.Wait()
}

func (a *App) startREST() error {
	//HTTP Server
	a.restServer = &http.Server{
		Addr:    a.cfg.ServerAddr,
		Handler: handlers.NewURLHandler(a.repository, *a.cfg, a.dChannel),
	}
	var err error

	// start server
	log.Printf("rest server start on %v", a.cfg.ServerAddr)
	if a.cfg.EnableHTTPS {
		err = a.restServer.ListenAndServeTLS(certFile, keyFile)
	} else {
		err = a.restServer.ListenAndServe()
	}
	return err
}

func (a *App) startGRPC() error {
	if strings.TrimSpace(a.cfg.GrpcAddr) == "" {
		return nil
	}

	listen, err := net.Listen("tcp", a.cfg.GrpcAddr)
	if err != nil {
		log.Fatal(err)
	}

	interceptor := handlersgrpc.NewAuthServerInterceptor(a.cfg)

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(interceptor.Unary()),
	}
	if a.cfg.EnableHTTPS {
		creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
		if err != nil {
			log.Fatalf("failed to generate credentials, err: %v", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	a.grpcServer = grpc.NewServer(
		opts...,
	)

	pb.RegisterShortenerServer(a.grpcServer,
		handlersgrpc.NewGRPCHandler(&a.repository, *a.cfg, a.dChannel))

	log.Printf("grpc server start on %v", a.cfg.GrpcAddr)
	return a.grpcServer.Serve(listen)
}

func checkKeyAndCert() error {
	var err error
	if !exists(keyFile) {
		if err = keyFileCreate(); err != nil {
			return err
		}
	}
	if !exists(certFile) {
		if err = certFileCreate(); err != nil {
			return err
		}
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
}

func keyFileCreate() error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)

	if err != nil {
		return err
	}

	var privateKeyPEM bytes.Buffer
	pem.Encode(&privateKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	return os.WriteFile(keyFile, privateKeyPEM.Bytes(), 0600)
}

func certFileCreate() error {
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(11943257),
		Subject: pkix.Name{
			Organization: []string{"Roga & Kopyta"},
			Country:      []string{"RU"},
		},
		IPAddresses: []net.IP{{127, 0, 0, 1}, net.IPv6loopback},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		DNSNames:    []string{"localhost"},
	}

	pemContent, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return err
	}

	block, _ := pem.Decode(pemContent)

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, cert, &privateKey.PublicKey, privateKey)
	if err != nil {
		return err
	}

	var certPEM bytes.Buffer
	pem.Encode(&certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	return os.WriteFile(certFile, certPEM.Bytes(), 0644)
}
