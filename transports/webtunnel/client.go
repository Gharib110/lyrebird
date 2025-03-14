package webtunnel

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	pt "gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/goptlib"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/webtunnel/transport/httpupgrade"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/webtunnel/transport/tls"

	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/transports/base"
)

type clientConfig struct {
	RemoteAddress string

	Path            string
	TLSKind         string
	TLSServerName   string
	UTLSFingerprint string
}

type clientFactory struct {
	parent base.Transport
}

func (c *clientFactory) Transport() base.Transport {
	return c.parent
}
func (c *clientFactory) ParseArgs(args *pt.Args) (interface{}, error) {
	i, err := c.parseArgs(args)
	if err != nil {
		pt.Log(pt.LogSeverityError, fmt.Sprintf("Error parsing args: %v", err))
		return nil, err
	}
	return i, nil
}
func (c *clientFactory) parseArgs(args *pt.Args) (interface{}, error) {
	var config clientConfig

	if urlStr, ok := args.Get("url"); ok {
		url, err := url.Parse(urlStr)
		if err != nil {
			return nil, fmt.Errorf("url parse error: %s", err)
		}
		defaultPort := ""
		switch url.Scheme {
		case "https":
			config.TLSKind = "tls"
			defaultPort = "443"
		case "http":
			config.TLSKind = ""
			defaultPort = "80"
		default:
			return nil, fmt.Errorf("url parse error: unknown scheme")
		}
		config.Path = strings.TrimPrefix(url.EscapedPath(), "/")
		config.TLSServerName = url.Hostname()
		port := url.Port()
		if port == "" {
			port = defaultPort
		}
		config.RemoteAddress = url.Hostname() + ":" + port

		if remoteAddress, ok := args.Get("addr"); ok {
			config.RemoteAddress = remoteAddress
		}

		config.TLSServerName = url.Hostname()
	}

	if tlsServerName, ok := args.Get("servername"); ok {
		config.TLSServerName = tlsServerName
	}

	if uTLSFingerprint, ok := args.Get("utls"); ok {
		config.UTLSFingerprint = uTLSFingerprint
		if config.UTLSFingerprint == "" {
			config.UTLSFingerprint = "hellorandomizednoalpn"
		}

		if config.UTLSFingerprint == "none" {
			config.UTLSFingerprint = ""
		}
	}

	return config, nil
}

func (c *clientFactory) Dial(network, address string, dialFn base.DialFunc, args interface{}) (net.Conn, error) {
	conn, err := c.dial(network, address, dialFn, args)
	if err != nil {
		pt.Log(pt.LogSeverityError, fmt.Sprintf("Error dialing: %v", err))
		return nil, err
	}
	return conn, nil
}

func (c *clientFactory) dial(network, address string, dialFn base.DialFunc, args interface{}) (net.Conn, error) {
	config := args.(clientConfig)

	conn, err := dialFn("tcp", config.RemoteAddress)
	if err != nil {
		return nil, fmt.Errorf("error dialing %s: %v", config.RemoteAddress, err)
	}

	if config.TLSKind != "" {
		conf := &tls.Config{ServerName: config.TLSServerName}
		if config.UTLSFingerprint == "" {
			if tlsTransport, err := tls.NewTLSTransport(conf); err != nil {
				return nil, err
			} else {
				if tlsConn, err := tlsTransport.Client(conn); err != nil {
					return nil, err
				} else {
					conn = tlsConn
				}
			}
		} else {
			utlsConfig := &uTLSConfig{ServerName: config.TLSServerName, uTLSFingerprint: config.UTLSFingerprint}
			if utlsTransport, err := newUTLSTransport(utlsConfig); err != nil {
				return nil, err
			} else {
				if utlsConn, err := utlsTransport.Client(conn); err != nil {
					return nil, err
				} else {
					conn = utlsConn
				}
			}
		}
	}
	upgradeConfig := httpupgrade.Config{Path: config.Path, Host: config.TLSServerName}
	if httpupgradeTransport, err := httpupgrade.NewHTTPUpgradeTransport(&upgradeConfig); err != nil {
		return nil, err
	} else {
		if httpUpgradeConn, err := httpupgradeTransport.Client(conn); err != nil {
			return nil, err
		} else {
			conn = httpUpgradeConn
		}
	}
	return conn, nil
}

// Not yet implemented
func (cf *clientFactory) OnEvent(f func(base.TransportEvent)) {}
