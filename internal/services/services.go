package services

import (
	"github.com/azzimoda/raspishika-go/internal/adminbot/reporter"
	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"
)

type Services struct {
	Repo     *database.Repository
	Browser  *browser.BrowserService
	Cache    *cache.Cache
	Reporter reporter.Reporter
}
