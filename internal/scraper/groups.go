package scraper

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"

	"github.com/PuerkitoBio/goquery"
	"github.com/corpix/uarand"
	"github.com/playwright-community/playwright-go"
	"github.com/rs/zerolog/log"
)

const DepartmentsURL = "https://mnokol.tyuiu.ru/site/index.php?option=com_content&view=article&id=1582&Itemid=247"
const (
	DepartmentsCacheKey = "departments"
)

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

func FetchGroups(
	repo *database.Repository, browser *browser.BrowserService, cache *cache.Cache,
) ([]database.Group, error) {
	if groups, err := checkGroups(repo); err == nil && len(groups) > 0 {
		log.Debug().Msg("Using cached groups")
		return groups, nil
	}
	log.Debug().Msg("Fetching groups")

	departments, err := FetchDepartments(cache)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch departments: %v", err)
	}

	groups := make([]database.Group, 0)
	for _, department := range departments {
		departmentGroups, err := scrapeDepartmentGroups(browser, department)
		if err != nil {
			return nil, fmt.Errorf("failed to scrape department (%s) groups: %v", department.Name, err)
		}
		groups = append(groups, departmentGroups...)
	}

	if err := repo.UpdateGroups(groups); err != nil {
		log.Error().Err(err).Msg("Failed to update groups")
		return nil, fmt.Errorf("failed to update groups: %v", err)
	}

	log.Trace().Int("groupsCount", len(groups)).Msg("Fetched groups")

	cache.C.Set(GroupsCacheKey, groups, time.Duration(cache.Config.GroupTTL)*24*time.Hour)
	return groups, nil
}

func FetchDepartmentGroups(
	repo *database.Repository, browser *browser.BrowserService, cache *cache.Cache, departmentName string,
) ([]database.Group, error) {
	groups, err := FetchGroups(repo, browser, cache)
	if err != nil {
		return nil, err
	}

	departmentGroups := make([]database.Group, 0)
	for _, group := range groups {
		if group.DepartmentName == departmentName {
			departmentGroups = append(departmentGroups, group)
		}
	}
	return departmentGroups, nil
}

func checkGroups(repo *database.Repository) ([]database.Group, error) {
	groups, err := repo.GetGroups()
	if err != nil {
		return nil, err
	}

	for _, group := range groups {
		// Group is actual when it is updated in the last 7 days.
		if time.Since(group.UpdatedAt) < 7*24*time.Hour {
			return groups, nil
		}
	}
	return nil, fmt.Errorf("All groups are outdated")
}

func scrapeDepartmentGroups(browser *browser.BrowserService, department Department) ([]database.Group, error) {
	log.Trace().Str("departmentName", department.Name).Str("departmentURL", department.URL).
		Msg("Scraping department groups")

	groups := make([]database.Group, 0)
	err := browser.WithPage(func(p playwright.Page) error {
		log.Trace().Msg("Navigating to department page")

		if _, err := p.Goto(department.URL); err != nil {
			return err
		}

		iframeLocator := p.FrameLocator("div.com-content-article__body iframe")
		selectLocator := iframeLocator.Locator("#groups")
		if err := selectLocator.WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(60_000),
		}); err != nil {
			return err
		}

		options, err := iframeLocator.Locator("#groups option").EvaluateAll(
			`els => els.map(el => ({ text: el.textContent.trim(), value: el.value, sid: el.getAttribute("sid") }))`)
		if err != nil {
			return err
		}

		for _, opt := range options.([]any) {
			opt := opt.(map[string]any)
			if opt["value"] == nil || opt["text"] == nil || opt["sid"] == nil {
				continue
			}

			groups = append(groups, database.Group{
				GroupID: opt["value"].(string), DepartmentID: opt["sid"].(string),
				GroupName: opt["text"].(string), DepartmentName: department.Name})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return groups, nil
}
