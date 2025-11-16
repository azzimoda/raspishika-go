package sendings

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/azzimoda/raspishika-go/internal/mainbot/commands"
)

type SendingManager struct {
	cron *cron.Cron
	ch   *commands.CommandHandler
}

func (sm *SendingManager) Start() {
	sm.cron.Start()
}

func NewSendingManager(commandHandler *commands.CommandHandler) *SendingManager {
	return &SendingManager{cron: cron.New(), ch: commandHandler}
}

func (sm *SendingManager) ScheduleDailySending() error {
	_, err := sm.cron.AddFunc("* * * * *", func() {
		go sm.processDailySending(time.Now())
	})
	return err
}

func (sm *SendingManager) SchedulePairSending() error {
	times := [][2]int{
		{7, 45},  // 8:00
		{9, 30},  // 9:45
		{11, 15}, // 11:30
		// Big break, 40 minutes.
		{13, 30}, // 13:45
		{15, 15}, // 15:30
		{17, 00}, // 17:15
		{18, 45}, // 19:00
	}
	for _, t := range times {
		h := t[0]
		m := t[1]
		_, err := sm.cron.AddFunc(fmt.Sprintf("%d %d * * 1-6", m, h), func() {
			go sm.processPairSending(time.Now())
		})
		if err != nil {
			return err
		}
	}
	return nil
}

type sendingResult struct {
	chatsNum  int
	errs      []error
	failedAll bool
}
