package models

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

func (sc *ScheduleConfig) FormatHTML() string {
	switch {
	case sc.Group != nil:
		return "Расписание группы — <i>" + sc.Group.GroupName + "</i>"
	case sc.Teacher != nil:
		return "Расписание преподавателя — <i>" + sc.Teacher.Name + "</i>"
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
