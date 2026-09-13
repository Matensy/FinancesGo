package finance

import (
	"time"

	"github.com/Matensy/FinancesGo/internal/store"
)

// GoalTracker holds the daily earnings goal state (with monthly rollover) plus
// weekly and monthly totals.
type GoalTracker struct {
	Enabled bool
	Goal    float64 // base daily goal

	EarnedToday  float64
	TargetToday  float64 // goal adjusted by the accumulated surplus/deficit this month
	MissingToday float64 // still needed today (0 if done)
	DoneToday    bool

	WeekEarned float64 // last 7 days
	WeekTarget float64 // goal * 7

	MonthEarned  float64
	MonthTarget  float64 // goal * days elapsed this month
	MonthBalance float64 // MonthEarned - MonthTarget (>0 ahead, <0 behind)
	Ahead        bool
	BalanceAbs   float64

	DaysElapsed int
	DaysInMonth int

	Days      []store.DayEarning // last 30 days series
	BestDay   float64
	AvgPerDay float64 // month average
}

// Goals computes the earnings goal tracker for today.
func (e *Engine) Goals() (GoalTracker, error) {
	var g GoalTracker
	cfg, err := e.store.Settings()
	if err != nil {
		return g, err
	}
	g.Goal = cfg.DailyGoal
	g.Enabled = cfg.DailyGoal > 0

	today := startOfDay(time.Now())
	monthStart := startOfMonth(today)

	earnedToday, err := e.store.EarningsBetween(today, today)
	if err != nil {
		return g, err
	}
	monthEarned, err := e.store.EarningsBetween(monthStart, today)
	if err != nil {
		return g, err
	}
	earnedBefore := monthEarned - earnedToday // month up to yesterday
	weekFrom := today.AddDate(0, 0, -6)
	weekEarned, err := e.store.EarningsBetween(weekFrom, today)
	if err != nil {
		return g, err
	}

	// The rollover streak starts on the first day you actually sold this month,
	// so days before you started selling don't count as a deficit.
	streakStart := today
	if fd, ok, _ := e.store.FirstEarningDay(monthStart); ok && fd.Before(today) {
		streakStart = fd
	}
	daysActive := int(today.Sub(streakStart).Hours()/24) + 1
	if daysActive < 1 {
		daysActive = 1
	}

	g.DaysElapsed = today.Day()
	g.DaysInMonth = lastDayOfMonth(today)
	g.EarnedToday = round2(earnedToday)
	g.MonthEarned = round2(monthEarned)
	g.WeekEarned = round2(weekEarned)
	g.MonthTarget = round2(cfg.DailyGoal * float64(daysActive))
	g.WeekTarget = round2(cfg.DailyGoal * 7)
	g.MonthBalance = round2(monthEarned - g.MonthTarget)
	g.Ahead = g.MonthBalance >= 0
	g.BalanceAbs = round2(abs(g.MonthBalance))

	// Rollover: today's target = base goal adjusted by how far ahead/behind you
	// were before today. Behind -> more today; ahead -> less today.
	prevTarget := cfg.DailyGoal * float64(daysActive-1)
	net := earnedBefore - prevTarget // >0 ahead, <0 behind (rolls over)
	target := cfg.DailyGoal - net
	if target < 0 {
		target = 0
	}
	g.TargetToday = round2(target)
	miss := target - earnedToday
	if miss < 0 {
		miss = 0
	}
	g.MissingToday = round2(miss)
	g.DoneToday = miss <= 0 && g.Enabled

	if days, err := e.store.EarningsByDay(30); err == nil {
		g.Days = days
		for _, d := range days {
			if d.Amount > g.BestDay {
				g.BestDay = d.Amount
			}
		}
	}
	if g.DaysElapsed > 0 {
		g.AvgPerDay = round2(monthEarned / float64(g.DaysElapsed))
	}
	return g, nil
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
