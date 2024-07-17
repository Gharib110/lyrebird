package webtunnel

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"

	pt "gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/goptlib"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/webtunnel/transport/httpupgrade"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/webtunnel/transport/tls"

	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/transports/base"
)

type clientConfig struct {
	RemoteAddresses []string

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

		config.RemoteAddresses, err = getAddressesFromHostname(url.Hostname(), port)
		if err != nil {
			log.Println(err)
			return nil, errors.New("")
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
	var conn net.Conn
	for _, addr := range config.RemoteAddresses {
		if tcpConn, err := dialFn("tcp", addr); err == nil {
			conn = tcpConn
		}
	}
	if conn == nil {
		return nil, fmt.Errorf("Can't connect to %v", config.RemoteAddresses)
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

func getAddressesFromHostname(hostname, port string) ([]string, error) {
	addresses := []string{}
	addr, err := net.LookupHost(hostname)
	if err != nil {
		return addresses, fmt.Errorf("Lookup error for host %s: %v", hostname, err)
	}

	for _, a := range addr {
		ip := net.ParseIP(a)
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsPrivate() {
			continue
		}
		addresses = append(addresses, a+":"+port)
	}
	if len(addresses) == 0 {
		return addresses, fmt.Errorf("Could not find any valid IP for %s", hostname)
	}
	return addresses, nil
}
