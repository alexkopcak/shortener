package handlershelper

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"log"
	"net"
	"strings"
)

var (
	ErrNotEqual = errors.New("NotEqualTokenError")
)

func DecodeJWT(secretKey, value string) (int32, error) {
	data, err := hex.DecodeString(value)
	if err != nil {
		return 0, err
	}

	var id int32
	err = binary.Read(bytes.NewReader(data[:4]), binary.BigEndian, &id)
	if err != nil {
		return 0, err
	}

	hm := hmac.New(sha256.New, []byte(secretKey))
	hm.Write(data[:4])
	sign := hm.Sum(nil)
	if hmac.Equal(data[4:], sign) {
		return id, nil
	}
	return 0, ErrNotEqual
}

func GenerateJWT(secretKey string) (string, int32, error) {
	id := make([]byte, 4)

	_, err := rand.Read(id)
	if err != nil {
		return "", 0, err
	}

	hm := hmac.New(sha256.New, []byte(secretKey))
	hm.Write(id)
	sign := hex.EncodeToString(append(id, hm.Sum(nil)...))

	var result int32
	err = binary.Read(bytes.NewReader(id), binary.BigEndian, &result)

	return sign, result, err
}

func SetTrustedSubnet(subnet string) *net.IPNet {
	if strings.TrimSpace(subnet) == "" {
		return nil
	}
	_, trustedNet, err := net.ParseCIDR(subnet)
	if err != nil {
		trustedNet = nil
		log.Printf("%s\nbad trusted subnet value \"%s\" ; trusted subnet is empty value\n", err, subnet)
	}
	return trustedNet
}
