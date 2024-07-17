package webtunnel

import (
	"errors"
	"net"

	utls "github.com/refraction-networking/utls"

	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/common/utlsutil"
)

type uTLSConfig struct {
	ServerName string

	uTLSFingerprint string
}

func newUTLSTransport(config *uTLSConfig) (uTLSTransport, error) {
	return uTLSTransport{kind: "utls", serverName: config.ServerName, uTLSFingerprint: config.uTLSFingerprint}, nil
}

type uTLSTransport struct {
	kind       string
	serverName string

	uTLSFingerprint string
}

func (t *uTLSTransport) Client(conn net.Conn) (net.Conn, error) {
	switch t.kind {
	case "utls":
		fp, err := utlsutil.ParseClientHelloID(t.uTLSFingerprint)
		if err != nil {
			return nil, err
		}
		conf := &utls.Config{ServerName: t.serverName}
		return utls.UClient(conn, conf, *fp), nil
	}
	return nil, errors.New("unknown kind")
}
