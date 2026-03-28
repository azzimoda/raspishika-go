package scraper

import (
	"net/http"
	"testing"

	"github.com/azzimoda/raspishika-go/pkg/logger"
)

func Test_httpGetRequest(t *testing.T) {
	logger.SetupLogger("trace", "")

	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		url     string
		want    *http.Response
		wantErr bool
	}{
		{"must succeed", "https://www.google.com", &http.Response{}, false},
		{"must fail", "https://chatgpt.com", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := HTTPGetRequestRetryingRandomHeaders(tt.url, 3)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("httpGetRequest() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("httpGetRequest() succeeded unexpectedly")
			}
			if got == nil || got.StatusCode != 200 {
				t.Errorf("httpGetRequest() = %v", got)
			}
		})
	}
}
