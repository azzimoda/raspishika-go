package main

import (
	"github.com/azzimoda/raspishika-go/internal/model"
)

func main() {
	cases := []model.Diff{
		{
			OldPair: model.Pair{
				Number:    1,
				StartTime: "8:00",
				EndTime:   "9:35",
				Kind:      model.PairKindEmpty,
			},
			NewPair: model.Pair{
				Number:     1,
				StartTime:  "8:00",
				EndTime:    "9:35",
				Kind:       model.PairKindSubject,
				Discipline: "Test",
				Teacher:    new("Tester"),
				Classroom:  "42",
			},
		},
		{
			OldPair: model.Pair{
				Number:     1,
				StartTime:  "8:00",
				EndTime:    "9:35",
				Kind:       model.PairKindSubject,
				Discipline: "Test",
				Teacher:    new("Tester"),
				Classroom:  "42",
			},
			NewPair: model.Pair{
				Number:    1,
				StartTime: "8:00",
				EndTime:   "9:35",
				Kind:      model.PairKindEmpty,
			},
		},
		{
			OldPair: model.Pair{
				Number:     1,
				StartTime:  "8:00",
				EndTime:    "9:35",
				Kind:       model.PairKindSubject,
				Discipline: "Test",
				Teacher:    new("Tester"),
				Classroom:  "42",
			},
			NewPair: model.Pair{
				Number:     1,
				StartTime:  "8:00",
				EndTime:    "9:35",
				Kind:       model.PairKindSubject,
				Discipline: "Test 2",
				Teacher:    new("Tester"),
				Classroom:  "42",
			},
		},
		{
			OldPair: model.Pair{
				Number:     1,
				StartTime:  "8:00",
				EndTime:    "9:35",
				Kind:       model.PairKindSubject,
				Discipline: "Test",
				Teacher:    new("Tester"),
				Classroom:  "42",
			},
			NewPair: model.Pair{
				Number:     1,
				StartTime:  "8:00",
				EndTime:    "9:35",
				Kind:       model.PairKindSubject,
				Discipline: "Test",
				Teacher:    new("Tester 2"),
				Classroom:  "42",
			},
		},
		{
			OldPair: model.Pair{
				Number:     1,
				StartTime:  "8:00",
				EndTime:    "9:35",
				Kind:       model.PairKindSubject,
				Discipline: "Test",
				Teacher:    new("Tester"),
				Classroom:  "42",
			},
			NewPair: model.Pair{
				Number:     1,
				StartTime:  "8:00",
				EndTime:    "9:35",
				Kind:       model.PairKindSubject,
				Discipline: "Test",
				Teacher:    new("Tester"),
				Classroom:  "54",
			},
		},
		{
			OldPair: model.Pair{
				Number:     1,
				StartTime:  "8:00",
				EndTime:    "9:35",
				Kind:       model.PairKindSubject,
				Discipline: "Test",
				Teacher:    new("Tester"),
				Classroom:  "42",
			},
			NewPair: model.Pair{
				Number:     1,
				StartTime:  "8:00",
				EndTime:    "9:35",
				Kind:       model.PairKindSubject,
				Discipline: "Test 2",
				Teacher:    new("Tester 2"),
				Classroom:  "54",
			},
		},
	}

	for _, d := range cases {
		println(d.HTML())
	}
}
