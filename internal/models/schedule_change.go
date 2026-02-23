package models

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/pkg/utils"
)

func NewScheduleChange(old, new ScheduleData) *ScheduleChange {
	if !old.Config.IsEqual(&new.Config) {
		return nil
	}
	old, new = Synchronize(old, new)
	return &ScheduleChange{old, new}
}

func Synchronize(old, new ScheduleData) (ScheduleData, ScheduleData) {
	if len(old.Days) == 0 || len(new.Days) == 0 {
		return old, new
	}
	if old.Days[0].Date != new.Days[0].Date {
		// Shift old schedule one day forward
		old.Days = old.Days[1:]
	}
	length := min(len(old.Days), len(new.Days))
	old.Days = old.Days[:length]
	new.Days = new.Days[:length]
	return old, new
}

type ScheduleChange struct{ Old, New ScheduleData }

func (s *ScheduleChange) Diffs() []Diff {
	var absDiffs []Diff
	for d := range s.Old.Days {
		for p := range s.Old.Days[d].Pairs {
			newDay := s.New.Days[d]
			oldPair := s.Old.Days[d].Pairs[p]
			newPair := s.New.Days[d].Pairs[p]
			if !reflect.DeepEqual(oldPair, newPair) {
				absDiffs = append(absDiffs, Diff{newDay: &newDay, oldPair: &oldPair, newPair: &newPair})
			}
		}
	}
	// TODO: Detect relative changes.

	return absDiffs
}

func (s *ScheduleChange) String() string {
	diffs := s.Diffs()
	if len(diffs) == 0 {
		log.Warn().Msg("No changes detected")
		return "Изменения не обнаружены"
	}

	// Sort diffs by date and pair number
	sort.Slice(diffs, func(i, j int) bool {
		if diffs[i].Day().Date == diffs[j].Day().Date {
			return diffs[i].Number() < diffs[j].Number()
		}

		date1, err := time.Parse("2006-01-02", diffs[i].Day().Date)
		if err != nil {
			log.Panic().Err(err).Msg("Failed to parse date")
		}
		date2, err := time.Parse("2006-01-02", diffs[j].Day().Date)
		if err != nil {
			log.Panic().Err(err).Msg("Failed to parse date")
		}

		return date1.Before(date2)
	})

	var text strings.Builder
	fmt.Fprintf(&text, "Изменения в расписании группы %s:", s.New.Config.Group.GroupName)
	currentDate := ""
	for _, diff := range diffs {
		if currentDate != diff.Day().Date {
			currentDate = diff.Day().Date
			fmt.Fprintf(&text, "\n\n%s: ", diff.Day().DateString())
		}
		fmt.Fprintf(&text, "\n\n%s", diff.String())
	}
	return text.String()
}

type Diff struct {
	oldDay  *ScheduleDay
	oldPair *Pair
	newDay  *ScheduleDay
	newPair *Pair
}

func (d *Diff) Day() *ScheduleDay {
	if d.newDay != nil {
		return d.newDay
	} else if d.oldDay != nil {
		return d.oldDay
	} else {
		panic("Diff must have at least one day instanse")
	}
}

func (d *Diff) Number() int {
	if d.newPair != nil {
		return d.newPair.Number
	} else if d.oldPair != nil {
		return d.oldPair.Number
	} else {
		log.Panic().Msg("Diff must have at least one pair instanse")
		panic("")
	}
}

func (d *Diff) String() string {
	if d.oldDay == nil && d.oldPair == nil {
		return fmt.Sprintf("Добавлено:\n%s", d.newPair.String()) // Pair added
	} else if d.newDay == nil && d.newPair == nil {
		return fmt.Sprintf("Удалено:\n%s", d.oldPair.String()) // Pair removed
	} else if d.oldDay != nil && d.newDay != nil && !d.oldDay.IsEqual(d.newDay) {
		// Pair moved to other day
		log.Warn().Msg("Schedule difference case not yet implemented: Pair moved to other day")
		// TODO: Implement this case later
		return "<not implemented yet>"
	} else if d.oldPair != nil && d.newPair != nil {
		// Pair moved with same day
		text := ""
		isOrderChanged := d.oldPair.Number != d.newPair.Number
		isClassroomChanged := d.oldPair.Classroom != d.newPair.Classroom
		if isOrderChanged && isClassroomChanged {
			text = fmt.Sprintf("_Перенесено с %s:_\n", d.oldPair.TimeSlotCabinetString())
		} else if isOrderChanged {
			text = fmt.Sprintf("_Перенесено с %s:_\n", d.oldPair.TimeSlotString())
		} else if isClassroomChanged {
			text = fmt.Sprintf("_Перенесено из кабинета %s:_\n", d.oldPair.Classroom)
		}
		return text + pairChangeString(d.oldPair, d.newPair)
	} else {
		return fmt.Sprintf("Заменено:\n%s", d.newPair.String())
	}
}

func pairChangeString(old, new *Pair) string {
	isDiscChanged := old.Discipline != new.Discipline
	isTeacherChanged := old.Teacher != nil && new.Teacher != nil && old.Teacher != new.Teacher

	text := new.TimeSlotCabinetString()
	switch new.Kind {
	case PairKindSubject:
		if isDiscChanged {
			text += fmt.Sprintf("\n    ~~%s~~ _%s_",
				bot.EscapeMarkdown(old.Discipline), bot.EscapeMarkdown(new.Discipline))
		}
		if isTeacherChanged {
			text += fmt.Sprintf("\n    ~~%s~~ _%s_",
				bot.EscapeMarkdown(utils.DerefOrTypeDefault(old.Teacher)),
				bot.EscapeMarkdown(utils.DerefOrTypeDefault(new.Teacher)),
			)
		}
	case PairKindExam, PairKindConsultation:
		text += fmt.Sprintf("\n    _%s_", bot.EscapeMarkdown(new.Label))
		if isDiscChanged {
			text += fmt.Sprintf("\n    ~~%s~~ _%s_",
				bot.EscapeMarkdown(old.Discipline), bot.EscapeMarkdown(new.Discipline))
		}
		if isTeacherChanged {
			text += fmt.Sprintf("\n    ~~%s~~ _%s_",
				utils.DerefOrTypeDefault(old.Teacher), utils.DerefOrTypeDefault(new.Teacher))
		}
	default:
		panic("unimplemented")
	}

	return text
}
