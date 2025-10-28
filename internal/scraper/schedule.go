package scraper

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

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

func (sc *ScheduleConfig) FormatMarkdown() string {
	switch {
	case sc.Group != nil:
		return "Расписание группы: *" + tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, sc.Group.GroupName) + "*"
	case sc.Teacher != nil:
		return "Расписание преподавателя: *" + tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, sc.Teacher.Name) + "*"
	default:
		return "?"
	}
}

type Pair struct {
	Kind       PairKind `json:"kind"`
	Number     int      `json:"number"`
	StartTime  string   `json:"start_time"`
	EndTime    string   `json:"end_time"`
	Label      string   `json:"label"`
	Title      string   `json:"title"`
	Discipline string   `json:"discipline"`
	Teacher    *string  `json:"teacher"`
	Group      *string  `json:"group"`
	Classroom  string   `json:"classroom"`
	Replaced   bool     `json:"replaced"`
}

func (p Pair) String() string {
	discipline := utils.EscapeMarkdown(p.Discipline)
	timeRange := utils.EscapeMarkdown(p.StartTime + "-" + p.EndTime)
	classroom := utils.EscapeMarkdown(p.Classroom)

	switch p.Kind {
	case PairKindSubject:
		return fmt.Sprintf("%d \\| %s \\| %s\n    *%s*\n    %s",
			p.Number, timeRange, classroom, discipline, *p.Teacher)
	case PairKindExam, PairKindConsultation:
		return fmt.Sprintf("%d \\| %s \\| %s\n    _%s_\n    *%s*\n    %s",
			p.Number, timeRange, classroom, p.Label, discipline, *p.Teacher)
	default:
		return fmt.Sprintf("%d \\| %s — %s", p.Number, timeRange, p.Label)
	}
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
	Days      [7]RawScheduleDay
}

type RawSchedule struct {
	Config ScheduleConfig
	Rows   []RawScheduleRow
}

func (s *RawSchedule) Transform() Schedule {
	schedule := Schedule{
		Config: s.Config,
		Days:   [7]ScheduleDay{},
	}

	for di := range len(s.Rows[0].Days) {
		schedule.Days[di].Date = s.Rows[0].Days[di].Date
		schedule.Days[di].WeekDay = s.Rows[0].Days[di].WeekDay
		schedule.Days[di].WeekKind = s.Rows[0].Days[di].WeekKind

		for ri := 0; ri < len(s.Rows); ri++ {
			schedule.Days[di].Pairs[ri] = s.Rows[ri].Days[di].Pair
			schedule.Days[di].Pairs[ri].Number = s.Rows[ri].Number
			parts := strings.Split(s.Rows[ri].TimeRange, "-")
			schedule.Days[di].Pairs[ri].StartTime = parts[0]
			schedule.Days[di].Pairs[ri].EndTime = parts[1]
		}
	}

	// log.Trace().Msgf("Transformed schedule: %#v", schedule)
	return schedule
}

// HTML returns HTML representation of the schedule.
//
// TODO: Provide tamplate as a parameter instead of fetching it in this function.
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
		"TABLE_BODY", s.generateTableBody(s.Rows),
		"TIMESTAMP", time.Now().Format(time.RFC3339),
	).Replace(template)

	if log.Logger.GetLevel() <= zerolog.DebugLevel {
		if err := os.MkdirAll("storage/cache/", 0755); err != nil {
			log.Error().Err(err).Msg("Failed to create cache directory")
		}
		filename := scheduleCacheFileName(s.Config)
		if err := os.WriteFile(filename, []byte(html), 0644); err != nil {
			log.Error().Err(err).Msg("Failed to save schedule HTML")
		}
		log.Debug().Msgf("Saved schedule HTML to %s", filename)
	}

	return html
}

func scheduleCacheFileName(cfg ScheduleConfig) string {
	if cfg.Group != nil {
		return "storage/cache/schedule_" + cfg.Group.GroupName + ".html"
	} else if cfg.Teacher != nil {
		return "storage/cache/schedule_" + cfg.Teacher.Name + ".html"
	} else {
		panic("unreachable")
	}
}

func (s *RawSchedule) generateTableBody(rows []RawScheduleRow) string {
	tableBody := ""

	for _, row := range rows {
		tableBody += fmt.Sprintf(
			`<tr>
				<td class="side_column_number">%d</td>
				<td class="side_column_time">%s</td>
				%s
			</tr>`,
			row.Number, strings.ReplaceAll(row.TimeRange, "-", "<hr>"), s.generateRowPairs(row.Days))
	}
	return tableBody
}

func (s *RawSchedule) generateRowPairs(days [7]RawScheduleDay) string {
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
				cssClass, day.Pair.Title, day.Pair.Discipline, *day.Pair.Teacher, day.Pair.Classroom)
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
				cssClass, day.Pair.Discipline, secondLine, day.Pair.Classroom)
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
	Date     string  `json:"date"`
	WeekDay  string  `json:"week_day"`
	WeekKind string  `json:"week_kind"`
	Pairs    [7]Pair `json:"pairs"`
}

func (s ScheduleDay) DetectOneKind() *PairKind {
	kind := s.Pairs[0].Kind
	for _, pair := range s.Pairs {
		if pair.Kind != kind {
			return nil
		}
	}
	return &kind
}

func (s ScheduleDay) IsEmpty() bool {
	k := s.DetectOneKind()
	return k != nil && *k == PairKindEmpty
}

func (s ScheduleDay) Left() ScheduleDay {
	leftSchedule := ScheduleDay{Date: s.Date, WeekDay: s.WeekDay, WeekKind: s.WeekKind, Pairs: [7]Pair{}}

	now := time.Now()
	p, err := s.CurrentPair(now)
	if err != nil {
		if now.Before(time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, now.Location())) {
			return s
		}
		p = &Pair{Number: 8}
	}

	for i := range 7 {
		if i < p.Number-1 {
			leftSchedule.Pairs[i].Kind = PairKindEmpty
		} else {
			leftSchedule.Pairs[i] = s.Pairs[i]
		}
	}

	return leftSchedule
}

func (s ScheduleDay) CurrentPair(t time.Time) (*Pair, error) {
	for _, pair := range s.Pairs {
		startTime, err := time.Parse("15:04", strings.TrimSpace(pair.StartTime))
		if err != nil {
			return nil, fmt.Errorf("failed to parse pair start time: %w", err)
		}

		endTime, err := time.Parse("15:04", strings.TrimSpace(pair.EndTime))
		if err != nil {
			return nil, fmt.Errorf("failed to parse pair end time: %w", err)
		}

		switch pair.Number {
		case 1:
			// This pair goes first; it needs a little shifting to be catched.
			startTime = startTime.Add(-1 * time.Minute)
		case 2, 3, 5, 6, 7:
			// These pairs go after 10-minute breaks.
			startTime = startTime.Add(-10 * time.Minute)
		case 4:
			// This pair goes after the big break, which is 45 minutes long.
			startTime = startTime.Add(-45 * time.Minute)
		}

		if t.After(startTime) && t.Before(endTime) {
			return &pair, nil
		}
	}

	return nil, fmt.Errorf("all pairs passed")
}

func (s ScheduleDay) String() string {
	text := fmt.Sprintf("📅 %s, %s: ", s.WeekDay, utils.EscapeMarkdown(s.Date))

	if kind := s.DetectOneKind(); kind != nil {
		log.Trace().Msgf("Detected one kind: %s", *kind)
		if *kind == PairKindEmpty {
			text += "Нет пар"
		} else {
			text += s.Pairs[0].Label
		}
		return text
	}

	// log.Trace().Msgf("Pairs: %d", len(s.Pairs))
	for _, pair := range s.Pairs {
		// log.Trace().Msgf("Pair: %+v", pair)
		if pair.Kind == PairKindEmpty {
			continue
		}
		text += "\n\n" + pair.String()
	}
	return text
}

type Schedule struct {
	Config ScheduleConfig `json:"config"`
	Days   [7]ScheduleDay `json:"days"`
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
