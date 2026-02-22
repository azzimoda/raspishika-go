package models

import "github.com/go-telegram/bot"

func GroupScheduleConfig(group *Group) ScheduleConfig {
	return ScheduleConfig{Group: group}
}

func TeacherScheduleConfig(teacher *Teacher) ScheduleConfig {
	return ScheduleConfig{Teacher: teacher}
}

type ScheduleConfig struct {
	Group   *Group   `json:"group"`
	Teacher *Teacher `json:"teacher"`
}

func (sc *ScheduleConfig) FormatMarkdown() string {
	switch {
	case sc.Group != nil:
		return "Расписание группы — *" + bot.EscapeMarkdown(sc.Group.GroupName) + "*"
	case sc.Teacher != nil:
		return "Расписание преподавателя — *" + bot.EscapeMarkdown(sc.Teacher.Name) + "*"
	default:
		return "?"
	}
}

func (s *ScheduleConfig) IsEqual(other *ScheduleConfig) bool {
	if s.Group != nil && other.Group != nil {
		return s.Group.ID == other.Group.ID
	} else if s.Teacher != nil && other.Teacher != nil {
		return s.Teacher.ID == other.Teacher.ID
	} else if (*s == ScheduleConfig{}) && (*other == ScheduleConfig{}) {
		return true
	}
	return false
}
