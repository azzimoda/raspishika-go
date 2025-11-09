package scraper

import (
	"fmt"
	"net/http"
	"time"

	"github.com/corpix/uarand"
	"github.com/rs/zerolog/log"
)

func httpGetRequestRetryingRandomHeaders(url string, maxRetries int) (*http.Response, error) {
	retries := 0
	for retries < maxRetries {
		resp, err := httpGetRequest(url, generateHeaders())
		if err == nil && resp.StatusCode == 200 {
			log.Debug().Str("url", url).Int("statusCode", resp.StatusCode).Msg("HTTP GET request succeeded")
			return resp, nil
		}

		e := log.Error().Err(err)
		if resp != nil {
			e = e.Str("status", resp.Status)
		}
		e.Msgf("HTTP GET request failed")

		retries++
		time.Sleep(time.Duration(retries) * time.Second)
	}
	return nil, fmt.Errorf("failed to get %s after %d retries", url, maxRetries)
}

func httpGetRequest(url string, headers map[string]string) (*http.Response, error) {
	log.Debug().Str("url", url).Any("headers", headers).Msg("HTTP GET request")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range headers {
		req.Header.Add(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func generateHeaders() map[string]string {
	return map[string]string{
		"User-Agent": uarand.GetRandom(),
		"Referer":    "https://coworking.tyuiu.ru/shs/all_t/",
	}
}
