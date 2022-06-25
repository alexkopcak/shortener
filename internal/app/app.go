package app

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/alexkopcak/shortener/internal/config"
	"github.com/alexkopcak/shortener/internal/handlers"
	"github.com/alexkopcak/shortener/internal/storage"
)

const (
	certFile = "localhost.crt"
	keyFile  = "localhost.key"
)

func Run(cfg config.Config) error {
	// // check config file os.Env
	// cfgFileName := os.Getenv(envConfigFile)

	// // env configuration
	// cfg := config.NewConfig(cfgFileName)

	// if err := env.Parse(&cfg); err != nil {
	// 	return err
	// }
	// // flags configuration
	// flag.StringVar(&cfg.ServerAddr, "a", cfg.ServerAddr, "Server address, example ip:port")
	// flag.StringVar(&cfg.BaseURL, "b", cfg.BaseURL, "Base URL address, example http://127.0.0.1:8080")
	// flag.StringVar(&cfg.FileStoragePath, "f", cfg.FileStoragePath, "File storage path")
	// flag.StringVar(&cfg.DBConnectionString, "d", cfg.DBConnectionString, "DB connection string")
	// flag.BoolVar(&cfg.EnableHTTPS, "s", cfg.EnableHTTPS, "Enable HTTPS")
	// flag.StringVar(&cfg.ConfigPath, "c", cfg.ConfigPath, "Config file path")
	// flag.StringVar(&cfg.ConfigPath, "config", cfg.ConfigPath, "Config file path")

	// flag.Parse()

	// if strings.TrimSpace(cfg.ConfigPath) != "" && cfgFileName == "" {
	// 	name, err := os.Executable()
	// 	if err != nil {
	// 		return err
	// 	}

	// 	var procAttr os.ProcAttr
	// 	procAttr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}
	// 	procAttr.Env = []string{fmt.Sprintf("%s=%s", envConfigFile, cfg.ConfigPath)}

	// 	proc, err := os.StartProcess(name, os.Args, &procAttr)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	_, err = proc.Wait()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	return nil
	// }

	// fmt.Println(cfg)
	// // Parse Base URL address
	// urlValue, err := url.Parse(cfg.BaseURL)
	// if err != nil {
	// 	return err
	// }

	// if cfg.EnableHTTPS {
	// 	urlValue.Scheme = "https"
	// } else {
	// 	urlValue.Scheme = "http"
	// }
	// cfg.BaseURL = urlValue.String()

	wg := &sync.WaitGroup{}
	dChannel := make(chan *storage.DeletedShortURLValues)

	// Repository
	repository, err := storage.InitializeStorage(cfg, wg, dChannel)
	if err != nil {
		return err
	}

	//HTTP Server
	server := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: handlers.URLHandler(repository, cfg, dChannel),
	}

	// start server
	if cfg.EnableHTTPS {
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

		err = server.ListenAndServeTLS(certFile, keyFile)
	} else {
		err = server.ListenAndServe()
	}

	close(dChannel)
	wg.Wait()

	return err
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
