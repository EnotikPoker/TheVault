package vault

import (
	"errors"
	"fmt"
	"math/bits"
	"strings"
)

var ErrVaultLocked = errors.New("vault: combination does not open the vault")

type Explanation struct {
	ID          SystemID
	Condition   string
	Observation string
}

// Explain reveals the rules only for a combination that opens the vault.
func (v Vault) Explain(combination Combination) ([SystemCount]Explanation, error) {
	result := v.Check(combination)
	if !result.Opened {
		return [SystemCount]Explanation{}, ErrVaultLocked
	}
	var explanations [SystemCount]Explanation
	state := combination.mask()
	for index, r := range v.rules {
		condition, observation := r.explain(state)
		explanations[index] = Explanation{
			ID:          result.Indicators[index].ID,
			Condition:   condition,
			Observation: observation,
		}
	}
	return explanations, nil
}

func (r rule) explain(state uint8) (condition, observation string) {
	var names, positions []string
	for index := range SwitchCount {
		if r.switches&(1<<index) == 0 {
			continue
		}
		name := fmt.Sprintf("S%d", index+1)
		position := "OFF"
		if state&(1<<index) != 0 {
			position = "ON"
		}
		names = append(names, name)
		positions = append(positions, name+" = "+position)
	}
	group := strings.Join(names, ", ")
	observation = strings.Join(positions, ", ") + "."
	switch r.kind {
	case equal:
		condition = fmt.Sprintf("%s и %s должны совпадать.", names[0], names[1])
	case different:
		condition = fmt.Sprintf("%s и %s должны различаться.", names[0], names[1])
	case exactCount:
		condition = fmt.Sprintf("Число включённых в группе %s должно быть равно %d.", group, r.target)
	case parity:
		parityName := "чётным"
		if r.target == 1 {
			parityName = "нечётным"
		}
		condition = fmt.Sprintf("В группе %s число включённых должно быть %s.", group, parityName)
	}
	if r.kind == exactCount || r.kind == parity {
		observation += fmt.Sprintf(" Включено: %d из %d.", bits.OnesCount8(state&r.switches), len(names))
	}
	return condition, observation
}
