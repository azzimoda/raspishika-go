package scraper

import (
	"fmt"
	"net/http"
	"time"

	"github.com/corpix/uarand"
	"github.com/rs/zerolog/log"
)

func httpGetRequest(url string, headers map[string]string) (*http.Response, error) {
	log.Debug().Str("url", url).Any("headers", headers).Msg("HTTP GET request")

	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Add(key, value)
	}

	const MaxRetries = 3 // TODO: Move it into config.
	retries := 0
	for retries < MaxRetries {
		resp, err := client.Do(req)
		if err == nil {
			log.Debug().Str("url", url).Int("statusCode", resp.StatusCode).Msg("HTTP GET request succeeded")
			return resp, nil
		}

		log.Error().Err(err).Msgf("HTTP GET request failed")
		retries++
		time.Sleep(time.Duration(retries) * time.Second)
	}
	return nil, fmt.Errorf("failed to get %s after %d retries", url, MaxRetries)
}

func generateHeaders() map[string]string {
	return map[string]string{
		"User-Agent": uarand.GetRandom(),
		"Referer":    "https://coworking.tyuiu.ru/shs/all_t/",
	}
}
