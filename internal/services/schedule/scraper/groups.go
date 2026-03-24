package scraper

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/services/browser"
)

const DepartmentsURL = "https://mnokol.tyuiu.ru/site/index.php?option=com_content&view=article&id=1582&Itemid=247"
const BaseDepartmentPageURL = "https://mnokol.tyuiu.ru"

type Department struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func FetchDepartments(repo *repository.Repository) ([]models.Department, error) {
	// Check cache
	departments, err := models.GetDepartments(repo.DB)
	ttl := config.GroupTTLDur()
	if err != nil && Every(departments, func(d *models.Department) bool { return d.IsActual(ttl) }) {
		log.Debug().Msg("Departments cache hit")
		return departments, nil
	}

	// Update cache
	log.Debug().Msg("Departments cache miss")
	resp, err := HTTPGetRequestRetryingRandomHeaders(DepartmentsURL, 10)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	departments = make([]models.Department, 0)
	doc.Find("ul.mod-menu li.col-lg.col-md-6 a").Each(func(i int, s *goquery.Selection) {
		name := s.Text()
		if !strings.Contains(strings.ToLower(name), "отделение") && !strings.Contains(strings.ToLower(name), "заоч") {
			return
		}

		url := BaseDepartmentPageURL + strings.ReplaceAll(s.AttrOr("href", ""), "&amp;", "&")
		department := models.Department{Name: models.DepartmentName(name), URL: models.URL(url)}
		departments = append(departments, department)
		if err := department.InsertOrUpdate(repo.DB); err != nil {
			log.Error().Err(err).Msg("Failed to update department cache in DB")
		}
	})

	return departments, nil
}

func FetchDepartmentIDs(repo *repository.Repository, browser *browser.BrowserService) ([]models.DepartmentID, error) {
	log.Trace().Msg("Fetching departments...")
	if _, err := FetchGroups(repo, browser); err != nil {
		return nil, err
	}
	return models.GetDepartmentIDs(repo.DB)
}

func FetchGroups(repo *repository.Repository, browser *browser.BrowserService) ([]models.Group, error) {
	if groups, err := checkGroups(repo, config.GroupTTLDur()); err == nil && len(groups) > 0 {
		log.Trace().Msg("Groups cache hit")
		return groups, nil
	}
	log.Debug().Msg("Fetching groups")

	departments, err := FetchDepartments(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch departments: %w", err)
	}

	groups := make([]models.Group, 0)
	for _, department := range departments {
		departmentGroups, err := scrapeDepartmentGroups(browser, &department)
		if err != nil {
			return nil, fmt.Errorf("failed to scrape department (%s) groups: %w", department.Name, err)
		}
		groups = append(groups, departmentGroups...)
	}
	log.Trace().Int("groupsCount", len(groups)).Msg("Fetched groups")

	if err := models.UpdateGroups(repo.DB, groups); err != nil {
		log.Error().Err(err).Msg("Failed to update groups")
		return nil, fmt.Errorf("failed to update groups: %w", err)
	}

	return groups, nil
}

func FetchDepartmentGroups(
	repo *repository.Repository,
	browser *browser.BrowserService,
	name models.DepartmentName,
) ([]models.Group, error) {
	groups, err := FetchGroups(repo, browser)
	if err != nil {
		return nil, err
	}

	departmentGroups := make([]models.Group, 0)
	for _, group := range groups {
		if group.DepartmentName == name {
			departmentGroups = append(departmentGroups, group)
		}
	}
	return departmentGroups, nil
}

func checkGroups(repo *repository.Repository, ttl time.Duration) ([]models.Group, error) {
	groups, err := models.GetGroups(repo.DB)
	if err != nil {
		return nil, err
	}

	outdatedCount := 0
	for _, group := range groups {
		if time.Since(group.UpdatedAt) > ttl {
			outdatedCount++
		}
	}
	outdatedGroupsPercent := float64(outdatedCount) / float64(len(groups))
	log.Trace().Float64("outdatedGroupsPercent", outdatedGroupsPercent).Msg("Checked groups")
	if outdatedGroupsPercent <= 0.5 {
		return groups, nil
	}
	return nil, fmt.Errorf("more than 50%% groups are outdated")
}

func scrapeDepartmentGroups(browser *browser.BrowserService, department *models.Department) ([]models.Group, error) {
	log.Trace().Any("department", department).Msg("Scraping department groups...")

	groups := make([]models.Group, 0)
	err := browser.WithPage(func(p playwright.Page) error {
		log.Trace().Msg("Navigating to department page")

		if _, err := p.Goto(department.URL.String()); err != nil {
			return fmt.Errorf("failed to navigate to department page: %w", err)
		}

		frameLocator := p.FrameLocator("div.com-content-article__body iframe")
		if err := frameLocator.Locator("#groups").WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(60_000),
		}); err != nil {
			return fmt.Errorf("failed to wait for groups iframe: %w", err)
		}

		options, err := frameLocator.Locator("#groups option").EvaluateAll(
			`els => els.map(el => ({ text: el.textContent.trim(), value: el.value, sid: el.getAttribute("sid"), year: el.getAttribute("year") }))`)
		if err != nil {
			return fmt.Errorf("failed to get groups options: %w", err)
		}

		for _, opt := range options.([]any) {
			opt := opt.(map[string]any)
			if !(validateOptionValue(opt["value"]) &&
				validateOptionValue(opt["text"]) &&
				validateOptionValue(opt["sid"]) &&
				validateOptionValue(opt["year"])) {
				log.Trace().Msg("Option is invalid")

				continue
			}

			year, err := strconv.ParseInt(opt["year"].(string), 10, 64)
			if err != nil {
				continue
			}

			groups = append(groups, models.Group{
				GroupID:        models.GroupID(opt["value"].(string)),
				DepartmentID:   models.DepartmentID(opt["sid"].(string)),
				GroupName:      models.GroupName(opt["text"].(string)),
				Year:           models.Year(year),
				DepartmentName: department.Name,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return groups, nil
}

func validateOptionValue(value any) bool {
	if value == nil {
		return false
	}
	if s, ok := value.(string); !ok {
		return false
	} else {
		return s != ""
	}
}

func Every[T any](elems []T, predicate func(*T) bool) bool {
	for _, elem := range elems {
		if !predicate(&elem) {
			return false
		}
	}
	return true
}
