package middleware

import (
	_ "context"
	_ "fmt"
	"net"
	"net/http"
	"strings"

	"github.com/Evgen-Mutagen/go-shortener-url/internal/configs"
)

// TrustedSubnetMiddleware проверяет, что IP-адрес клиента входит в доверенную подсеть
func TrustedSubnetMiddleware(cfg *configs.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if cfg.TrustedSubnet == "" {
				http.Error(w, "Access denied", http.StatusForbidden)
				return
			}

			clientIP := r.Header.Get("X-Real-IP")
			if clientIP == "" {
				clientIP = getClientIP(r)
			}

			if !isIPInSubnet(clientIP, cfg.TrustedSubnet) {
				http.Error(w, "Access denied", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP извлекает IP-адрес клиента из запроса
func getClientIP(r *http.Request) string {

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// isIPInSubnet проверяет, входит ли IP-адрес в указанную подсеть
func isIPInSubnet(clientIP, subnet string) bool {
	_, network, err := net.ParseCIDR(subnet)
	if err != nil {
		ip := net.ParseIP(subnet)
		if ip == nil {
			return false
		}
		if ip.To4() != nil {
			_, network, err = net.ParseCIDR(subnet + "/32")
		} else {
			_, network, err = net.ParseCIDR(subnet + "/128")
		}
		if err != nil {
			return false
		}
	}

	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	// Проверяем, входит ли IP в подсеть
	return network.Contains(ip)
}
