package scraper

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"golang.org/x/net/html"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/internal/services/browser"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

var (
	ErrParserPanicked = errors.New("parser panicked")
)

func ScrapeSchedule(url models.URL, config models.ScheduleConfig) (*models.RawSchedule, error) {
	log.Trace().Msg("Scraping schedule with HTTP")

	resp, err := utils.HTTPGetRequestRetryingRandomHeaders(url.String(), 10)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %s", resp.Status)
	}

	text, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	fixedEncoding, err := utils.Windows1251ToUTF8(string(text))
	if err != nil {
		return nil, fmt.Errorf("encoding conversion failed: %w", err)
	}

	if log.Logger.GetLevel() == zerolog.TraceLevel {
		SaveScheduleHTML(config, fixedEncoding)
	}

	return parseSchedule(fixedEncoding, config)
}

func SaveScheduleHTML(conf models.ScheduleConfig, html string) (filename string, err error) {
	filename = "schedule_temp.html"
	if conf.Group != nil {
		filename = fmt.Sprintf("schedule_group_%s_%s.html", conf.Group.DepartmentName, conf.Group.GroupName)
	} else if conf.Teacher != nil {
		filename = fmt.Sprintf("schedule_teacher_%s.html", conf.Teacher.Name)
	}
	filename = path.Join(viper.GetString(config.KeyCacheDir), filename) // TODO NOW: Continue using config key constants!
	if err := os.WriteFile(filename, []byte(html), 0644); err != nil {
		log.Error().Err(err).Msg("Failed to save schedule HTML to file")
		return "", err
	} else {
		log.Trace().Msgf("Saved schedule HTML to file %s", filename)
	}
	return filename, nil
}

func ScrapeScheduleWithBrowser(
	browser *browser.BrowserService,
	url models.URL,
	config models.ScheduleConfig,
) (*models.RawSchedule, error) {
	log.Trace().Any("URL", url).Any("scheduleConfig", config).Msg("Scraping schedule with browser")

	html, err := fetchSchedulePageWithBrowser(browser, url, config)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch schedule page with browser: %w", err)
	}
	return parseSchedule(html, config)
}

func fetchSchedulePageWithBrowser(
	browser *browser.BrowserService,
	url models.URL,
	conf models.ScheduleConfig,
) (string, error) {
	var lastErr error
	var html string
	for range viper.GetInt(config.KeyBrowserMaxRetries) {
		headers := utils.GenerateHeaders()
		lastErr = browser.WithPage(func(p playwright.Page) (err error) {
			log.Trace().Msgf("Fetching schedule page...")

			if err = p.SetExtraHTTPHeaders(headers); err != nil {
				log.Error().Err(err).Msgf("Failed to set extra HTTP headers; retrying...")
				return fmt.Errorf("failed to set extra HTTP headers: %w", err)
			}
			if _, err = p.Goto(url.String()); err != nil {
				log.Error().Err(err).Msgf("Failed to goto URL; retrying...")
				return fmt.Errorf("failed to goto URL: %w", err)
			}
			tableLocator := p.Locator("#main_table")
			if err = tableLocator.WaitFor(playwright.LocatorWaitForOptions{}); err != nil {
				log.Error().Err(err).Msgf("Failed to wait for table; retrying...")
				return fmt.Errorf("failed to wait for table: %w", err)
			}
			time.Sleep(1 * time.Second)

			html, err = p.Content()
			if err != nil {
				log.Error().Err(err).Msgf("Failed to get HTML content; retrying...")
				return fmt.Errorf("failed to get HTML content: %w", err)
			}
			return err
		})
		if lastErr == nil {
			break
		}
		log.Error().Err(lastErr).Any("url", url).Any("headers", headers).
			Msgf("Failed to fetch schedule page; retrying...")
		time.Sleep(1 * time.Second)
	}

	if lastErr != nil {
		return "", fmt.Errorf("failed to fetch schedule page with browser after %d retries: %w",
			viper.GetInt(config.KeyBrowserMaxRetries), lastErr)
	}

	if log.Logger.GetLevel() == zerolog.TraceLevel {
		filename := fmt.Sprintf("schedule_%s.html", time.Now().Format("20060102"))
		if conf.Group != nil {
			filename = fmt.Sprintf("schedule_%s_%s.html", conf.Group.DepartmentName, conf.Group.GroupName)
		} else if conf.Teacher != nil {
			filename = fmt.Sprintf("schedule_%s.html", conf.Teacher.Name)
		}
		filename = filepath.Join(viper.GetString(config.KeyCacheDir), filename)
		if err := os.WriteFile(filename, []byte(html), 0644); err != nil {
			log.Error().Err(err).Msg("Failed to save schedule HTML to file")
		} else {
			log.Debug().Msgf("Saved schedule HTML to file %s", filename)
		}
	}

	return html, nil
}

func parseSchedule(sourceHTML string, config models.ScheduleConfig) (schedule *models.RawSchedule, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Err(fmt.Errorf("panic: %+v", r)).Msg("Parser panicked")
			err = fmt.Errorf("%w: %v", ErrParserPanicked, r)
		}
	}()

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(sourceHTML))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	table := doc.Find("table#main_table")
	if table.Length() == 0 {
		return nil, fmt.Errorf("table element not found")
	}

	var headers []map[string]string
	table.Find("tr").First().Find("td").Slice(2, goquery.ToEnd).Each(func(i int, s *goquery.Selection) {
		var parts []string
		for _, node := range s.Nodes {
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode {
					text := strings.TrimSpace(c.Data)
					if text != "" {
						parts = append(parts, text)
					}
				}
			}
		}

		for len(parts) < 3 {
			parts = append(parts, "")
		}

		headers = append(headers, map[string]string{"date": parts[0], "weekday": parts[1], "week_kind": parts[2]})
	})

	var rows []models.RawScheduleRow
	table.Find("tr.para_num:not(:first-child)").Each(func(i int, s *goquery.Selection) {
		rows = append(rows, parseScheduleRow(&config, headers, s))
	})
	return &models.RawSchedule{Config: config, Rows: rows}, nil
}

func parseScheduleRow(config *models.ScheduleConfig, headers []map[string]string, rowSelection *goquery.Selection) models.RawScheduleRow {
	numberStr := rowSelection.Find("td:first-child").First().Text()
	time_range := rowSelection.Find("td:nth-child(2)")

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		panic(err)
	}

	row := models.RawScheduleRow{
		Number:    number,
		TimeRange: models.TimeRange(time_range.First().Text()),
		Days:      []models.RawScheduleDay{},
	}
	rowSelection.Find("td:nth-child(n+3)").Each(func(i int, daySelection *goquery.Selection) {
		row.Days = append(row.Days, parseScheduleDay(config, headers[i], daySelection))
	})

	return row
}

func parseScheduleDay(
	config *models.ScheduleConfig,
	header map[string]string,
	daySelection *goquery.Selection,
) models.RawScheduleDay {
	day := models.RawScheduleDay{
		Date:     models.Date(header["date"]),
		WeekDay:  models.Weekday(header["weekday"]),
		WeekKind: models.WeekKind(header["week_kind"]),
		Pair:     models.Pair{},
	}

	day.Pair.Replaced = daySelection.Find("table").HasClass("zamena")
	day.Pair.Kind = detectPairKind(daySelection)

	switch day.Pair.Kind {
	case models.PairKindSubject:
		parseDisciplinePair(config, daySelection, &day.Pair)
	case models.PairKindExam, models.PairKindConsultation:
		parseExamConsultationPair(daySelection, &day.Pair)
	case models.PairKindEmpty:
		// Nothing
	default:
		parseOtherPair(daySelection, &day.Pair)
	}

	return day
}

func parseDisciplinePair(config *models.ScheduleConfig, daySelection *goquery.Selection, pair *models.Pair) {
	// log.Trace().Str("text", daySelection.Text()).Msg("teacher found")
	teacher := daySelection.Find(".prep").Text()
	pair.Teacher = &teacher
	classroom := daySelection.Find(".cabs").Text()
	pair.Classroom = classroom
	subgroupSelection := daySelection.Find(".podgrupp")
	if subgroupSelection != nil {
		pair.Subgroup = subgroupSelection.Text()
	}

	if config.Group != nil {
		discipline := daySelection.Find(".disc").Text()
		pair.Discipline = discipline
	} else {
		var parts []string
		for _, node := range daySelection.Find(".disc").Nodes {
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode {
					text := strings.TrimSpace(c.Data)
					if text != "" {
						parts = append(parts, text)
					}
				} else if c.Type == html.ElementNode && c.Data == "div" {
					parts = append(parts, c.FirstChild.Data)
				}
			}
		}

		for len(parts) < 2 {
			parts = append(parts, "")
		}

		pair.Discipline = parts[0]
		pair.Group = &parts[1]
	}
}

func parseExamConsultationPair(daySelection *goquery.Selection, pair *models.Pair) {
	pair.Title = daySelection.Find(".head_ekz").Text()
	pair.Discipline = daySelection.Find(".disc").Text()
	teacher := daySelection.Find(".prep").Text()
	pair.Teacher = &teacher
	pair.Classroom = daySelection.Find(".cabs").Text()
}

func parseOtherPair(daySelection *goquery.Selection, pair *models.Pair) {
	pair.Label = daySelection.Text()
}

func detectPairKind(daySelection *goquery.Selection) models.PairKind {
	switch {
	case strings.Contains(strings.ToLower(daySelection.Find(".disc").Text()), "снято"):
		return models.PairKindEmpty
	case daySelection.Find(".disc").Text() != "":
		return models.PairKindSubject
	case daySelection.HasClass("head_urok_kanik"):
		return models.PairKindVacation
	case daySelection.HasClass("event"):
		return models.PairKindEvent
	case daySelection.HasClass("head_urok_praktik"):
		return models.PairKindPractice
	case daySelection.HasClass("head_urok_session"):
		return models.PairKindSession
	case daySelection.HasClass("head_urok_iga"):
		return models.PairKindIGA
	case daySelection.HasClass("zachet") || daySelection.HasClass("difzachet") || daySelection.HasClass("ekzamen"):
		return models.PairKindExam
	case daySelection.Find("table.consultation").Length() > 0:
		return models.PairKindConsultation
	default:
		return models.PairKindEmpty
	}
}

// ScheduleURL returns formatted URL for group or teacher schedule page depending on the given schedule config.
// Parameter departmentIDs is used for teacher schedule page only and may be empty or nil for group.
//
// Returns empty string if config is invalid.
func ScheduleURL(config models.ScheduleConfig, departmentIDs []models.DepartmentID) models.URL {
	switch {
	case config.Group != nil:
		zFlag := "" // Заочное обучение?
		if strings.Contains(strings.ToLower(config.Group.DepartmentName.String()), "заоч") {
			zFlag = "z"
		}
		return models.URL(fmt.Sprintf(
			"https://coworking.tyuiu.ru/shs/all_t/sh%s.php?action=group&union=0&sid=%s&gr=%s&year=%d&vr=1",
			zFlag, config.Group.DepartmentID, config.Group.GroupID, config.Group.Year))
	case config.Teacher != nil:
		var departmentArgs strings.Builder
		for i, id := range departmentIDs {
			fmt.Fprintf(
				&departmentArgs,
				"&shed[%d]=%s&union[%d]=0&year[%d]=%d",
				i, id, i, i, time.Now().Year(),
				// Note: Here I use current year because I cannot fetch it from DB.
				// Commonly it shouldn't give trouble,
				// because the year in their DB changes at the end of the first semester.
			)
		}
		return models.URL(fmt.Sprintf(
			"https://coworking.tyuiu.ru/shs/all_t/sh.php?action=prep&prep=%s&vr=1&count=%d%s",
			config.Teacher.TeacherID, len(departmentIDs), departmentArgs.String()))
	default:
		// Error: invalid config
		return ""
	}
}
