package scraper

import (
	"fmt"
	"strings"
	"time"

	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/services/browser"
	"github.com/playwright-community/playwright-go"
	"github.com/rs/zerolog/log"
)

const TeachersPageURL = "https://mnokol.tyuiu.ru/site/index.php?option=com_content&view=article&id=1247&Itemid=304"

func FetchTeachers(repo *repository.Repository, browser *browser.BrowserService) ([]models.Teacher, error) {
	// Check existing cache
	if teachers, err := checkTeachers(repo); err == nil && len(teachers) > 0 {
		log.Debug().Msg("Using cached teachers")
		return teachers, nil
	}

	// Update cache
	log.Debug().Msg("Fetching teachers")

	teachers, err := scrapeTeachers(browser)
	if err != nil {
		return nil, err
	}

	if err := models.UpdateTeachers(repo.DB, teachers); err != nil {
		return nil, err
	}
	return teachers, nil
}

func checkTeachers(repo *repository.Repository) ([]models.Teacher, error) {
	teachers, err := models.GetTeachers(repo.DB)
	if err != nil {
		return nil, err
	}

	for _, t := range teachers {
		if time.Since(t.UpdatedAt) < 7*24*time.Hour {
			return teachers, nil
		}
	}
	return nil, fmt.Errorf("all teachers are outdated")
}

func scrapeTeachers(browser *browser.BrowserService) (teachers []models.Teacher, err error) {
	err = browser.WithPage(func(p playwright.Page) error {
		if _, err := p.Goto(TeachersPageURL); err != nil {
			return fmt.Errorf("failed to goto teachers page: %w", err)
		}

		iframeLocator := p.FrameLocator("div.com-content-article__body iframe")
		selectLocator := iframeLocator.Locator("#preps")
		if err := selectLocator.WaitFor(playwright.LocatorWaitForOptions{}); err != nil {
			return fmt.Errorf("failed to wait for teacher select: %w", err)
		}

		options, err := iframeLocator.Locator("#preps option").EvaluateAll(
			`els => els.map(el => ({ text: el.textContent.trim(), value: el.value }))`)
		if err != nil {
			return fmt.Errorf("failed to get teacher options: %w", err)
		}

		for _, opt := range options.([]any) {
			opt := opt.(map[string]any)
			if opt["value"] == nil || opt["text"] == nil {
				continue
			}

			teachers = append(teachers, models.Teacher{
				TeacherID: models.TeacherID(opt["value"].(string)),
				Name:      models.TeacherName(strings.TrimSpace(opt["text"].(string))),
			})
		}
		return nil
	})
	return
}
