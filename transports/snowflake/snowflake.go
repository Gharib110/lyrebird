package snowflake

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	pt "gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/goptlib"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/transports/base"
	sf "gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/v2/client/lib"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/v2/common/event"
	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/v2/common/proxy"
)

const transportName = "snowflake"

type sfEventLogger struct {
	onEventCallback func(e base.TransportEvent)
}

func (el *sfEventLogger) OnNewSnowflakeEvent(e event.SnowflakeEvent) {
	if el.onEventCallback != nil {
		el.onEventCallback(e)
	}
}

type Transport struct{}

// Name returns the name of the snowflake transport protocol.
func (t *Transport) Name() string {
	return transportName
}

// ClientFactory returns a new snowflakeClientFactory instance.
func (t *Transport) ClientFactory(stateDir string) (base.ClientFactory, error) {
	cf := &snowflakeClientFactory{transport: t}
	cf.eventLogger = &sfEventLogger{}
	return cf, nil
}

// ServerFactory is not implemented for snowflake
func (t *Transport) ServerFactory(stateDir string, args *pt.Args) (base.ServerFactory, error) {
	return nil, errors.New("ServerFactory not implemented for the snowflake transport")
}

type snowflakeClientFactory struct {
	transport   base.Transport
	eventLogger *sfEventLogger
}

func (cf *snowflakeClientFactory) Transport() base.Transport {
	return cf.transport
}

func (cf *snowflakeClientFactory) ParseArgs(args *pt.Args) (interface{}, error) {
	config := sf.ClientConfig{}
	if arg, ok := args.Get("ampcache"); ok {
		config.AmpCacheURL = arg
	}
	if arg, ok := args.Get("sqsqueue"); ok {
		config.SQSQueueURL = arg
	}
	if arg, ok := args.Get("sqscreds"); ok {
		config.SQSCredsStr = arg
	}
	if arg, ok := args.Get("fronts"); ok {
		if arg != "" {
			config.FrontDomains = strings.Split(strings.TrimSpace(arg), ",")
		}
	} else if arg, ok := args.Get("front"); ok {
		config.FrontDomains = strings.Split(strings.TrimSpace(arg), ",")
	}
	if arg, ok := args.Get("ice"); ok {
		config.ICEAddresses = strings.Split(strings.TrimSpace(arg), ",")
	}
	if arg, ok := args.Get("max"); ok {
		max, err := strconv.Atoi(arg)
		if err != nil {
			return nil, fmt.Errorf("Invalid SOCKS arg: max=%s", arg)
		}
		config.Max = max
	}
	if arg, ok := args.Get("url"); ok {
		config.BrokerURL = arg
	}
	if arg, ok := args.Get("utls-nosni"); ok {
		switch strings.ToLower(arg) {
		case "true", "yes":
			config.UTLSRemoveSNI = true
		}
	}
	if arg, ok := args.Get("utls-imitate"); ok {
		config.UTLSClientID = arg
	}
	if arg, ok := args.Get("fingerprint"); ok {
		config.BridgeFingerprint = arg
	}
	if arg, ok := args.Get("proxy"); ok {
		outboundProxy, err := url.Parse(arg)
		if err != nil {
			return nil, fmt.Errorf("Invalid SOCKS arg: proxy=%s", arg)
		}
		if err := proxy.CheckProxyProtocolSupport(outboundProxy); err != nil {
			return nil, fmt.Errorf("proxy is not supported: %s", err.Error())
		}
		client := proxy.NewSocks5UDPClient(outboundProxy)
		conn, err := client.ListenPacket("udp", nil)
		if err != nil {
			return nil, fmt.Errorf("proxy test failure: %s", err.Error())
		}
		conn.Close()
		config.CommunicationProxy = outboundProxy
	}
	return config, nil
}

func (cf *snowflakeClientFactory) OnEvent(f func(e base.TransportEvent)) {
	cf.eventLogger.onEventCallback = f
}

func (cf *snowflakeClientFactory) Dial(network, address string, dialFn base.DialFunc, args interface{}) (net.Conn, error) {
	config, ok := args.(sf.ClientConfig)
	if !ok {
		return nil, errors.New("invalid type for args")
	}
	transport, err := sf.NewSnowflakeClient(config)
	if err != nil {
		return nil, err
	}
	transport.AddSnowflakeEventListener(cf.eventLogger)
	return transport.Dial()
}
