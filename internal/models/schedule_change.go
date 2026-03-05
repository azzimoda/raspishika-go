package models

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

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

type ScheduleChange struct {
	Old ScheduleData `json:"old"`
	New ScheduleData `json:"new"`
}

func (s *ScheduleChange) Diffs() []Diff {
	var absDiffs []Diff
	for d := range s.Old.Days {
		for p := range s.Old.Days[d].Pairs {
			newDay := s.New.Days[d]
			oldPair := s.Old.Days[d].Pairs[p]
			newPair := s.New.Days[d].Pairs[p]
			if !reflect.DeepEqual(oldPair, newPair) {
				absDiffs = append(absDiffs, Diff{NewDay: &newDay, OldPair: &oldPair, NewPair: &newPair})
			}
		}
	}
	// TODO: Detect relative changes.

	return absDiffs
}

func (s *ScheduleChange) HTML() string {
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

		date1, err := time.Parse("02.01.2006", diffs[i].Day().Date.String())
		if err != nil {
			log.Panic().Err(err).Msg("Failed to parse date")
		}
		date2, err := time.Parse("02.01.2006", diffs[j].Day().Date.String())
		if err != nil {
			log.Panic().Err(err).Msg("Failed to parse date")
		}

		return date1.Before(date2)
	})

	var text strings.Builder
	fmt.Fprintf(&text, "Изменения в расписании группы %s:", s.New.Config.Group.GroupName)

	var currentDate Date
	for _, diff := range diffs {
		date := diff.Day().Date
		if currentDate != date {
			currentDate = date
			fmt.Fprintf(&text, "\n\n%s: ", diff.Day().DateHTML())
		}
		fmt.Fprintf(&text, "\n\n%s", diff.HTML())
	}

	return text.String()
}

type Diff struct {
	OldDay  *ScheduleDay `json:"old_day"`
	OldPair *Pair        `json:"old_pair"`
	NewDay  *ScheduleDay `json:"new_day"`
	NewPair *Pair        `json:"new_pair"`
}

func (d *Diff) Day() *ScheduleDay {
	if d.NewDay != nil {
		return d.NewDay
	} else if d.OldDay != nil {
		return d.OldDay
	} else {
		panic("Diff must have at least one day instanse")
	}
}
func (d *Diff) Number() int {
	if d.NewPair != nil {
		return d.NewPair.Number
	} else if d.OldPair != nil {
		return d.OldPair.Number
	} else {
		log.Panic().Msg("Diff must have at least one pair instanse")
		panic("")
	}
}

// HTML representation of the difference.
func (d *Diff) HTML() (result string) {
	if d.OldDay != nil && d.NewDay != nil && !d.OldDay.IsEqual(d.NewDay) {
		// Pair moved to other day
		log.Warn().Msg("Schedule difference case not yet implemented: Pair moved to other day")
		// TODO: Implement this case later.

		result = "?"
	} else if d.OldPair != nil && d.NewPair != nil {
		// Pair moved with same day
		text := ""

		isOrderChanged := d.OldPair.Number != d.NewPair.Number
		isDiscChanged := d.OldPair.Discipline != d.NewPair.Discipline
		isTeacherChanged := d.OldPair.Teacher != nil && d.NewPair.Teacher != nil &&
			*d.OldPair.Teacher != *d.NewPair.Teacher
		isClassroomChanged := d.OldPair.Classroom != d.NewPair.Classroom
		log.Trace().Bool("isOrderChanged", isOrderChanged).Bool("isDiscChanged", isDiscChanged).
			Bool("isClassroomChanged", isClassroomChanged).Send()

		if d.OldPair.Kind != PairKindEmpty && d.NewPair.Kind == PairKindEmpty && d.NewPair.Replaced {
			text = "<i>Снято:</i>"
		} else if d.OldPair.Kind == PairKindEmpty && d.NewPair.Kind != PairKindEmpty && d.NewPair.Replaced {
			text = "<i>Добавлено:</i>"
		} else if isDiscChanged || isTeacherChanged {
			text = "<i>Заменено:</i>"
		} else if isOrderChanged && isClassroomChanged {
			text = fmt.Sprintf("<i>Перенесено с %s:</i>", d.OldPair.TimeSlotCabinetString())
		} else if isOrderChanged {
			text = fmt.Sprintf("<i>Перенесено с %s:</i>", d.OldPair.TimeSlotString())
		} else if isClassroomChanged {
			text = fmt.Sprintf("<i>Перенесено из кабинета %s:</i>", d.OldPair.Classroom)
		}

		result = text + "\n" + pairChangeHTML(d.OldPair, d.NewPair)
	} else {
		// TODO: Ensure that this case is unreachable and remove it.
		result = fmt.Sprintf("Заменено:\n%s", d.NewPair.HTML())
	}
	log.Trace().Msgf("(Diff).String() => %v", result)
	return result
}

// pairChangeHTML returns formatted pair change string.
func pairChangeHTML(old, new *Pair) string {
	isDiscChanged := old.Discipline != new.Discipline
	isTeacherChanged := old.Teacher != nil && new.Teacher != nil && *old.Teacher != *new.Teacher

	text := ""
	switch new.Kind {
	case PairKindEmpty:
		text += fmt.Sprintf("\n<s>%s</s>", old.HTML())

	case PairKindSubject:
		if old.Kind == PairKindEmpty {
			text += fmt.Sprintf("\n%s", old.HTML())
		} else {
			text = new.TimeSlotCabinetString()
			if isDiscChanged {
				text += fmt.Sprintf("\n    <s>%s</s> <i>%s</i>",
					old.Discipline, new.Discipline)
			} else {
				text += fmt.Sprintf("\n    %s", new.Discipline)
			}

			if isTeacherChanged {
				text += fmt.Sprintf("\n    <s>%s</s> <i>%s</i>",
					utils.DerefOrTypeDefault(old.Teacher),
					utils.DerefOrTypeDefault(new.Teacher),
				)
			} else {
				text += fmt.Sprintf("\n    %s", utils.DerefOrTypeDefault(new.Teacher))
			}
		}

	case PairKindExam, PairKindConsultation:
		text = new.TimeSlotCabinetString()
		text += fmt.Sprintf("\n    _%s_", new.Label)
		if isDiscChanged {
			text += fmt.Sprintf("\n    <s>%s</s> <i>%s</i>",
				old.Discipline, new.Discipline)
		} else {
			text += fmt.Sprintf("\n    %s", new.Discipline)
		}
		if isTeacherChanged {
			text += fmt.Sprintf("\n    <s>%s</s> <i>%s</i>",
				utils.DerefOrTypeDefault(old.Teacher), utils.DerefOrTypeDefault(new.Teacher))
		} else {
			text += fmt.Sprintf("\n    %s", utils.DerefOrTypeDefault(new.Teacher))
		}

	default:
		text += fmt.Sprintf("\n<s>%s</s>\n%s", old.HTML(), new.HTML())
	}

	return text
}
