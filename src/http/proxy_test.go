package http

import (
	"github.com/smartping/smartping/src/g"
	"testing"
)

func TestValidateProxyURL(t *testing.T) {
	// Setup mock config for test
	g.Cfg.Addr = "10.0.0.1"
	g.Cfg.Port = 8899
	g.AuthAgentIpMap = map[string]bool{
		"10.0.0.2":    true,
		"agent.local": true,
	}

	tests := []struct {
		name      string
		rawURL    string
		expectErr bool
	}{
		{
			name:      "Valid target in agent map",
			rawURL:    "http://10.0.0.2:8899/api/config.json",
			expectErr: false,
		},
		{
			name:      "Valid target hostname in agent map",
			rawURL:    "http://agent.local:8899/api/ping.json?ip=1.1.1.1",
			expectErr: false,
		},
		{
			name:      "Valid self address",
			rawURL:    "http://10.0.0.1:8899/api/tcpping.json",
			expectErr: false,
		},
		{
			name:      "Valid localhost with matching port",
			rawURL:    "http://127.0.0.1:8899/api/topology.json",
			expectErr: false,
		},
		{
			name:      "SSRF - Cloud metadata endpoint blocked",
			rawURL:    "http://169.254.169.254/latest/meta-data/",
			expectErr: true,
		},
		{
			name:      "SSRF - Cloud metadata even if path was api endpoint",
			rawURL:    "http://169.254.169.254/api/config.json",
			expectErr: true,
		},
		{
			name:      "SSRF - Disallowed file protocol",
			rawURL:    "file:///etc/passwd",
			expectErr: true,
		},
		{
			name:      "SSRF - Disallowed gopher protocol",
			rawURL:    "gopher://127.0.0.1:6379/_flushall",
			expectErr: true,
		},
		{
			name:      "SSRF - Unapproved internal IP blocked",
			rawURL:    "http://192.168.1.1/api/config.json",
			expectErr: true,
		},
		{
			name:      "SSRF - Localhost on unapproved internal port blocked",
			rawURL:    "http://127.0.0.1:6379/api/config.json",
			expectErr: true,
		},
		{
			name:      "SSRF - Unauthorized API path blocked",
			rawURL:    "http://10.0.0.2:8899/admin/shell",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateProxyURL(tt.rawURL)
			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateProxyURL(%q) err = %v, expectErr = %v", tt.rawURL, err, tt.expectErr)
			}
		})
	}
}
