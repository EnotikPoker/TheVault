package vault

import (
	"errors"
	"math/bits"
	"math/rand/v2"
	"sync"
)

var ErrNoValidVault = errors.New("vault: no valid rule sets")

var catalog = sync.OnceValue(buildCatalog)

// Generate returns a reproducible vault with exactly one solution.
func Generate(seed int64) (Vault, error) {
	choices := catalog()
	if len(choices) == 0 {
		return Vault{}, ErrNoValidVault
	}
	rng := rand.New(rand.NewPCG(uint64(seed), 0))
	rules := choices[rng.IntN(len(choices))]
	rng.Shuffle(len(rules), func(i, j int) {
		rules[i], rules[j] = rules[j], rules[i]
	})
	return Vault{rules: rules}, nil
}

type candidate struct {
	rule   rule
	passed uint32
}

func buildCatalog() [][SystemCount]rule {
	rules := buildCandidates()
	var choices [][SystemCount]rule
	const allSwitches = (1 << SwitchCount) - 1
	for i, a := range rules {
		for j := i + 1; j < len(rules); j++ {
			b := rules[j]
			ab := a.passed & b.passed
			if bits.OnesCount32(ab) <= 1 {
				continue
			}
			for k := j + 1; k < len(rules); k++ {
				c := rules[k]
				if a.rule.switches|b.rule.switches|c.rule.switches != allSwitches {
					continue
				}
				if bits.OnesCount32(ab&c.passed) != 1 {
					continue
				}
				// Removing any indicator must introduce another solution.
				if bits.OnesCount32(a.passed&c.passed) <= 1 || bits.OnesCount32(b.passed&c.passed) <= 1 {
					continue
				}
				choices = append(choices, [SystemCount]rule{a.rule, b.rule, c.rule})
			}
		}
	}
	return choices
}

func buildCandidates() []candidate {
	var rules []candidate
	seen := make(map[uint32]bool)
	add := func(r rule) {
		var passed uint32
		for state := range 1 << SwitchCount {
			if r.matches(uint8(state)) {
				passed |= 1 << state
			}
		}
		if !seen[passed] {
			seen[passed] = true
			rules = append(rules, candidate{rule: r, passed: passed})
		}
	}

	for switches := uint8(1); switches < 1<<SwitchCount; switches++ {
		if bits.OnesCount8(switches) == 2 {
			add(rule{kind: equal, switches: switches})
			add(rule{kind: different, switches: switches})
		}
	}
	for switches := uint8(1); switches < 1<<SwitchCount; switches++ {
		size := bits.OnesCount8(switches)
		if size >= 2 && size <= 4 {
			for target := 1; target < size; target++ {
				add(rule{kind: exactCount, switches: switches, target: uint8(target)})
			}
		}
	}
	for switches := uint8(1); switches < 1<<SwitchCount; switches++ {
		size := bits.OnesCount8(switches)
		if size >= 3 && size <= 4 {
			add(rule{kind: parity, switches: switches, target: 0})
			add(rule{kind: parity, switches: switches, target: 1})
		}
	}
	return rules
}
