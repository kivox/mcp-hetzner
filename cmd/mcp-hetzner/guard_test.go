package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGuardPublicListen(t *testing.T) {
	if err := guardPublicListen("127.0.0.1", ""); err != nil {
		t.Fatal(err)
	}
	if err := guardPublicListen("::1", ""); err != nil {
		t.Fatal(err)
	}
	if err := guardPublicListen("0.0.0.0", ""); err == nil {
		t.Fatal("a public listen without MCP_AUTH_TOKEN must be refused")
	}
	if err := guardPublicListen("0.0.0.0", "mcp-test"); err != nil {
		t.Fatal(err)
	}
}

func TestRequireBearer(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	handler := requireBearer("mcp-test", next)

	cases := []struct {
		name   string
		header string
		want   int
		called bool
	}{
		{"missing header", "", http.StatusUnauthorized, false},
		{"wrong token", "Bearer nope", http.StatusUnauthorized, false},
		{"prefix only", "Bearer ", http.StatusUnauthorized, false},
		{"wrong scheme", "Basic mcp-test", http.StatusUnauthorized, false},
		{"correct", "Bearer mcp-test", http.StatusNoContent, true},
	}
	for _, tc := range cases {
		called = false
		request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		if tc.header != "" {
			request.Header.Set("Authorization", tc.header)
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != tc.want {
			t.Fatalf("%s: status %d want %d", tc.name, recorder.Code, tc.want)
		}
		if called != tc.called {
			t.Fatalf("%s: next handler called=%v want %v", tc.name, called, tc.called)
		}
	}
}

func TestRequireBearerEmptyDisablesAuth(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	requireBearer("", next).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if recorder.Code != http.StatusNoContent || !called {
		t.Fatalf("loopback-only server should pass through: status=%d called=%v", recorder.Code, called)
	}
}
