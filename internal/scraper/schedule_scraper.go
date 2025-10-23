package scraper

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/pkg/utils"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

func FetchGroupSchedule(cache *cache.Cache, config ScheduleConfig) (*RawSchedule, error) {
	url := scheduleURL(config)

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

	return parseSchedule(fixedEncoding, config)
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
		Days:      []RawScheduleDay{},
	}
	rowSelection.Find("td:nth-child(n+3)").Each(func(i int, daySelection *goquery.Selection) {
		day := parseScheduleDay(config, headers[i], daySelection)
		row.Days = append(row.Days, day)
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
	pair.Classroom = &classroom

	if config.Group != nil {
		discipline := daySelection.Find(".disc").Text()
		pair.Discipline = &discipline
	} else {
		// TODO
		panic("unimplemented")
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
	case daySelection.HasClass("head_urok_block") ||
		strings.ToLower(daySelection.Text()) == "нет занятий" ||
		strings.ToLower(daySelection.Text()) == "снято":

		return PairKindEmpty
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
		return PairKindSubject
	}
}

func scheduleURL(config ScheduleConfig) string {
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
		return "" // TODO: Implement teacher schedule URL.
	default:
		panic("invalid schedule config")
	}
}
