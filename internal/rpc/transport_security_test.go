package rpc

import (
	"context"
	"net/url"
	"testing"

	"github.com/TeaOSLab/EdgeAdmin/internal/configs"
	"github.com/TeaOSLab/EdgeCommon/pkg/rpcauth"
	"google.golang.org/grpc/metadata"
)

func TestRPCTransportRequiresVerifiedTLSOrExplicitMigration(t *testing.T) {
	t.Setenv("EDGE_RPC_CA_FILE", "")
	t.Setenv("EDGE_ALLOW_PLAINTEXT_RPC", "")
	t.Setenv("EDGE_ALLOW_LOOPBACK_RPC", "")
	https, _ := url.Parse("https://api.example:8002")
	creds, err := TransportCredentials(https)
	if err != nil {
		t.Fatal(err)
	}
	if creds.Info().SecurityProtocol != "tls" || creds.Info().ServerName != "api.example" {
		t.Fatalf("unexpected TLS peer policy: %+v", creds.Info())
	}
	http, _ := url.Parse("http://127.0.0.1:8002")
	if _, err := TransportCredentials(http); err == nil {
		t.Fatal("plaintext RPC silently accepted")
	}
	t.Setenv("EDGE_ALLOW_PLAINTEXT_RPC", "true")
	if _, err := TransportCredentials(http); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDGE_RPC_CA_FILE", "/definitely/missing/edge-rpc-ca.pem")
	if _, err := TransportCredentials(https); err == nil {
		t.Fatal("bad CA silently accepted")
	}
}

func TestRPCTransportLoopbackModeRejectsExternalHTTPAndDNS(t *testing.T) {
	t.Setenv("EDGE_ALLOW_PLAINTEXT_RPC", "")
	t.Setenv("EDGE_ALLOW_LOOPBACK_RPC", "true")
	for _, raw := range []string{"http://127.0.0.1:8002", "http://127.2.3.4:8002", "http://[::1]:8002"} {
		endpoint, _ := url.Parse(raw)
		if _, err := TransportCredentials(endpoint); err != nil {
			t.Errorf("literal loopback rejected: %s: %v", raw, err)
		}
	}
	for _, raw := range []string{"http://localhost:8002", "http://api.example:8002", "http://192.0.2.1:8002", "http://127.0.0.1.attacker:8002", "http://2130706433:8002"} {
		endpoint, _ := url.Parse(raw)
		if _, err := TransportCredentials(endpoint); err == nil {
			t.Errorf("non-loopback plaintext accepted: %s", raw)
		}
	}
}

func TestAdminContextsUseBoundSignedClaims(t *testing.T) {
	client := &RPCClient{apiConfig: &configs.APIConfig{NodeId: "admin-node", Secret: "0123456789abcdef0123456789abcdef"}}
	for role, ctx := range map[string]context.Context{"admin": client.Context(42), "api": client.APIContext(0)} {
		md, _ := metadata.FromOutgoingContext(ctx)
		token := md.Get("token")[0]
		claims, err := rpcauth.Verify(token, client.apiConfig.Secret, client.apiConfig.NodeId)
		if err != nil {
			t.Fatal(err)
		}
		if claims.Type != role || claims.NodeID != client.apiConfig.NodeId || (role == "admin" && claims.UserID != 42) {
			t.Fatalf("signed wrong role: %+v", claims)
		}
	}
}
