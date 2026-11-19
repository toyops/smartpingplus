package http

import (
	"encoding/json"
	"fmt"
	"github.com/smartping/smartping/src/g"
	"github.com/wcharczuk/go-chart"
	"github.com/wcharczuk/go-chart/drawing"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var allowedProxyPaths = map[string]bool{
	"/api/config.json":   true,
	"/api/ping.json":     true,
	"/api/tcpping.json":  true,
	"/api/topology.json": true,
	"/api/mapping.json":  true,
	"/api/alert.json":    true,
	"/api/tools.json":    true,
}

func isAuthorizedProxyTarget(host, port string) bool {
	if host == "" {
		return false
	}
	// Self
	if host == g.Cfg.Addr {
		return true
	}
	cfgPortStr := strconv.Itoa(g.Cfg.Port)
	if (host == "127.0.0.1" || host == "localhost") && (port == cfgPortStr || port == "") {
		return true
	}
	// Cloud endpoint
	if ep := g.Cfg.Mode["Endpoint"]; ep != "" {
		if host == ep || strings.HasPrefix(ep, host) {
			return true
		}
	}
	// Configured agent map
	g.AuthLock.RLock()
	if g.AuthAgentIpMap != nil && g.AuthAgentIpMap[host] {
		g.AuthLock.RUnlock()
		return true
	}
	g.AuthLock.RUnlock()
	// Check Chinamap
	if g.Cfg.Chinamap != nil {
		for _, prov := range g.Cfg.Chinamap {
			for _, ips := range prov {
				for _, ip := range ips {
					if ip == host {
						return true
					}
				}
			}
		}
	}
	return false
}

// ValidateProxyURL validates proxy targets to prevent SSRF and internal network scanning.
func ValidateProxyURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}
	if !allowedProxyPaths[u.Path] {
		return nil, fmt.Errorf("unauthorized API path: %s", u.Path)
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("host cannot be empty")
	}
	port := u.Port()

	// Check if target is in the configured agent / self list
	if !isAuthorizedProxyTarget(host, port) {
		return nil, fmt.Errorf("proxy target %s is not in configured agent list", host)
	}

	// Resolve IPs to check for link-local or metadata addresses (e.g. 169.254.x.x)
	parsedIP := net.ParseIP(host)
	var ips []net.IP
	if parsedIP != nil {
		ips = []net.IP{parsedIP}
	} else {
		resolved, err := net.LookupIP(host)
		if err == nil {
			ips = resolved
		}
	}
	for _, ip := range ips {
		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.String() == "169.254.169.254" {
			return nil, fmt.Errorf("access to link-local or metadata address is prohibited")
		}
	}
	return u, nil
}

// SafeProxyClient returns an http.Client configured to prevent SSRF redirect bypasses.
func SafeProxyClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func ValidIP4(ipAddress string) bool {
	ipAddress = strings.Trim(ipAddress, " ")
	re, _ := regexp.Compile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
	if re.MatchString(ipAddress) {
		return true
	}
	return false
}

// ValidHost 校验节点地址：IPv4 或合法域名（供 Network 条目使用，
// 使 TcpPing 的域名目标也能挂名称）。
func ValidHost(host string) bool {
	if ValidIP4(host) {
		return true
	}
	host = strings.Trim(host, " ")
	re, _ := regexp.Compile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)
	return re.MatchString(host)
}

func RenderJson(w http.ResponseWriter, v interface{}) {
	bs, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Write(bs)
}

func AuthUserIp(RemoteAddr string) bool {
	g.AuthLock.RLock()
	defer g.AuthLock.RUnlock()
	if len(g.AuthUserIpMap) == 0 {
		return true
	}
	ips := strings.Split(RemoteAddr, ":")
	if len(ips) == 2 {
		if _, ok := g.AuthUserIpMap[ips[0]]; ok {
			return true
		}
	}
	return false
}

func AuthAgentIp(RemoteAddr string, drt bool) bool {
	g.AuthLock.RLock()
	defer g.AuthLock.RUnlock()
	if drt {
		if len(g.AuthUserIpMap) == 0 {
			return true
		}
	}
	if len(g.AuthAgentIpMap) == 0 {
		return true
	}
	ips := strings.Split(RemoteAddr, ":")
	if len(ips) == 2 {
		if _, ok := g.AuthAgentIpMap[ips[0]]; ok {
			return true
		}
	}
	return false
}

func GraphText(x int, y int, txt string) chart.Renderer {
	f, _ := chart.GetDefaultFont()
	rhart, _ := chart.PNG(300, 130)
	chart.Draw.Text(rhart, txt, x, y, chart.Style{
		FontColor: drawing.ColorBlack,
		FontSize:  10,
		Font:      f,
	})
	return rhart
}

// securityHeadersMiddleware applies standard security headers and handles CORS preflight.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Handle CORS preflight for API requests
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

var routesOnce sync.Once

// InitHttpServer configures routes, security headers and timeouts, returning an http.Server instance.
func InitHttpServer() *http.Server {
	routesOnce.Do(func() {
		configApiRoutes()
		configIndexRoutes()
	})
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", g.Cfg.Port),
		Handler:           securityHeadersMiddleware(http.DefaultServeMux),
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}
}
