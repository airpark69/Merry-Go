package handlers

import (
	"log"
	"net"
	"net/http"
	"strings"
)

// getIPAddress는 요청에서 클라이언트의 IP 주소를 추출합니다.
func getIPAddress(r *http.Request) string {
	// X-Forwarded-For 헤더 확인 (프록시 뒤에 있는 경우)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}

	// X-Real-IP 헤더 확인
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// 직접 연결된 클라이언트의 IP 주소
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// logRequest는 요청 정보를 로그에 기록합니다.
func LogRequest(r *http.Request) {
	ip := getIPAddress(r)
	log.Printf("Received request from %s: %s %s %s", ip, r.Method, r.URL.Path, r.Proto)
}
