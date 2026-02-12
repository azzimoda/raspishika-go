package browser

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/playwright-community/playwright-go"
)

const BrowserWindowWidth = 1280 // px

// TODO: Implement regular restart.

func New() (*BrowserService, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}

	isHeadless := viper.GetBool("browser.headless")

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(isHeadless),
		Timeout:  playwright.Float(float64(viper.GetInt("browser.timeout")) * 1000),
	})
	if err != nil {
		return nil, err
	}

	ctx, cancelExecAllocator := chromedp.NewExecAllocator(
		context.Background(),
		chromedp.Flag("headless", isHeadless),
		chromedp.WindowSize(BrowserWindowWidth, BrowserWindowWidth/2),
	)
	ctx, cancelChromeDP := chromedp.NewContext(ctx, chromedp.WithBrowserOption())

	cancel := func() {
		log.Warn().Msg("Cancelling chromedp contexts...")
		cancelExecAllocator()
		cancelChromeDP()
	}
	bs := BrowserService{pw: pw, pwBrowser: browser, chromedpCtx: ctx, chromedpCancel: cancel}

	return &bs, nil
}

type BrowserService struct {
	pw             *playwright.Playwright
	pwBrowser      playwright.Browser
	chromedpCtx    context.Context
	chromedpCancel context.CancelFunc
}

func (b *BrowserService) Close() error {
	browserErr := b.pwBrowser.Close()
	pwErr := b.pw.Stop()

	if browserErr != nil || pwErr != nil {
		return fmt.Errorf("Browser services closed with errors: %w", errors.Join(browserErr, pwErr))
	}

	b.chromedpCancel()

	return nil
}

func (b *BrowserService) WithContext(f func(playwright.BrowserContext) error) error {
	ctx, err := b.pwBrowser.NewContext()
	if err != nil {
		return fmt.Errorf("failed to create browser context: %w", err)
	}
	defer ctx.Close()

	return f(ctx)
}

func (b *BrowserService) WithPage(f func(playwright.Page) error) error {
	return b.WithContext(func(bc playwright.BrowserContext) error {
		page, err := bc.NewPage()
		if err != nil {
			return fmt.Errorf("failed to create page: %w", err)
		}
		defer page.Close()

		return f(page)
	})
}

func (b *BrowserService) TakeScreenshotHTML(html, outputFilename string) error {
	var ctx context.Context = b.chromedpCtx
	var imageData []byte
	screenshotElement := chromedp.Tasks{chromedp.Navigate("about:blank"), chromedp.ActionFunc(func(ctx context.Context) error {
		frameTree, err := page.GetFrameTree().Do(ctx)
		if err != nil {
			return err
		}
		return page.SetDocumentContent(frameTree.Frame.ID, html).Do(ctx)
	}), chromedp.FullScreenshot(&imageData, 100)}
	if err := chromedp.Run(ctx, screenshotElement); err != nil {
		return fmt.Errorf("failed to take screenshot: %w", err)
	}
	if err := os.WriteFile(outputFilename, imageData, 0644); err != nil {
		return fmt.Errorf("failed to write screenshot to file: %w", err)
	}
	return nil
}
