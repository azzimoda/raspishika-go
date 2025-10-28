package browser

import (
	"errors"
	"fmt"

	"github.com/azzimoda/raspishika-go/internal/config"

	"github.com/playwright-community/playwright-go"
)

// TODO: Implement regular restart.

type BrowserService struct {
	pw      *playwright.Playwright
	browser playwright.Browser
}

func (b *BrowserService) Close() error {
	browserErr := b.browser.Close()
	pwErr := b.pw.Stop()

	if browserErr != nil || pwErr != nil {
		return fmt.Errorf("Browser services closed with errors: %w", errors.Join(browserErr, pwErr))
	}
	return nil
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

func (b *BrowserService) TakeScreenshotHTML(html string, filename string) error {
	return b.WithPage(func(p playwright.Page) error {
		if err := p.SetContent(html); err != nil {
			return fmt.Errorf("Failed to set content: %w", err)
		}

		if _, err := p.Screenshot(playwright.PageScreenshotOptions{
			Path:     playwright.String(filename),
			FullPage: playwright.Bool(true),
			Type:     playwright.ScreenshotTypePng,
		}); err != nil {
			return fmt.Errorf("Failed to take screenshot: %w", err)
		}
		return nil
	})
}

func New(cfg *config.Config) (*BrowserService, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: &cfg.Browser.Headless,
		Timeout:  playwright.Float(float64(cfg.Browser.Timeout) * 1000),
	})
	if err != nil {
		return nil, err
	}

	return &BrowserService{pw: pw, browser: browser}, nil
}
