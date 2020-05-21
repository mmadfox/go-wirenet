package wirenet

import (
	"crypto/tls"
	"path/filepath"
)

func LoadCertificates(name string, certPath string) (*tls.Config, error) {
	if len(name) == 0 {
		return nil, ErrUnknownCertificateName
	}
	pemFile := filepath.Join(certPath, name+".pem")
	keyFile := filepath.Join(certPath, name+".key")

	cert, err := tls.LoadX509KeyPair(pemFile, keyFile)
	if err != nil {
		return nil, err
	}

	conf := tls.Config{Certificates: []tls.Certificate{cert}}
	// conf.Rand = rand.Reader

	return &conf, nil
}
