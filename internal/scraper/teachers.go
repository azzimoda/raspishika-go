package scraper

import (
	"fmt"
	"time"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/playwright-community/playwright-go"
	"github.com/rs/zerolog/log"
)

const TeachersPageURL = "https://mnokol.tyuiu.ru/site/index.php?option=com_content&view=article&id=1247&Itemid=304"

func FetchTeachers(repo *database.Repository, browser *browser.BrowserService) ([]database.Teacher, error) {
	if teachers, err := checkTeachers(repo); err == nil && len(teachers) > 0 {
		log.Debug().Msg("Using cached teachers")
		return teachers, nil
	}
	log.Debug().Msg("Fetching teachers")

	teachers, err := scrapeTeachers(browser)
	if err != nil {
		return nil, err
	}

	if err := repo.UpdateTeachers(teachers); err != nil {
		return nil, err
	}
	return teachers, nil
}

func checkTeachers(repo *database.Repository) ([]database.Teacher, error) {
	teachers, err := repo.GetTeachers()
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

func scrapeTeachers(browser *browser.BrowserService) (teachers []database.Teacher, err error) {
	err = browser.WithPage(func(p playwright.Page) error {
		if _, err := p.Goto(TeachersPageURL); err != nil {
			return err
		}

		iframeLocator := p.FrameLocator("div.com-content-article__body iframe")
		selectLocator := iframeLocator.Locator("#preps")
		if err := selectLocator.WaitFor(playwright.LocatorWaitForOptions{}); err != nil {
			return err
		}

		options, err := iframeLocator.Locator("#preps option").EvaluateAll(
			`els => els.map(el => ({ text: el.textContent.trim(), value: el.value }))`)
		if err != nil {
			return err
		}

		for _, opt := range options.([]any) {
			opt := opt.(map[string]any)
			if opt["value"] == nil || opt["text"] == nil {
				continue
			}

			teachers = append(teachers, database.Teacher{TeacherID: opt["value"].(string), Name: opt["text"].(string)})
		}
		return nil
	})
	return
}
