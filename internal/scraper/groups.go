package scraper

import (
	"net/http"
	"strings"
	"time"

	"github.com/azzimoda/raspishika-go/internal/cache"

	"github.com/PuerkitoBio/goquery"
	"github.com/corpix/uarand"
)

const DepartmentsURL = "https://mnokol.tyuiu.ru/site/index.php?option=com_content&view=article&id=1582&Itemid=247"
const DepartmentsCacheKey = "departments"

type Department struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func FetchDepartments(cache *cache.Cache) ([]Department, error) {
	if data, found := cache.C.Get(DepartmentsCacheKey); found {
		return data.([]Department), nil
	}

	resp, err := httpGetRequest(DepartmentsURL, generateHeaders())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	const BaseURL = "https://mnokol.tyuiu.ru"

	var departments []Department
	doc.Find("ul.mod-menu li.col-lg.col-md-6 a").Each(func(i int, s *goquery.Selection) {
		name := s.Text()
		if !strings.Contains(strings.ToLower(name), "отделение") && !strings.Contains(strings.ToLower(name), "заоч") {
			return
		}

		departments = append(departments, Department{
			Name: name,
			URL:  BaseURL + strings.ReplaceAll(s.AttrOr("href", ""), "&amp;", "&"),
		})
	})

	cache.C.Set(DepartmentsCacheKey, departments, time.Duration(cache.Config.GroupTTL)*24*time.Hour)
	return departments, nil
}

func httpGetRequest(url string, headers map[string]string) (*http.Response, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Add(key, value)
	}
	return client.Do(req)
}

func generateHeaders() map[string]string {
	return map[string]string{
		"User-Agent": uarand.GetRandom(),
		"Referer":    "httos://coworking.tyuiu.ru/shs/all_t/",
	}
}
