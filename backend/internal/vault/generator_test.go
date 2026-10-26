package vault

import (
	"fmt"
	"math"
	"testing"
)

func TestGenerateReproducible(t *testing.T) {
	for _, seed := range []int64{0, 1, -1, 42, math.MinInt64, math.MaxInt64} {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			first, err := Generate(seed)
			if err != nil {
				t.Fatal(err)
			}
			for range 10 {
				got, err := Generate(seed)
				if err != nil {
					t.Fatal(err)
				}
				if got != first {
					t.Fatal("same seed produced different rules or indicator assignments")
				}
			}
			assertPlayable(t, first, fmt.Sprintf("seed %d", seed))
		})
	}
}

func TestGenerateVariety(t *testing.T) {
	seen := make(map[Vault]bool)
	positions := make(map[ruleKind][SystemCount]bool)
	for seed := range int64(256) {
		v, err := Generate(seed)
		if err != nil {
			t.Fatal(err)
		}
		assertPlayable(t, v, fmt.Sprintf("seed %d", seed))
		seen[v] = true
		for index, r := range v.rules {
			assigned := positions[r.kind]
			assigned[index] = true
			positions[r.kind] = assigned
		}
	}
	if len(seen) < 2 {
		t.Fatal("all seeds produced the same vault")
	}
	for _, kind := range []ruleKind{equal, different, exactCount, parity} {
		if positions[kind] != [SystemCount]bool{true, true, true} {
			t.Errorf("rule kind %d did not appear on every indicator: %v", kind, positions[kind])
		}
	}
}

func TestGenerateConcurrent(t *testing.T) {
	for seed := range int64(32) {
		want, err := Generate(seed)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			t.Parallel()
			for range 20 {
				got, err := Generate(seed)
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Fatal("generating other vaults changed this seed's result")
				}
			}
		})
	}
}

func TestCatalogContainsOnlyPlayableVaults(t *testing.T) {
	choices := catalog()
	if len(choices) == 0 {
		t.Fatal("no valid rule sets")
	}
	for index, rules := range choices {
		assertPlayable(t, Vault{rules: rules}, fmt.Sprintf("catalog entry %d", index))
	}
}

func assertPlayable(t *testing.T, v Vault, label string) {
	t.Helper()
	var results [1 << SwitchCount]CheckResult
	solutions := 0
	var necessary [SystemCount]bool
	var passed [SystemCount]int
	for state := range results {
		result := v.Check(combinationFromState(state))
		results[state] = result
		if result.Opened {
			solutions++
		}
		for index, indicator := range result.Indicators {
			if indicator.Passed {
				passed[index]++
				continue
			}
			othersPassed := true
			for other, otherIndicator := range result.Indicators {
				if other != index && !otherIndicator.Passed {
					othersPassed = false
				}
			}
			if othersPassed {
				necessary[index] = true
			}
		}
	}
	if solutions != 1 {
		t.Fatalf("%s has %d solutions, want 1", label, solutions)
	}
	for index := range SystemCount {
		if !necessary[index] {
			t.Fatalf("%s: indicator %d adds no restriction", label, index)
		}
		if passed[index] == 0 || passed[index] == len(results) {
			t.Fatalf("%s: indicator %d is constant", label, index)
		}
	}
	for index := range SwitchCount {
		influential := false
		for state, result := range results {
			if result.Indicators != results[state^(1<<index)].Indicators {
				influential = true
				break
			}
		}
		if !influential {
			t.Fatalf("%s: S%d has no effect on any indicator", label, index+1)
		}
	}
}
