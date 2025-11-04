package scraper

import (
	"net/http"
	"testing"

	"github.com/azzimoda/raspishika-go/pkg/logger"
)

func Test_httpGetRequest(t *testing.T) {
	logger.SetupLogger("trace")

	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		url     string
		headers map[string]string
		want    *http.Response
		wantErr bool
	}{
		{"must succeed", "https://www.google.com", map[string]string{}, &http.Response{}, false},
		{"must fail", "https://kek.huher.huh/42", map[string]string{}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := httpGetRequest(tt.url, tt.headers)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("httpGetRequest() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("httpGetRequest() succeeded unexpectedly")
			}
			// TODO: update the condition below to compare got with tt.want.
			if got == nil || got.StatusCode != 200 {
				t.Errorf("httpGetRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}
