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
	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/service/browser"
)

const DepartmentsURL = "https://mnokol.tyuiu.ru/site/index.php?option=com_content&view=article&id=1582&Itemid=247"
const BaseDepartmentPageURL = "https://mnokol.tyuiu.ru"

type Department struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func FetchDepartments(repo repository.GroupRepository) ([]model.Department, error) {
	// Check cache
	departments, err := repo.Departments()
	ttl := config.GroupTTLDur()
	if err != nil && Every(departments, func(d *model.Department) bool { return d.IsActual(ttl) }) {
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

	departments = make([]model.Department, 0)
	doc.Find("ul.mod-menu li.col-lg.col-md-6 a").Each(func(i int, s *goquery.Selection) {
		name := s.Text()
		if !strings.Contains(strings.ToLower(name), "отделение") && !strings.Contains(strings.ToLower(name), "заоч") {
			return
		}

		url := BaseDepartmentPageURL + strings.ReplaceAll(s.AttrOr("href", ""), "&amp;", "&")
		department := model.Department{Name: model.DepartmentName(name), URL: model.URL(url)}
		if err := repo.InsertOrUpdateDepartment(&department); err != nil {
			log.Error().Err(err).Msg("Failed to update department cache in DB")
		}
		departments = append(departments, department)
	})

	return departments, nil
}

func FetchDepartmentIDs(
	repo repository.GroupRepository,
	browser *browser.BrowserService,
) ([]model.DepartmentID, error) {
	log.Trace().Msg("Fetching departments...")
	if _, err := FetchGroups(repo, browser); err != nil {
		return nil, err
	}
	return repo.DepartmentIDs()
}

func FetchGroups(repo repository.GroupRepository, browser *browser.BrowserService) ([]model.Group, error) {
	if groups, err := checkGroups(repo, config.GroupTTLDur()); err == nil && len(groups) > 0 {
		log.Trace().Msg("Groups cache hit")
		return groups, nil
	}
	log.Debug().Msg("Fetching groups")

	departments, err := FetchDepartments(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch departments: %w", err)
	}

	groups := make([]model.Group, 0)
	for _, department := range departments {
		departmentGroups, err := scrapeDepartmentGroups(browser, &department)
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
	repo repository.GroupRepository,
	browser *browser.BrowserService,
	name model.DepartmentName,
) ([]model.Group, error) {
	groups, err := FetchGroups(repo, browser)
	if err != nil {
		return nil, err
	}

	departmentGroups := make([]model.Group, 0)
	for _, group := range groups {
		if group.DepartmentName == name {
			departmentGroups = append(departmentGroups, group)
		}
	}
	return departmentGroups, nil
}

func checkGroups(repo repository.GroupRepository, ttl time.Duration) ([]model.Group, error) {
	groups, err := repo.All()
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

func scrapeDepartmentGroups(browser *browser.BrowserService, department *model.Department) ([]model.Group, error) {
	log.Trace().Any("department", department).Msg("Scraping department groups...")

	groups := make([]model.Group, 0)
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

			groups = append(groups, model.Group{
				GroupID:        model.GroupID(opt["value"].(string)),
				DepartmentID:   model.DepartmentID(opt["sid"].(string)),
				GroupName:      model.GroupName(opt["text"].(string)),
				Year:           model.Year(year),
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
