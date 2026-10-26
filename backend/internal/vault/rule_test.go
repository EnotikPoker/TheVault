package vault

import "testing"

func TestRuleMatches(t *testing.T) {
	tests := []struct {
		name string
		rule rule
		pass []uint8
		fail []uint8
	}{
		{
			name: "equal S1 S3",
			rule: rule{kind: equal, switches: 0b00101},
			pass: []uint8{0b00000, 0b00101, 0b01010, 0b11111},
			fail: []uint8{0b00001, 0b00100, 0b11011},
		},
		{
			name: "different S1 S3",
			rule: rule{kind: different, switches: 0b00101},
			pass: []uint8{0b00001, 0b00100, 0b11011},
			fail: []uint8{0b00000, 0b00101, 0b01010, 0b11111},
		},
		{
			name: "two of S1 S3 S5",
			rule: rule{kind: exactCount, switches: 0b10101, target: 2},
			pass: []uint8{0b00101, 0b10001, 0b10100, 0b01111},
			fail: []uint8{0b00000, 0b00001, 0b10101},
		},
		{
			name: "one of four",
			rule: rule{kind: exactCount, switches: 0b01111, target: 1},
			pass: []uint8{0b00001, 0b00010, 0b00100, 0b01000, 0b10001},
			fail: []uint8{0b00000, 0b00011, 0b01111},
		},
		{
			name: "three of four",
			rule: rule{kind: exactCount, switches: 0b01111, target: 3},
			pass: []uint8{0b00111, 0b01011, 0b01101, 0b01110, 0b11110},
			fail: []uint8{0b00000, 0b00011, 0b01111},
		},
		{
			name: "even S1 S3 S5",
			rule: rule{kind: parity, switches: 0b10101, target: 0},
			pass: []uint8{0b00000, 0b00101, 0b10001, 0b10100, 0b01010},
			fail: []uint8{0b00001, 0b00100, 0b10000, 0b10101},
		},
		{
			name: "odd S1 S3 S5",
			rule: rule{kind: parity, switches: 0b10101, target: 1},
			pass: []uint8{0b00001, 0b00100, 0b10000, 0b10101},
			fail: []uint8{0b00000, 0b00101, 0b10001, 0b10100, 0b01010},
		},
		{
			name: "even four",
			rule: rule{kind: parity, switches: 0b01111, target: 0},
			pass: []uint8{0b00000, 0b00011, 0b01111, 0b10000},
			fail: []uint8{0b00001, 0b00111},
		},
		{
			name: "odd four",
			rule: rule{kind: parity, switches: 0b01111, target: 1},
			pass: []uint8{0b00001, 0b00111, 0b10001},
			fail: []uint8{0b00000, 0b00011, 0b01111},
		},
		{
			name: "uninitialized",
			fail: []uint8{0b00000, 0b00001, 0b11111},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, state := range tt.pass {
				if !tt.rule.matches(state) {
					t.Errorf("state %05b should pass", state)
				}
			}
			for _, state := range tt.fail {
				if tt.rule.matches(state) {
					t.Errorf("state %05b should fail", state)
				}
			}
		})
	}
}
