package browser

import (
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/playwright-community/playwright-go"
)

type BrowserService struct {
	pw      *playwright.Playwright
	browser playwright.Browser
}

func (b *BrowserService) WithContext(f func(playwright.BrowserContext) error) error {
	ctx, err := b.browser.NewContext()
	if err != nil {
		return err
	}
	defer ctx.Close()

	return f(ctx)
}

func (b *BrowserService) WithPage(f func(playwright.Page) error) error {
	return b.WithContext(func(bc playwright.BrowserContext) error {
		page, err := bc.NewPage()
		if err != nil {
			return err
		}
		defer page.Close()

		return f(page)
	})
}

func New(cfg *config.Config) (*BrowserService, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}

	browser, err := pw.Chromium.Launch()
	if err != nil {
		return nil, err
	}

	return &BrowserService{pw: pw, browser: browser}, nil
}
