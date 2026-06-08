package service

import "sync"

type DNSProvider interface {
	Name() string
	TLSDirective(domain string) (string, error)
}

var (
	dnsProviderMu sync.RWMutex
	dnsProviders  = map[string]DNSProvider{}
)

func RegisterDNSProvider(p DNSProvider) {
	if p == nil {
		return
	}
	dnsProviderMu.Lock()
	defer dnsProviderMu.Unlock()
	dnsProviders[p.Name()] = p
}

func getDNSProvider(name string) DNSProvider {
	dnsProviderMu.RLock()
	defer dnsProviderMu.RUnlock()
	return dnsProviders[name]
}
