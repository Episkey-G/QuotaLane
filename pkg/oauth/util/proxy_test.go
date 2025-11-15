package util

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateHTTPClient(t *testing.T) {
	tests := []struct {
		name      string
		proxyURL  string
		timeout   time.Duration
		wantErr   bool
		checkFunc func(t *testing.T, client *http.Client)
	}{
		{
			name:     "No proxy - direct connection",
			proxyURL: "",
			timeout:  10 * time.Second,
			wantErr:  false,
			checkFunc: func(t *testing.T, client *http.Client) {
				assert.NotNil(t, client)
				assert.Equal(t, 10*time.Second, client.Timeout)
				assert.Nil(t, client.Transport, "Transport should be nil for direct connection")
			},
		},
		{
			name:     "HTTP proxy",
			proxyURL: "http://proxy.example.com:8080",
			timeout:  5 * time.Second,
			wantErr:  false,
			checkFunc: func(t *testing.T, client *http.Client) {
				assert.NotNil(t, client)
				assert.Equal(t, 5*time.Second, client.Timeout)
				assert.NotNil(t, client.Transport, "Transport should be set for proxy")
			},
		},
		{
			name:     "HTTPS proxy",
			proxyURL: "https://proxy.example.com:8443",
			timeout:  3 * time.Second,
			wantErr:  false,
			checkFunc: func(t *testing.T, client *http.Client) {
				assert.NotNil(t, client)
				assert.NotNil(t, client.Transport)
			},
		},
		{
			name:     "HTTP proxy with authentication",
			proxyURL: "http://user:pass@proxy.example.com:8080",
			timeout:  10 * time.Second,
			wantErr:  false,
			checkFunc: func(t *testing.T, client *http.Client) {
				assert.NotNil(t, client)
				assert.NotNil(t, client.Transport)
			},
		},
		{
			name:     "SOCKS5 proxy",
			proxyURL: "socks5://localhost:1080",
			timeout:  10 * time.Second,
			wantErr:  false,
			checkFunc: func(t *testing.T, client *http.Client) {
				assert.NotNil(t, client)
				assert.NotNil(t, client.Transport)
			},
		},
		{
			name:     "SOCKS5 proxy with authentication",
			proxyURL: "socks5://user:pass@localhost:1080",
			timeout:  10 * time.Second,
			wantErr:  false,
			checkFunc: func(t *testing.T, client *http.Client) {
				assert.NotNil(t, client)
				assert.NotNil(t, client.Transport)
			},
		},
		{
			name:     "Invalid proxy URL",
			proxyURL: "://invalid",
			timeout:  10 * time.Second,
			wantErr:  true,
		},
		{
			name:     "Unsupported proxy scheme",
			proxyURL: "ftp://proxy.example.com:21",
			timeout:  10 * time.Second,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := CreateHTTPClient(tt.proxyURL, tt.timeout)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)

			if tt.checkFunc != nil {
				tt.checkFunc(t, client)
			}
		})
	}
}

func TestHTTPClientWithProxy_Integration(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	t.Run("Direct connection without proxy", func(t *testing.T) {
		client, err := CreateHTTPClient("", 5*time.Second)
		require.NoError(t, err)

		resp, err := client.Get(server.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// Note: Testing actual proxy connections requires a running proxy server
	// These are skipped in unit tests but can be enabled for integration tests
	t.Run("SOCKS5 proxy connection - skipped", func(t *testing.T) {
		t.Skip("Requires running SOCKS5 proxy server")
		// client, err := CreateHTTPClient("socks5://localhost:1080", 5*time.Second)
		// require.NoError(t, err)
		// resp, err := client.Get(server.URL)
		// require.NoError(t, err)
		// defer resp.Body.Close()
	})
}

func TestCreateHTTPClient_Timeout(t *testing.T) {
	// Create a slow server that delays response
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	t.Run("Request timeout", func(t *testing.T) {
		client, err := CreateHTTPClient("", 500*time.Millisecond)
		require.NoError(t, err)

		_, err = client.Get(slowServer.URL)
		assert.Error(t, err, "should timeout")
	})

	t.Run("Request success within timeout", func(t *testing.T) {
		client, err := CreateHTTPClient("", 5*time.Second)
		require.NoError(t, err)

		resp, err := client.Get(slowServer.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// Benchmark tests
func BenchmarkCreateHTTPClient_NoProxy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = CreateHTTPClient("", 10*time.Second)
	}
}

func BenchmarkCreateHTTPClient_HTTPProxy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = CreateHTTPClient("http://proxy.example.com:8080", 10*time.Second)
	}
}

func BenchmarkCreateHTTPClient_SOCKS5Proxy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = CreateHTTPClient("socks5://localhost:1080", 10*time.Second)
	}
}
