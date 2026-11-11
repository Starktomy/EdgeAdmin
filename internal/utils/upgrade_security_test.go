package utils

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/TeaOSLab/EdgeAdmin/internal/utils/upgradesecurity"
)

func TestUpgradeTransportRejectsUntrustedCertificateAndRedirects(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	location, _ := url.Parse(server.URL)
	trust := &upgradesecurity.Trust{AllowedHosts: []string{location.Host}}
	client := upgradeHTTPClient(trust)
	defer client.CloseIdleConnections()
	if response, err := client.Get(server.URL); err == nil {
		response.Body.Close()
		t.Fatal("accepted an untrusted TLS certificate")
	}
	for _, target := range []string{"http://" + location.Host + "/release.zip", "https://untrusted.example/release.zip"} {
		req, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.CheckRedirect(req, []*http.Request{{}}); err == nil {
			t.Fatal("accepted an untrusted upgrade redirect")
		}
	}
}

func TestUpgradeMetadataReadIsBounded(t *testing.T) {
	if _, err := readBounded(strings.NewReader("too long"), 3); err == nil {
		t.Fatal("oversized metadata accepted")
	}
	if data, err := readBounded(strings.NewReader("ok"), 3); err != nil || string(data) != "ok" {
		t.Fatal("valid metadata rejected", err)
	}
}
