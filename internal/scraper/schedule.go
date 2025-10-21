package scraper

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type PairKind string

const (
	PairKindEmpty        PairKind = "empty"
	PairKindVacation     PairKind = "vacation"
	PairKindEvent        PairKind = "event"
	PairKindSession      PairKind = "session"
	PairKindIGA          PairKind = "iga"
	PairKindSubject      PairKind = "subject"
	PairKindPractice     PairKind = "practice"
	PairKindExam         PairKind = "exam"
	PairKindConsultation PairKind = "consultation"
)

type ScheduleConfig struct {
	Group   *database.Group
	Teacher *database.Teacher
}

type Pair struct {
	Kind       PairKind `json:"kind"`
	Label      string   `json:"label"`
	Title      *string  `json:"title"`
	Discipline *string  `json:"discipline"`
	Teacher    *string  `json:"teacher"`
	Group      *string  `json:"group"`
	Classroom  *string  `json:"classroom"`
	Replaced   bool     `json:"replaced"`
}

type RawScheduleDay struct {
	Date     string
	WeekDay  string
	WeekKind string
	Pair     Pair
}

type RawScheduleRow struct {
	Number    int
	TimeRange string
	Days      []RawScheduleDay
}

type RawSchedule struct {
	Config ScheduleConfig
	Rows   []RawScheduleRow
}

// HTML returns HTML representation of the schedule.
func (s *RawSchedule) HTML(cache *cache.Cache, templateFile string) string {
	template := loadTemplate(cache, templateFile)

	var header string
	if s.Config.Group != nil {
		header = "Расписание группы " + s.Config.Group.GroupName + " — " + s.Config.Group.DepartmentName
	} else if s.Config.Teacher != nil {
		header = "Расписание преподавателя — " + s.Config.Teacher.Name
	} else {
		header = "Расписание"
	}

	tableHead := ""
	for _, day := range s.Rows[0].Days {
		tableHead += fmt.Sprintf("<th>%s<br>%s<br>%s</th>\n", day.Date, day.WeekDay, day.WeekKind)
	}

	html := strings.NewReplacer(
		"HEADER", header,
		"TABLE_HEAD", tableHead,
		"TABLE_BODY", generateTableBody(s.Rows),
		"TIMESTAMP", time.Now().Format(time.RFC3339),
	).Replace(template)

	if log.Logger.GetLevel() <= zerolog.DebugLevel {
		if err := os.MkdirAll("storage/cache/", 0755); err != nil {
			log.Error().Err(err).Msg("Failed to create cache directory")
		}
		filename := "storage/cache/schedule_" + s.Config.Group.GroupName + ".html"
		if err := os.WriteFile(filename, []byte(html), 0644); err != nil {
			log.Error().Err(err).Msg("Failed to save schedule HTML")
		}
		log.Debug().Msgf("Saved schedule HTML to %s", filename)
	}

	return html
}

func generateTableBody(rows []RawScheduleRow) string {
	tableBody := ""

	for _, row := range rows {
		tableBody += fmt.Sprintf(
			`<tr>
				<td class="side_column_number">%d</td>
				<td class="side_column_time">%s</td>
				%s
			</tr>`,
			row.Number, strings.ReplaceAll(row.TimeRange, "-", "<hr>"), generateRowPairs(row.Days))
	}
	return tableBody
}

func generateRowPairs(days []RawScheduleDay) string {
	rowPairs := ""

	for _, day := range days {
		cssClass := day.Pair.Kind
		if day.Pair.Replaced {
			cssClass += " replaced"
		}

		switch day.Pair.Kind {
		case PairKindEvent, PairKindVacation, PairKindSession, PairKindPractice, PairKindIGA:
			rowPairs += fmt.Sprintf(`<td class="%s"><span>%s</span></td>`, cssClass, day.Pair.Label)
		case PairKindExam, PairKindConsultation:
			rowPairs += fmt.Sprintf(
				`<td class='%s'>
					<span class='title'>%s</span><br>
					<hr>
					<span class='discipline'>%s</span><br> <br>
					<span class='teacher'>%s</span><br>
					<span class='classroom'>%s</span><br>
				</td>`,
				cssClass, *day.Pair.Title, *day.Pair.Discipline, *day.Pair.Teacher, *day.Pair.Classroom)
		case PairKindSubject:
			secondLine := ""
			if day.Pair.Group != nil {
				secondLine = fmt.Sprintf("<span class='group'>%s</span>", *day.Pair.Group)
			} else if day.Pair.Teacher != nil {
				secondLine = fmt.Sprintf("<span class='teacher'>%s</span>", *day.Pair.Teacher)
			}

			rowPairs += fmt.Sprintf(
				`<td class='%s'>
					<span class='discipline'>%s</span><br>
					<br>
					%s<br>
					<span class='classroom'>%s</span><br>
				</td>`,
				cssClass, *day.Pair.Discipline, secondLine, *day.Pair.Classroom)
		default:
			label := ""
			if day.Pair.Replaced {
				label = "Снято"
			}
			rowPairs += fmt.Sprintf(`<td class="%s"><span>%s</span></td>`, cssClass, label)
		}
		rowPairs += "\n"
	}

	return rowPairs
}

type ScheduleDay struct {
	Date     string
	Week     int
	WeekKind string
	Pairs    []Pair
}

type Schedule struct {
	config ScheduleConfig
	Days   []ScheduleDay
}

func GroupScheduleConfig(group *database.Group) ScheduleConfig {
	return ScheduleConfig{Group: group}
}

func TeacherScheduleConfig(teacher *database.Teacher) ScheduleConfig {
	return ScheduleConfig{Teacher: teacher}
}

// loadTemplate loads schedule HTML template from file and caches it in memory.
func loadTemplate(cache *cache.Cache, templateFile string) string {
	if data, found := cache.C.Get("template"); found {
		return data.(string)
	}

	data, err := os.ReadFile(templateFile)
	if err != nil {
		log.Panic().Err(err).Str("filename", templateFile).Msg("Failed to load template file")
	}

	str := string(data)
	cache.C.Set("template", str, -1)
	return str
}
