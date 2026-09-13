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

	MonthEarned       float64
	MonthTarget       float64 // goal * days in the month (the full monthly goal)
	MonthTargetToDate float64 // goal * days elapsed so far (for pace comparison)
	MonthBalance      float64 // MonthEarned - MonthTargetToDate (>0 ahead, <0 behind)
	Ahead             bool
	BalanceAbs        float64

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
	// Full monthly goal (e.g. 100/day * 30 days = 3000) and the weekly goal.
	g.MonthTarget = round2(cfg.DailyGoal * float64(g.DaysInMonth))
	g.WeekTarget = round2(cfg.DailyGoal * 7)
	// Pace: what you should have by now, given the active streak, decides
	// whether you're ahead or behind.
	g.MonthTargetToDate = round2(cfg.DailyGoal * float64(daysActive))
	g.MonthBalance = round2(monthEarned - g.MonthTargetToDate)
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
