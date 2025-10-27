package scraper

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/utils"
	"github.com/playwright-community/playwright-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

func FetchSchedule(repo *database.Repository, config ScheduleConfig) (*RawSchedule, error) {
	departmentIDs, err := repo.GetDepartmentIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to get department IDs: %w", err)
	}

	url := scheduleURL(config, departmentIDs)

	resp, err := httpGetRequest(url, generateHeaders())
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

	if log.Logger.GetLevel() == zerolog.DebugLevel {
		filename := fmt.Sprintf("storage/cache/schedule_%s.html", time.Now().Format("20060102150405"))
		if err := os.WriteFile(filename, []byte(fixedEncoding), 0644); err != nil {
			log.Error().Err(err).Msg("Failed to save schedule HTML to file")
		} else {
			log.Debug().Msgf("Saved schedule HTML to file %s", filename)
		}
	}

	return parseSchedule(fixedEncoding, config)
}

func FetchScheduleWithBrowser(
	repo *database.Repository,
	browser *browser.BrowserService,
	config ScheduleConfig,
) (*RawSchedule, error) {
	departmentIDs, err := repo.GetDepartmentIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to get department IDs: %w", err)
	}
	url := scheduleURL(config, departmentIDs)

	var schedule *RawSchedule
	err = browser.WithPage(func(p playwright.Page) error {
		if err := p.SetExtraHTTPHeaders(generateHeaders()); err != nil {
			return fmt.Errorf("failed to set extra HTTP headers: %w", err)
		}
		if _, err := p.Goto(url); err != nil {
			return fmt.Errorf("failed to goto URL: %w", err)
		}

		tableLocator := p.Locator("#main_table")
		if err := tableLocator.WaitFor(playwright.LocatorWaitForOptions{}); err != nil {
			return fmt.Errorf("failed to wait for table: %w", err) // TODO: Implement retrying.
		}

		time.Sleep(1 * time.Second)

		html, err := p.Content()
		if err != nil {
			return fmt.Errorf("failed to get HTML content: %w", err)
		}

		if log.Logger.GetLevel() == zerolog.TraceLevel {
			filename := fmt.Sprintf("storage/cache/schedule_%s.html", time.Now().Format("20060102150405"))
			if err := os.WriteFile(filename, []byte(html), 0644); err != nil {
				log.Error().Err(err).Msg("Failed to save schedule HTML to file")
			} else {
				log.Debug().Msgf("Saved schedule HTML to file %s", filename)
			}
		}

		schedule, err = parseSchedule(html, config)
		return err
	})
	if err != nil {
		return nil, err
	}
	return schedule, nil
}

func parseSchedule(sourceHTML string, config ScheduleConfig) (*RawSchedule, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(sourceHTML))
	if err != nil {
		return nil, err
	}

	table := doc.Find("table#main_table")
	if table.Length() == 0 {
		return nil, fmt.Errorf("table not found")
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

	var rows []RawScheduleRow
	table.Find("tr.para_num:not(:first-child)").Each(func(i int, s *goquery.Selection) {
		rows = append(rows, parseScheduleRow(&config, headers, s))
	})
	return &RawSchedule{Config: config, Rows: rows}, nil
}

func parseScheduleRow(config *ScheduleConfig, headers []map[string]string, rowSelection *goquery.Selection) RawScheduleRow {
	numberStr := rowSelection.Find("td:first-child").First().Text()
	time_range := rowSelection.Find("td:nth-child(2)")

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		panic(err)
	}

	row := RawScheduleRow{
		Number:    number,
		TimeRange: time_range.First().Text(),
		Days:      [7]RawScheduleDay{},
	}
	rowSelection.Find("td:nth-child(n+3)").Each(func(i int, daySelection *goquery.Selection) {
		row.Days[i] = parseScheduleDay(config, headers[i], daySelection)
	})

	return row
}

func parseScheduleDay(config *ScheduleConfig, header map[string]string, daySelection *goquery.Selection) RawScheduleDay {
	day := RawScheduleDay{Date: header["date"], WeekDay: header["weekday"], WeekKind: header["week_kind"], Pair: Pair{}}

	day.Pair.Replaced = daySelection.Find("table").HasClass("zamena")
	day.Pair.Kind = detectPairKind(daySelection)

	switch day.Pair.Kind {
	case PairKindSubject:
		parseDisciplinePair(config, daySelection, &day.Pair)
	case PairKindExam:
		parseExamPair(daySelection, &day.Pair)
	case PairKindConsultation:
		parseConsultationPair(daySelection, &day.Pair)
	case PairKindEmpty:
		// Nothing
	default:
		parseOtherPair(daySelection, &day.Pair)
	}

	return day
}

func parseDisciplinePair(config *ScheduleConfig, daySelection *goquery.Selection, pair *Pair) {
	// log.Trace().Str("text", daySelection.Text()).Msg("teacher found")
	teacher := daySelection.Find(".prep").Text()
	pair.Teacher = &teacher
	classroom := daySelection.Find(".cabs").Text()
	pair.Classroom = classroom

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

func parseExamPair(daySelection *goquery.Selection, pair *Pair) {
	panic("unimplemented")
}

func parseConsultationPair(daySelection *goquery.Selection, pair *Pair) {
	panic("unimplemented")
}

func parseOtherPair(daySelection *goquery.Selection, pair *Pair) {
	panic("unimplemented")
}

func detectPairKind(daySelection *goquery.Selection) PairKind {
	switch {
	case daySelection.Find(".disc").Text() != "":
		return PairKindSubject
	case daySelection.HasClass("head_urok_kanik"):
		return PairKindVacation
	case daySelection.HasClass("event"):
		return PairKindEvent
	case daySelection.HasClass("head_urok_praktik"):
		return PairKindPractice
	case daySelection.HasClass("head_urok_session"):
		return PairKindSession
	case daySelection.HasClass("head_urok_iga"):
		return PairKindIGA
	case daySelection.HasClass("zachet") || daySelection.HasClass("difzachet") || daySelection.HasClass("ekzamen"):
		return PairKindExam
	case daySelection.Find("table.consultation").Length() > 0:
		return PairKindConsultation
	default:
		return PairKindEmpty
	}
}

// scheduleURL returns formatted URL for group or teacher schedule page depending on the given schedule config.
// Parameter departmentIDs is used for teacher schedule page only and may be empty or nil for group.
func scheduleURL(config ScheduleConfig, departmentIDs []string) string {
	switch {
	case config.Group != nil:
		zaochnoeFlag := ""
		if strings.Contains(strings.ToLower(config.Group.DepartmentName), "заоч") {
			zaochnoeFlag = "z"
		}
		return fmt.Sprintf(
			"https://coworking.tyuiu.ru/shs/all_t/sh%s.php?action=group&union=0&sid=%s&gr=%s&year=%d&vr=1",
			zaochnoeFlag, config.Group.DepartmentID, config.Group.GroupID, time.Now().Year())
	case config.Teacher != nil:
		departmentArgs := ""
		for i, id := range departmentIDs {
			departmentArgs += fmt.Sprintf("&shed[%d]=%s&union[%d]=0&year[%d]=%d", i, id, i, i, time.Now().Year())
		}
		return fmt.Sprintf(
			"https://coworking.tyuiu.ru/shs/all_t/sh.php?action=prep&prep=%s&vr=1&count=%d%s",
			config.Teacher.TeacherID, len(departmentIDs), departmentArgs)
	default:
		panic("invalid schedule config")
	}
}
