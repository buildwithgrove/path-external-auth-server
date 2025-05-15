package auth

import (
	"net/http"
	"testing"

	"github.com/buildwithgrove/path-external-auth-server/store"
)

func Test_extractPortalAppID(t *testing.T) {
	tests := []struct {
		name    string
		headers http.Header
		path    string
		want    store.PortalAppID
		wantErr bool
	}{
		{
			name: "should extract from header if present",
			headers: convertMapToHeader(map[string]string{
				reqHeaderPortalAppID: "headerID",
			}),
			path:    "/v1/shouldNotBeUsed",
			want:    "headerID",
			wantErr: false,
		},
		{
			name:    "should fall back to path if header missing",
			headers: http.Header{},
			path:    "/v1/pathID",
			want:    "pathID",
			wantErr: false,
		},
		{
			name:    "should error if both header and path are missing",
			headers: http.Header{},
			path:    "/v1/",
			want:    "",
			wantErr: true,
		},
		{
			name:    "should error if neither valid header nor valid path",
			headers: http.Header{},
			path:    "/invalid/path",
			want:    "",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := extractPortalAppID(test.headers, test.path)
			if (err != nil) != test.wantErr {
				t.Errorf("extractPortalAppID() error = %v, wantErr %v", err, test.wantErr)
				return
			}
			if got != test.want {
				t.Errorf("extractPortalAppID() = %v, want %v", got, test.want)
			}
		})
	}
}

func Test_extractFromHeader(t *testing.T) {
	tests := []struct {
		name    string
		headers http.Header
		want    store.PortalAppID
	}{
		{
			name: "should extract portal app ID from header",
			headers: convertMapToHeader(map[string]string{
				reqHeaderPortalAppID: "1a2b3c4d",
			}),
			want: "1a2b3c4d",
		},
		{
			name:    "should return empty when header is missing",
			headers: http.Header{},
			want:    "",
		},
		{
			name: "should return empty when header is empty",
			headers: http.Header{
				reqHeaderPortalAppID: []string{""},
			},
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := extractFromHeader(test.headers)
			if got != test.want {
				t.Errorf("extractFromHeader() = %v, want %v", got, test.want)
			}
		})
	}
}

func Test_extractFromPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want store.PortalAppID
	}{
		{
			name: "should extract portal app ID from valid path",
			path: "/v1/1a2b3c4d",
			want: "1a2b3c4d",
		},
		{
			name: "should return empty for path without portal app ID",
			path: "/v1/",
			want: "",
		},
		{
			name: "should return empty for invalid path",
			path: "/invalid/1a2b3c4d",
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := extractFromPath(test.path)
			if got != test.want {
				t.Errorf("extractFromPath() = %v, want %v", got, test.want)
			}
		})
	}
}
