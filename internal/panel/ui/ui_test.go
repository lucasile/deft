package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestHandlerServesExtensionlessStaticRoutes(t *testing.T) {
	publicFS := fstest.MapFS{
		"index.html":                      &fstest.MapFile{Data: []byte("index")},
		"login.html":                      &fstest.MapFile{Data: []byte("login")},
		"register.html":                   &fstest.MapFile{Data: []byte("register")},
		"app.js":                          &fstest.MapFile{Data: []byte("js")},
		"_app/immutable/chunks/_chunk.js": &fstest.MapFile{Data: []byte("chunk")},
	}
	handler := Handler(publicFS)

	tests := []struct {
		path string
		want string
		code int
	}{
		{path: "/", want: "index", code: http.StatusOK},
		{path: "/commands", want: "index", code: http.StatusOK},
		{path: "/login", want: "login", code: http.StatusOK},
		{path: "/register", want: "register", code: http.StatusOK},
		{path: "/nodes/node_123", want: "index", code: http.StatusOK},
		{path: "/nodes/node_123/containers/container_456", want: "index", code: http.StatusOK},
		{path: "/nodes/join/request-id", want: "index", code: http.StatusOK},
		{path: "/_app/immutable/chunks/_chunk.js", want: "chunk", code: http.StatusOK},
		{path: "/missing.js", code: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.code {
				t.Fatalf("status = %d, want %d", rec.Code, tt.code)
			}
			if tt.want != "" && rec.Body.String() != tt.want {
				t.Fatalf("body = %q, want %q", rec.Body.String(), tt.want)
			}
		})
	}
}
