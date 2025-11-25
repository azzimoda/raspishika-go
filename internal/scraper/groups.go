package scraper

import (
	"fmt"
	"strings"
	"time"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/utils"

	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
	"github.com/rs/zerolog/log"
)

const DepartmentsURL = "https://mnokol.tyuiu.ru/site/index.php?option=com_content&view=article&id=1582&Itemid=247"
const BaseDepartmentPageURL = "https://mnokol.tyuiu.ru"
const DepartmentsCacheKey = "departments"

type Department struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func FetchDepartments(cache *cache.Cache) ([]Department, error) {
	if data, found := cache.C.Get(DepartmentsCacheKey); found {
		return data.([]Department), nil
	}

	resp, err := utils.HTTPGetRequestRetryingRandomHeaders(DepartmentsURL, 10)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var departments []Department
	doc.Find("ul.mod-menu li.col-lg.col-md-6 a").Each(func(i int, s *goquery.Selection) {
		name := s.Text()
		if !strings.Contains(strings.ToLower(name), "отделение") && !strings.Contains(strings.ToLower(name), "заоч") {
			return
		}

		departments = append(departments, Department{
			Name: name,
			URL:  BaseDepartmentPageURL + strings.ReplaceAll(s.AttrOr("href", ""), "&amp;", "&"),
		})
	})

	cache.C.Set(DepartmentsCacheKey, departments, cache.Config.GroupTTLDuration())
	return departments, nil
}

func FetchGroups(
	repo *database.Repository, browser *browser.BrowserService, cache *cache.Cache,
) ([]database.Group, error) {
	if groups, err := checkGroups(repo, cache.Config.GroupTTLDuration()); err == nil && len(groups) > 0 {
		log.Debug().Msg("Using cached groups")
		return groups, nil
	}
	log.Debug().Msg("Fetching groups")

	departments, err := FetchDepartments(cache)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch departments: %w", err)
	}

	groups := make([]database.Group, 0)
	for _, department := range departments {
		departmentGroups, err := scrapeDepartmentGroups(browser, department)
		if err != nil {
			return nil, fmt.Errorf("failed to scrape department (%s) groups: %w", department.Name, err)
		}
		groups = append(groups, departmentGroups...)
	}
	log.Trace().Int("groupsCount", len(groups)).Msg("Fetched groups")

	if err := repo.UpdateGroups(groups); err != nil {
		log.Error().Err(err).Msg("Failed to update groups")
		return nil, fmt.Errorf("failed to update groups: %w", err)
	}

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

func checkGroups(repo *database.Repository, ttl time.Duration) ([]database.Group, error) {
	groups, err := repo.GetGroups()
	if err != nil {
		return nil, err
	}

	for _, group := range groups {
		if time.Since(group.UpdatedAt) < ttl {
			return groups, nil
		}
	}
	return nil, fmt.Errorf("all groups are outdated")
}

func scrapeDepartmentGroups(browser *browser.BrowserService, department Department) ([]database.Group, error) {
	log.Trace().Str("departmentName", department.Name).Str("departmentURL", department.URL).
		Msg("Scraping department groups")

	groups := make([]database.Group, 0)
	err := browser.WithPage(func(p playwright.Page) error {
		log.Trace().Msg("Navigating to department page")

		if _, err := p.Goto(department.URL); err != nil {
			return fmt.Errorf("failed to navigate to department page: %w", err)
		}

		frameLocator := p.FrameLocator("div.com-content-article__body iframe")
		if err := frameLocator.Locator("#groups").WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(60_000),
		}); err != nil {
			return fmt.Errorf("failed to wait for groups iframe: %w", err)
		}

		options, err := frameLocator.Locator("#groups option").EvaluateAll(
			`els => els.map(el => ({ text: el.textContent.trim(), value: el.value, sid: el.getAttribute("sid") }))`)
		if err != nil {
			return fmt.Errorf("failed to get groups options: %w", err)
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
