// 文件说明：验证 HTTP 认证输入边界先于依赖和慢哈希，不把处理器单测称为真实数据库验证。
package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPasswordPolicyAtBothHTTPRoutes(t *testing.T) {
	for _, path := range []string{"/register", "/login"} {
		for _, password := range []string{"12345", strings.Repeat("a", 33), "abc中文def", "abc😀def", "abc def", "abc\x7fdef", "abc\ndef"} {
			data, _ := json.Marshal(map[string]string{"account": "demo", "password": password})
			req := httptest.NewRequest("POST", path, strings.NewReader(string(data)))
			req.Header.Set("Content-Type", "application/json")
			out := httptest.NewRecorder()
			(&LoginHTTP{}).routes().ServeHTTP(out, req)
			if out.Code != 400 || !strings.Contains(out.Body.String(), "invalid_password") {
				t.Fatal("invalid password passed HTTP input boundary", path, out.Code)
			}
		}
	}
}

func TestAuthenticationInputBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, method, contentType, origin, body string
		want                                    int
	}{
		{"method", "GET", "application/json", "", "{}", 405},
		{"missing_password", "POST", "application/json", "", `{"account":"demo"}`, 400},
		{"unknown_field", "POST", "application/json", "", `{"account":"demo","password":"valid-password-93","extra":1}`, 400},
		{"trailing_json", "POST", "application/json", "", `{"account":"demo","password":"valid-password-93"}{}`, 400},
		{"large", "POST", "application/json", "", strings.Repeat("x", 2100), 400},
		{"text", "POST", "text/plain", "", `{"account":"demo","password":"valid-password-93"}`, 415},
		{"cross_origin", "POST", "application/json", "https://elsewhere.invalid", `{"account":"demo","password":"valid-password-93"}`, 403},
		{"ready_boundary", "POST", "application/json", "", `{"account":"demo","password":"valid-password-93"}`, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://127.0.0.1/login", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			req.Header.Set("Origin", tc.origin)
			out := httptest.NewRecorder()
			(&LoginHTTP{}).routes().ServeHTTP(out, req)
			if out.Code != tc.want {
				t.Fatal(out.Code, out.Body.String())
			}
			if out.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("credential response cacheable")
			}
		})
	}
}
