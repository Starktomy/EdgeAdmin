package rpc

import (
	"errors"
	"net/url"
	"os"

	"github.com/TeaOSLab/EdgeCommon/pkg/rpcauth"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// TransportCredentials applies the same verified transport policy to normal
// connections and discovery probes. ServerName is the configured endpoint name,
// even if a local connection is dialed through its loopback address.
func TransportCredentials(endpoint *url.URL) (credentials.TransportCredentials, error) {
	if endpoint == nil || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Fragment != "" {
		return nil, errors.New("invalid RPC endpoint")
	}
	switch endpoint.Scheme {
	case "https":
		config, err := rpcauth.VerifiedTLSConfig(endpoint.Hostname(), os.Getenv("EDGE_RPC_CA_FILE"))
		if err != nil {
			return nil, err
		}
		return credentials.NewTLS(config), nil
	case "http":
		if !rpcauth.PlaintextAllowed(endpoint.Host) {
			return nil, errors.New("plaintext RPC is disabled; use HTTPS or EDGE_ALLOW_LOOPBACK_RPC=true for literal loopback IPs")
		}
		return insecure.NewCredentials(), nil
	default:
		return nil, errors.New("RPC endpoints must use HTTPS")
	}
}
