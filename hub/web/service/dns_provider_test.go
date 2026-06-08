package service

import "testing"

type testDNSProvider struct{}

func (testDNSProvider) Name() string { return "testdns" }

func (testDNSProvider) TLSDirective(domain string) (string, error) {
	return "dns testdns", nil
}

func TestDNSProviderRegistry(t *testing.T) {
	RegisterDNSProvider(testDNSProvider{})
	got := getDNSProvider("testdns")
	if got == nil {
		t.Fatal("provider not registered")
	}
	directive, err := bootstrapTLSDirective("testdns", "node.example.com")
	if err != nil {
		t.Fatalf("bootstrapTLSDirective returned error: %v", err)
	}
	if directive != "dns testdns" {
		t.Fatalf("directive = %q, want %q", directive, "dns testdns")
	}
}
