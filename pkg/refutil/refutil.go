package refutil

func DerefOrTypeDefault[T any](s *T) T {
	var val T
	if s == nil {
		return val
	} else {
		return *s
	}
}
