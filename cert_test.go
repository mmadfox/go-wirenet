package wirenet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadCertificates(t *testing.T) {
	conf, err := LoadCertificates("server", "./certs")
	assert.Nil(t, err)
	assert.Len(t, conf.Certificates, 1)

	conf, err = LoadCertificates("client", "./certs")
	assert.Nil(t, err)
	assert.Len(t, conf.Certificates, 1)

	conf, err = LoadCertificates("", "./certs")
	assert.Equal(t, ErrUnknownCertificateName, err)
	assert.Nil(t, conf)
}
