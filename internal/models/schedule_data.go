package models

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/pkg/utils"
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

type ScheduleData struct {
	Config ScheduleConfig `json:"config"`
	Days   []ScheduleDay  `json:"days"`
}

type ScheduleDay struct {
	Date     string `json:"date"`
	WeekDay  string `json:"week_day"`
	WeekKind string `json:"week_kind"`
	Pairs    []Pair `json:"pairs"`
}

func (s ScheduleDay) DetectOneKind() *PairKind {
	if len(s.Pairs) == 0 {
		kind := PairKindEmpty
		return &kind
	}

	kind := s.Pairs[0].Kind
	for _, pair := range s.Pairs {
		if pair.Kind != kind {
			return nil
		}
	}
	return &kind
}

func (s *ScheduleDay) IsEqual(other *ScheduleDay) bool {
	if s.Date != other.Date || s.WeekDay != other.WeekDay || s.WeekKind != other.WeekKind || len(s.Pairs) != len(other.Pairs) {
		return false
	}

	for i := range s.Pairs {
		if !reflect.DeepEqual(s.Pairs[i], other.Pairs[i]) {
			return false
		}
	}

	return true
}

func (s *ScheduleDay) IsEmpty() bool {
	k := s.DetectOneKind()
	return k != nil && *k == PairKindEmpty
}

func (s ScheduleDay) Left() ScheduleDay {
	leftSchedule := ScheduleDay{Date: s.Date, WeekDay: s.WeekDay, WeekKind: s.WeekKind, Pairs: []Pair{}}

	now := time.Now() // TODO: Add current time as a parameter.
	p, err := s.CurrentPair(now)
	if err != nil {
		if now.Before(time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, now.Location())) {
			return s
		}
		p = &Pair{Number: 8}
	}

	for i := range len(s.Pairs) {
		if i >= p.Number-1 {
			leftSchedule.Pairs = append(leftSchedule.Pairs, s.Pairs[i])
		}
	}

	return leftSchedule
}

func (s ScheduleDay) CurrentPair(t time.Time) (*Pair, error) {
	log.Trace().Time("time", t).Str("timeStr", t.String()).Msg("CurrentPair")
	year, month, day := t.Date()
	for _, pair := range s.Pairs {
		startTime, err := time.Parse("15:04", strings.TrimSpace(pair.StartTime))
		if err != nil {
			return nil, fmt.Errorf("failed to parse pair start time: %w", err)
		}

		endTime, err := time.Parse("15:04", strings.TrimSpace(pair.EndTime))
		if err != nil {
			return nil, fmt.Errorf("failed to parse pair end time: %w", err)
		}

		// Adjust date.
		startTime = time.Date(year, month, day, startTime.Hour(), startTime.Minute(), 0, 0, t.Location())
		endTime = time.Date(year, month, day, endTime.Hour(), endTime.Minute(), 0, 0, t.Location())

		// Adjust start time.
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
	text := s.DateString() + ": "

	if kind := s.DetectOneKind(); kind != nil {
		log.Trace().Msgf("Detected one kind: %s", *kind)
		if *kind == PairKindEmpty {
			text += "Нет пар"
		} else {
			text += s.Pairs[0].Label
		}
		return text
	}

	for _, pair := range s.Pairs {
		if pair.Kind == PairKindEmpty {
			continue
		}
		text += "\n\n" + pair.String()
	}
	return text
}

func (s *ScheduleDay) DateString() string {
	return fmt.Sprintf("📅 %s, %s", s.WeekDay, bot.EscapeMarkdown(s.Date))
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
	Subgroup   string   `json:"subgroup"`
	Classroom  string   `json:"classroom"`
	Replaced   bool     `json:"replaced"`
}

func (p Pair) String() string {
	log.Trace().Any("pair", p).Msg("Formating pair...")

	discipline := func() string { return bot.EscapeMarkdown(p.Discipline) }
	teacher := func() string { return bot.EscapeMarkdown(utils.DerefOrTypeDefault(p.Teacher)) }
	label := func() string { return bot.EscapeMarkdown(p.Label) }

	switch p.Kind {
	case PairKindSubject:
		return fmt.Sprintf("%s\n    *%s*\n    %s", p.TimeSlotCabinetString(), discipline(), teacher())
	case PairKindExam, PairKindConsultation:
		return fmt.Sprintf("%s\n    _%s_\n    *%s*\n    %s",
			p.TimeSlotCabinetString(), label(), discipline(), teacher())
	default:
		return fmt.Sprintf("%s — %s", p.TimeSlotString(), label())
	}
}
func (p *Pair) TimeSlotString() string {
	result := bot.EscapeMarkdown(fmt.Sprintf("%d | %s - %s", p.Number, p.StartTime, p.EndTime))
	log.Trace().Msgf("Formatted time slot string: %s", result)
	return result
}
func (p *Pair) TimeSlotCabinetString() string {
	result := bot.EscapeMarkdown(fmt.Sprintf("%d | %s - %s | %s", p.Number, p.StartTime, p.EndTime, p.Classroom))
	log.Trace().Msgf("Formatted time slot cabinet string: %s", result)
	return result
}
