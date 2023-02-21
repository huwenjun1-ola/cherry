package timer

import (
	"runtime/debug"
	"time"

	cherryLogger "github.com/cherry-game/cherry/logger"
)

type IEasyTimer interface {
	AfterFunc(d time.Duration, cb func(*Timer))
	CronFunc(cronExpr *CronExpr, cb func(cron *Cron))
	TickerFunc(d time.Duration, cb func(ticker *Ticker))
	Destroy()
}
type EasyTimer struct {
	dispatcher *Dispatcher //timer

	mapActiveTimer map[ITimer]struct{}
}

func NewEasyTimer() *EasyTimer {
	return &EasyTimer{
		mapActiveTimer: make(map[ITimer]struct{}),
		dispatcher:     NewDispatcher(1000),
	}
}

func (s *EasyTimer) GetDispatcher() *Dispatcher {
	return s.dispatcher
}
func (s *EasyTimer) OnCloseTimer(t ITimer) {
	delete(s.mapActiveTimer, t)
}

func (s *EasyTimer) OnAddTimer(t ITimer) {
	if t != nil {
		if s.mapActiveTimer == nil {
			s.mapActiveTimer = map[ITimer]struct{}{}
		}

		s.mapActiveTimer[t] = struct{}{}
	}
}
func (s *EasyTimer) AfterFunc(d time.Duration, cb func(*Timer)) {
	s.dispatcher.AfterFunc(d, nil, cb, s.OnCloseTimer, s.OnAddTimer)
}
func (s *EasyTimer) CronFunc(cronExpr *CronExpr, cb func(cron *Cron)) {
	s.dispatcher.CronFunc(cronExpr, nil, cb, s.OnCloseTimer, s.OnAddTimer)

}
func (s *EasyTimer) TickerFunc(d time.Duration, cb func(ticker *Ticker)) {
	safeFun := func(ticker *Ticker) {
		defer func() {
			if err := recover(); err != nil {
				cherryLogger.Errorf("%v", err)
				stack := string(debug.Stack())
				cherryLogger.Error(stack)
			}
		}()
		cb(ticker)
	}
	s.dispatcher.TickerFunc(d, nil, safeFun, s.OnCloseTimer, s.OnAddTimer)
}

func (s *EasyTimer) Destroy() {
	for pTimer := range s.mapActiveTimer {
		pTimer.Cancel()
	}

	s.mapActiveTimer = nil
}
