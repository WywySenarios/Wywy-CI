package mcp

import (
	"net"
	"net/http"
	"os"
)

const (
	// DefaultMCPHost is the default MCP server bind address.
	DefaultMCPHost = "127.0.0.1"
	// DefaultMCPPort is the default MCP server port.
	DefaultMCPPort = "2527"

	// EnvMCPHost is the environment variable to configure the MCP server host.
	EnvMCPHost = "CI_MCP_HOST"
	// EnvMCPPort is the environment variable to configure the MCP server port.
	EnvMCPPort = "CI_MCP_PORT"
)

// Config holds the MCP server transport configuration.
type Config struct {
	Host string
	Port string
}

// ConfigFromEnv reads the MCP server configuration from environment variables,
// falling back to DefaultMCPHost and DefaultMCPPort when not set.
func ConfigFromEnv() Config {
	host := os.Getenv(EnvMCPHost)
	if host == "" {
		host = DefaultMCPHost
	}
	port := os.Getenv(EnvMCPPort)
	if port == "" {
		port = DefaultMCPPort
	}
	return Config{Host: host, Port: port}
}

// Addr returns the address string (host:port), handling IPv6 addresses correctly.
func (c Config) Addr() string {
	return net.JoinHostPort(c.Host, c.Port)
}

// NewTransport creates an HTTP server configured with the MCP handler
// and the address from CI_MCP_HOST / CI_MCP_PORT environment variables.
func NewTransport(handler http.Handler) *http.Server {
	cfg := ConfigFromEnv()
	return &http.Server{
		Addr:    cfg.Addr(),
		Handler: handler,
	}
}
