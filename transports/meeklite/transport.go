/*
 * Copyright (c) 2019 Yawning Angel <yawning at schwanenlied dot me>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package meeklite

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	utls "github.com/refraction-networking/utls"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/transports/base"
	"golang.org/x/net/http2"
)

var (
	errProtocolNegotiated = errors.New("meek_lite: protocol negotiated")

	// This should be kept in sync with what is available in utls.
	clientHelloIDMap = map[string]*utls.ClientHelloID{
		"hellogolang":           nil, // Don't bother with utls.
		"hellorandomized":       &utls.HelloRandomized,
		"hellorandomizedalpn":   &utls.HelloRandomizedALPN,
		"hellorandomizednoalpn": &utls.HelloRandomizedNoALPN,
		"hellofirefox_auto":     &utls.HelloFirefox_Auto,
		"hellofirefox_55":       &utls.HelloFirefox_55,
		"hellofirefox_56":       &utls.HelloFirefox_56,
		"hellofirefox_63":       &utls.HelloFirefox_63,
		"hellofirefox_65":       &utls.HelloFirefox_65,
		"hellofirefox_99":       &utls.HelloFirefox_99,
		"hellofirefox_102":      &utls.HelloFirefox_102,
		"hellofirefox_105":      &utls.HelloFirefox_105,
		"hellochrome_auto":      &utls.HelloChrome_Auto,
		"hellochrome_58":        &utls.HelloChrome_58,
		"hellochrome_62":        &utls.HelloChrome_62,
		"hellochrome_70":        &utls.HelloChrome_70,
		"hellochrome_72":        &utls.HelloChrome_72,
		"hellochrome_83":        &utls.HelloChrome_83,
		"hellochrome_87":        &utls.HelloChrome_87,
		"hellochrome_96":        &utls.HelloChrome_96,
		"hellochrome_100":       &utls.HelloChrome_100,
		"hellochrome_102":       &utls.HelloChrome_102,
		"helloios_auto":         &utls.HelloIOS_Auto,
		"helloios_11_1":         &utls.HelloIOS_11_1,
		"helloios_12_1":         &utls.HelloIOS_12_1,
		"helloios_13":           &utls.HelloIOS_13,
		"helloios_14":           &utls.HelloIOS_14,
		"helloandroid_11":       &utls.HelloAndroid_11_OkHttp,
		"helloedge_auto":        &utls.HelloEdge_Auto,
		"helloedge_85":          &utls.HelloEdge_85,
		"helloedge_106":         &utls.HelloEdge_106,
		"hellosafari_auto":      &utls.HelloSafari_Auto,
		"hellosafari_16_0":      &utls.HelloSafari_16_0,
		"hello360_auto":         &utls.Hello360_Auto,
		"hello360_7_5":          &utls.Hello360_7_5,
		"hello360_11_0":         &utls.Hello360_11_0,
		"helloqq_auto":          &utls.HelloQQ_Auto,
		"helloqq_11_1":          &utls.HelloQQ_11_1,
	}
	defaultClientHello = &utls.HelloChrome_Auto
)

type roundTripper struct {
	sync.Mutex

	clientHelloID *utls.ClientHelloID
	dialFn        base.DialFunc
	transport     http.RoundTripper

	initConn net.Conn
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Note: This isn't protected with a lock, since the meeklite ioWorker
	// serializes RoundTripper requests.
	//
	// This also assumes that req.URL.Host will remain constant for the
	// lifetime of the roundTripper, which is a valid assumption for meeklite.
	if rt.transport == nil {
		if err := rt.getTransport(req); err != nil {
			return nil, err
		}
	}
	return rt.transport.RoundTrip(req)
}

func (rt *roundTripper) getTransport(req *http.Request) error {
	switch strings.ToLower(req.URL.Scheme) {
	case "http":
		rt.transport = newHTTPTransport(rt.dialFn, nil)
		return nil
	case "https":
	default:
		return fmt.Errorf("meek_lite: invalid URL scheme: '%v'", req.URL.Scheme)
	}

	_, err := rt.dialTLS("tcp", getDialTLSAddr(req.URL))
	switch err {
	case errProtocolNegotiated:
	case nil:
		// Should never happen.
		panic("meek_lite: dialTLS returned no error when determining transport")
	default:
		return err
	}

	return nil
}

func (rt *roundTripper) dialTLS(network, addr string) (net.Conn, error) {
	// Unlike rt.transport, this is protected by a critical section
	// since past the initial manual call from getTransport, the HTTP
	// client will be the caller.
	rt.Lock()
	defer rt.Unlock()

	// If we have the connection from when we determined the HTTPS
	// transport to use, return that.
	if conn := rt.initConn; conn != nil {
		rt.initConn = nil
		return conn, nil
	}

	rawConn, err := rt.dialFn(network, addr)
	if err != nil {
		return nil, err
	}

	var host string
	if host, _, err = net.SplitHostPort(addr); err != nil {
		host = addr
	}

	conn := utls.UClient(rawConn, &utls.Config{
		ServerName: host,

		// `crypto/tls` gradually ramps up the record size.  While this is
		// a good optimization and is a relatively common server feature,
		// neither Firefox nor Chromium appear to use such optimizations.
		DynamicRecordSizingDisabled: true,
	}, *rt.clientHelloID)

	if err = conn.Handshake(); err != nil {
		conn.Close()
		return nil, err
	}

	if rt.transport != nil {
		return conn, nil
	}

	// No http.Transport constructed yet, create one based on the results
	// of ALPN.
	switch conn.ConnectionState().NegotiatedProtocol {
	case http2.NextProtoTLS:
		// The remote peer is speaking HTTP 2 + TLS.
		rt.transport = &http2.Transport{DialTLS: rt.dialTLSHTTP2}
	default:
		// Assume the remote peer is speaking HTTP 1.x + TLS.
		rt.transport = newHTTPTransport(nil, rt.dialTLS)
	}

	// Stash the connection just established for use servicing the
	// actual request (should be near-immediate).
	rt.initConn = conn

	return nil, errProtocolNegotiated
}

func (rt *roundTripper) dialTLSHTTP2(network, addr string, cfg *tls.Config) (net.Conn, error) {
	return rt.dialTLS(network, addr)
}

func getDialTLSAddr(u *url.URL) string {
	host, port, err := net.SplitHostPort(u.Host)
	if err == nil {
		return net.JoinHostPort(host, port)
	}
	pInt, _ := net.LookupPort("tcp", u.Scheme)

	return net.JoinHostPort(u.Host, strconv.Itoa(pInt))
}

func newRoundTripper(dialFn base.DialFunc, clientHelloID *utls.ClientHelloID) http.RoundTripper {
	return &roundTripper{
		clientHelloID: clientHelloID,
		dialFn:        dialFn,
	}
}

func parseClientHelloID(s string) (*utls.ClientHelloID, error) {
	s = strings.ToLower(s)
	switch s {
	case "none":
		return nil, nil
	case "":
		return defaultClientHello, nil
	default:
		if ret := clientHelloIDMap[s]; ret != nil {
			return ret, nil
		}
	}
	return nil, fmt.Errorf("invalid ClientHelloID: '%v'", s)
}

func newHTTPTransport(dialFn, dialTLSFn base.DialFunc) *http.Transport {
	base := (http.DefaultTransport).(*http.Transport)

	return &http.Transport{
		Dial:    dialFn,
		DialTLS: dialTLSFn,

		// Use default configuration values, taken from the runtime.
		MaxIdleConns:          base.MaxIdleConns,
		IdleConnTimeout:       base.IdleConnTimeout,
		TLSHandshakeTimeout:   base.TLSHandshakeTimeout,
		ExpectContinueTimeout: base.ExpectContinueTimeout,
	}
}

func init() {
	// Attempt to increase compatibility, there's an encrypted link
	// underneath, and this doesn't (shouldn't) affect the external
	// fingerprint.
	utls.EnableWeakCiphers()
}
