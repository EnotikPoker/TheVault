package vault

import (
	"fmt"
	"testing"
)

func exampleVault() Vault {
	return Vault{rules: [SystemCount]rule{
		{kind: different, switches: 0b00101},
		{kind: equal, switches: 0b01010},
		{kind: exactCount, switches: 0b11001, target: 2},
	}}
}

func TestCombinationSwitchOrder(t *testing.T) {
	for index := range SwitchCount {
		var combination Combination
		combination[index] = true
		if got, want := combination.mask(), uint8(1<<index); got != want {
			t.Errorf("S%d mask = %05b, want %05b", index+1, got, want)
		}
	}
}

func TestVaultCheck(t *testing.T) {
	v := exampleVault()
	tests := []struct {
		name        string
		combination Combination
		passed      [SystemCount]bool
		opened      bool
	}{
		{"all off", Combination{}, [SystemCount]bool{false, true, false}, false},
		{"all on", Combination{true, true, true, true, true}, [SystemCount]bool{false, true, false}, false},
		{"power fails", Combination{true, false, true, false, true}, [SystemCount]bool{false, true, true}, false},
		{"sync fails", Combination{true, true, false, false, true}, [SystemCount]bool{true, false, true}, false},
		{"access fails", Combination{true, false, false, false, false}, [SystemCount]bool{true, true, false}, false},
		{"opened", Combination{true, false, false, false, true}, [SystemCount]bool{true, true, true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.Check(tt.combination)
			want := CheckResult{
				Indicators: [SystemCount]IndicatorResult{
					{ID: SystemPower, Passed: tt.passed[0]},
					{ID: SystemSync, Passed: tt.passed[1]},
					{ID: SystemAccess, Passed: tt.passed[2]},
				},
				Opened: tt.opened,
			}
			if got != want {
				t.Errorf("Check(%v) = %+v, want %+v", tt.combination, got, want)
			}
		})
	}
}

func TestVaultChecksAreIndependent(t *testing.T) {
	v := exampleVault()
	before := v
	var results [1 << SwitchCount]CheckResult
	for state := range results {
		results[state] = v.Check(combinationFromState(state))
	}
	for state := len(results) - 1; state >= 0; state-- {
		if got := v.Check(combinationFromState(state)); got != results[state] {
			t.Errorf("result changed for state %05b", state)
		}
	}
	if v != before {
		t.Fatal("checking combinations changed the vault")
	}
	for state, want := range results {
		t.Run(fmt.Sprintf("concurrent/%05b", state), func(t *testing.T) {
			t.Parallel()
			for range 100 {
				if got := v.Check(combinationFromState(state)); got != want {
					t.Fatal("concurrent check returned a different result")
				}
			}
		})
	}
}

func TestZeroVaultStaysClosed(t *testing.T) {
	for state := range 1 << SwitchCount {
		got := (Vault{}).Check(combinationFromState(state))
		if got.Opened {
			t.Fatalf("zero vault opened for state %05b", state)
		}
		for _, indicator := range got.Indicators {
			if indicator.Passed {
				t.Fatalf("zero vault passed %s for state %05b", indicator.ID, state)
			}
		}
	}
}

func combinationFromState(state int) Combination {
	var combination Combination
	for index := range combination {
		combination[index] = state&(1<<index) != 0
	}
	return combination
}
