package auth

import (
	"testing"

	envoy_auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
)

func Test_extractEndpointID(t *testing.T) {
	tests := []struct {
		name    string
		request *envoy_auth.AttributeContext_HttpRequest
		want    string
		wantErr bool
	}{
		{
			name: "should extract from header if present",
			request: &envoy_auth.AttributeContext_HttpRequest{
				Path: "/v1/shouldNotBeUsed",
				Headers: map[string]string{
					reqHeaderEndpointID: "headerID",
				},
			},
			want:    "headerID",
			wantErr: false,
		},
		{
			name: "should fall back to path if header missing",
			request: &envoy_auth.AttributeContext_HttpRequest{
				Path:    "/v1/pathID",
				Headers: map[string]string{},
			},
			want:    "pathID",
			wantErr: false,
		},
		{
			name: "should error if both header and path are missing",
			request: &envoy_auth.AttributeContext_HttpRequest{
				Path:    "/v1/",
				Headers: map[string]string{},
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "should error if neither valid header nor valid path",
			request: &envoy_auth.AttributeContext_HttpRequest{
				Path:    "/invalid/path",
				Headers: map[string]string{},
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := extractEndpointID(test.request)
			if (err != nil) != test.wantErr {
				t.Errorf("extractEndpointID() error = %v, wantErr %v", err, test.wantErr)
				return
			}
			if got != test.want {
				t.Errorf("extractEndpointID() = %v, want %v", got, test.want)
			}
		})
	}
}

func Test_extractFromPath(t *testing.T) {
	tests := []struct {
		name    string
		request *envoy_auth.AttributeContext_HttpRequest
		want    string
	}{
		{
			name: "should extract endpoint ID from valid path",
			request: &envoy_auth.AttributeContext_HttpRequest{
				Path: "/v1/1a2b3c4d",
			},
			want: "1a2b3c4d",
		},
		{
			name: "should return empty for path without endpoint ID",
			request: &envoy_auth.AttributeContext_HttpRequest{
				Path: "/v1/",
			},
			want: "",
		},
		{
			name: "should return empty for invalid path",
			request: &envoy_auth.AttributeContext_HttpRequest{
				Path: "/invalid/1a2b3c4d",
			},
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := extractFromPath(test.request)
			if got != test.want {
				t.Errorf("extractFromPath() = %v, want %v", got, test.want)
			}
		})
	}
}

func Test_extractFromHeader(t *testing.T) {
	tests := []struct {
		name    string
		request *envoy_auth.AttributeContext_HttpRequest
		want    string
	}{
		{
			name: "should extract endpoint ID from header",
			request: &envoy_auth.AttributeContext_HttpRequest{
				Headers: map[string]string{
					reqHeaderEndpointID: "1a2b3c4d",
				},
			},
			want: "1a2b3c4d",
		},
		{
			name: "should return empty when header is missing",
			request: &envoy_auth.AttributeContext_HttpRequest{
				Headers: map[string]string{},
			},
			want: "",
		},
		{
			name: "should return empty when header is empty",
			request: &envoy_auth.AttributeContext_HttpRequest{
				Headers: map[string]string{
					reqHeaderEndpointID: "",
				},
			},
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := extractFromHeader(test.request)
			if got != test.want {
				t.Errorf("extractFromHeader() = %v, want %v", got, test.want)
			}
		})
	}
}
