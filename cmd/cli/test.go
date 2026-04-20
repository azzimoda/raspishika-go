package main

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/service"
	"github.com/azzimoda/raspishika-go/internal/service/scraper"
	"github.com/azzimoda/raspishika-go/pkg/database"
)

func runTests() {
	viper.Set(config.KeyDatabaseFile, "database/test.db")
	defer func() {
		// Remove DB
		os.Remove("database/test.db")
	}()

	db, err := database.New()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create database")
	}
	defer db.Close()

	container := repository.NewContainer(db)

	services, err := service.New(container, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create services")
	}
	defer services.Close()

	// Fetch groups
	log.Info().Msg("Fetching groups...")
	groups, err := scraper.FetchGroups(container.Group, services.Browser)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch groups")
	}
	log.Info().Int("groups", len(groups)).Msg("Groups fetched successfully")

	// Fetch teachers
	log.Info().Msg("Fetching teachers...")
	teachers, err := scraper.FetchTeachers(container.Group, services.Browser)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch teachers")
	}
	log.Info().Int("teachers", len(teachers)).Msg("Teachers fetched successfully")

	// Fetch schedules of 10 random groups
	var groupsToFetch []model.Group
	if len(groups) > 10 {
		groupsToFetch = groups[:10]
	} else {
		groupsToFetch = groups
	}
	log.Info().Int("groups", len(groupsToFetch)).Msg("Fetching schedules...")
	for _, group := range groupsToFetch {
		schedule, err := services.Schedule.Get(model.GroupScheduleConfig(&group, false))
		if err != nil {
			log.Error().Err(err).Msg("Failed to fetch schedule")
			continue
		}
		log.Info().Str("group", group.GroupName.String()).Msg("Schedule fetched successfully")
		_ = schedule
	}
}
