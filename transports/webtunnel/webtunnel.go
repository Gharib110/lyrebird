package webtunnel

import (
	"errors"

	pt "gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/goptlib"

	"gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird/transports/base"
)

const (
	ptName = "webtunnel"
)

var (
	unimplementedFeature = errors.New("unimplemented")
)

type transport struct {
}

func (t *transport) Name() string {
	return ptName
}

func (t *transport) ClientFactory(stateDir string) (base.ClientFactory, error) {
	return &clientFactory{parent: t}, nil
}

func (t *transport) ServerFactory(stateDir string, args *pt.Args) (base.ServerFactory, error) {
	return nil, unimplementedFeature
}

var Transport base.Transport = (*transport)(nil)
