// Package vault implements the rules and generation of vault puzzles.
package vault

const (
	SwitchCount = 5
	SystemCount = 3
)

type SystemID string

const (
	SystemPower  SystemID = "power"
	SystemSync   SystemID = "sync"
	SystemAccess SystemID = "access"
)

// Combination holds switch positions in order from S1 to S5.
type Combination [SwitchCount]bool

func (c Combination) mask() uint8 {
	var mask uint8
	for index, enabled := range c {
		if enabled {
			mask |= 1 << index
		}
	}
	return mask
}

type IndicatorResult struct {
	ID     SystemID
	Passed bool
}

type CheckResult struct {
	Indicators [SystemCount]IndicatorResult
	Opened     bool
}

// Vault is an immutable set of rules. Its zero value stays closed.
type Vault struct {
	rules [SystemCount]rule
}

func (v Vault) Check(combination Combination) CheckResult {
	state := combination.mask()
	indicators := [SystemCount]IndicatorResult{
		{ID: SystemPower, Passed: v.rules[0].matches(state)},
		{ID: SystemSync, Passed: v.rules[1].matches(state)},
		{ID: SystemAccess, Passed: v.rules[2].matches(state)},
	}
	return CheckResult{
		Indicators: indicators,
		Opened:     indicators[0].Passed && indicators[1].Passed && indicators[2].Passed,
	}
}
