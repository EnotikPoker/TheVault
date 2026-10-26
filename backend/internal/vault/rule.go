package vault

import "math/bits"

type ruleKind uint8

const (
	invalidRule ruleKind = iota
	equal
	different
	exactCount
	parity
)

type rule struct {
	kind     ruleKind
	switches uint8
	target   uint8
}

func (r rule) matches(state uint8) bool {
	enabled := bits.OnesCount8(state & r.switches)
	switch r.kind {
	case equal:
		return enabled == 0 || enabled == 2
	case different:
		return enabled == 1
	case exactCount:
		return enabled == int(r.target)
	case parity:
		return enabled%2 == int(r.target)
	default:
		return false
	}
}
