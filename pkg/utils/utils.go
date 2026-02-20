package utils

func DerefOrTypeDefault[T any](s *T) T {
	var val T
	if s == nil {
		return val
	} else {
		return *s
	}
}

func Every[T any](elems []T, predicate func(*T) bool) bool {
	for _, elem := range elems {
		if !predicate(&elem) {
			return false
		}
	}
	return true
}
