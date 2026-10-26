package vault

import (
	"errors"
	"testing"
)

func TestRuleExplanations(t *testing.T) {
	tests := []struct {
		name        string
		rule        rule
		state       uint8
		condition   string
		observation string
	}{
		{
			name:        "equal on",
			rule:        rule{kind: equal, switches: 0b01010},
			state:       0b11111,
			condition:   "S2 и S4 должны совпадать.",
			observation: "S2 = ON, S4 = ON.",
		},
		{
			name:        "equal off",
			rule:        rule{kind: equal, switches: 0b01010},
			state:       0b10101,
			condition:   "S2 и S4 должны совпадать.",
			observation: "S2 = OFF, S4 = OFF.",
		},
		{
			name:        "different first on",
			rule:        rule{kind: different, switches: 0b00101},
			state:       0b00011,
			condition:   "S1 и S3 должны различаться.",
			observation: "S1 = ON, S3 = OFF.",
		},
		{
			name:        "different second on",
			rule:        rule{kind: different, switches: 0b00101},
			state:       0b10100,
			condition:   "S1 и S3 должны различаться.",
			observation: "S1 = OFF, S3 = ON.",
		},
		{
			name:        "count lower boundary",
			rule:        rule{kind: exactCount, switches: 0b01111, target: 1},
			state:       0b10010,
			condition:   "Число включённых в группе S1, S2, S3, S4 должно быть равно 1.",
			observation: "S1 = OFF, S2 = ON, S3 = OFF, S4 = OFF. Включено: 1 из 4.",
		},
		{
			name:        "count upper boundary",
			rule:        rule{kind: exactCount, switches: 0b01111, target: 3},
			state:       0b11101,
			condition:   "Число включённых в группе S1, S2, S3, S4 должно быть равно 3.",
			observation: "S1 = ON, S2 = OFF, S3 = ON, S4 = ON. Включено: 3 из 4.",
		},
		{
			name:        "even zero",
			rule:        rule{kind: parity, switches: 0b10101, target: 0},
			state:       0b01010,
			condition:   "В группе S1, S3, S5 число включённых должно быть чётным.",
			observation: "S1 = OFF, S3 = OFF, S5 = OFF. Включено: 0 из 3.",
		},
		{
			name:        "even four",
			rule:        rule{kind: parity, switches: 0b01111, target: 0},
			state:       0b11111,
			condition:   "В группе S1, S2, S3, S4 число включённых должно быть чётным.",
			observation: "S1 = ON, S2 = ON, S3 = ON, S4 = ON. Включено: 4 из 4.",
		},
		{
			name:        "odd three",
			rule:        rule{kind: parity, switches: 0b10101, target: 1},
			state:       0b10101,
			condition:   "В группе S1, S3, S5 число включённых должно быть нечётным.",
			observation: "S1 = ON, S3 = ON, S5 = ON. Включено: 3 из 3.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.rule.matches(tt.state) {
				t.Fatal("explanation fixture does not satisfy its rule")
			}
			condition, observation := tt.rule.explain(tt.state)
			if condition != tt.condition || observation != tt.observation {
				t.Errorf("got (%q, %q), want (%q, %q)", condition, observation, tt.condition, tt.observation)
			}
		})
	}
}

func TestVaultExplain(t *testing.T) {
	v := exampleVault()
	before := v
	solution := Combination{true, false, false, false, true}
	want := [SystemCount]Explanation{
		{
			ID:          SystemPower,
			Condition:   "S1 и S3 должны различаться.",
			Observation: "S1 = ON, S3 = OFF.",
		},
		{
			ID:          SystemSync,
			Condition:   "S2 и S4 должны совпадать.",
			Observation: "S2 = OFF, S4 = OFF.",
		},
		{
			ID:          SystemAccess,
			Condition:   "Число включённых в группе S1, S4, S5 должно быть равно 2.",
			Observation: "S1 = ON, S4 = OFF, S5 = ON. Включено: 2 из 3.",
		},
	}
	for range 2 {
		got, err := v.Explain(solution)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("Explain() = %+v, want %+v", got, want)
		}
	}
	if v != before {
		t.Fatal("explaining a solution changed the vault")
	}
	assertLockedExplanation(t, v, Combination{})
	assertLockedExplanation(t, Vault{}, solution)
}

func TestGeneratedVaultExplanations(t *testing.T) {
	for seed := range int64(256) {
		v, err := Generate(seed)
		if err != nil {
			t.Fatal(err)
		}
		for state := range 1 << SwitchCount {
			combination := combinationFromState(state)
			result := v.Check(combination)
			if !result.Opened {
				assertLockedExplanation(t, v, combination)
				continue
			}
			explanations, err := v.Explain(combination)
			if err != nil {
				t.Fatalf("seed %d solution %05b: %v", seed, state, err)
			}
			for index, explanation := range explanations {
				if explanation.ID != result.Indicators[index].ID || explanation.Condition == "" || explanation.Observation == "" {
					t.Fatalf("seed %d: incomplete explanation for indicator %d: %+v", seed, index, explanation)
				}
			}
		}
	}
}

func assertLockedExplanation(t *testing.T, v Vault, combination Combination) {
	t.Helper()
	explanations, err := v.Explain(combination)
	if !errors.Is(err, ErrVaultLocked) {
		t.Fatalf("Explain(%v) error = %v, want ErrVaultLocked", combination, err)
	}
	if explanations != [SystemCount]Explanation{} {
		t.Fatalf("Explain(%v) leaked rules for a failing combination: %+v", combination, explanations)
	}
}
