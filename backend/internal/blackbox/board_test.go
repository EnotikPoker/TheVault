package blackbox

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestRayRules(t *testing.T) {
	tests := []struct {
		name  string
		cells []Cell
		entry Port
		kind  string
		exit  *Port
	}{
		{"straight", nil, Port{"top", 2}, "exit", &Port{"bottom", 2}},
		{"head on", []Cell{{1, 2}}, Port{"top", 2}, "hit", nil},
		{"head on from side", []Cell{{1, 2}}, Port{"left", 1}, "hit", nil},
		{"diagonal deflection", []Cell{{1, 2}}, Port{"top", 1}, "exit", &Port{"left", 0}},
		{"opposite diagonal", []Cell{{1, 2}}, Port{"top", 3}, "exit", &Port{"right", 0}},
		{"two diagonals", []Cell{{2, 1}, {2, 3}}, Port{"top", 2}, "reflection", nil},
		{"entrance reflection", []Cell{{0, 2}}, Port{"top", 1}, "reflection", nil},
		{"absorption priority at entrance", []Cell{{0, 1}, {0, 2}}, Port{"top", 1}, "hit", nil},
		{"absorption priority inside", []Cell{{2, 1}, {2, 2}, {2, 3}}, Port{"top", 2}, "hit", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var board Board
			for _, cell := range tt.cells {
				board.mask |= 1 << (cell.Row*Size + cell.Col)
			}
			trace, err := board.Trace(tt.entry)
			if err != nil || trace.Result.Kind != tt.kind || !reflect.DeepEqual(trace.Result.Exit, tt.exit) {
				t.Fatalf("Trace(%v) = %+v, %v", tt.entry, trace, err)
			}
			for index := 1; index < len(trace.Path); index++ {
				a, b := trace.Path[index-1], trace.Path[index]
				if abs(a.Row-b.Row)+abs(a.Col-b.Col) != 1 {
					t.Fatalf("non-adjacent path points: %v, %v", a, b)
				}
			}
		})
	}
}

func TestBoardValidation(t *testing.T) {
	cells := []Cell{{0, 0}, {2, 3}, {5, 5}}
	board, err := NewBoard(cells)
	if err != nil || !board.Matches([]Cell{cells[2], cells[0], cells[1]}) {
		t.Fatalf("valid board: %v", err)
	}
	for _, invalid := range [][]Cell{nil, {{0, 0}, {0, 0}, {1, 1}}, {{-1, 0}, {2, 3}, {5, 5}}, {{0, 6}, {2, 3}, {5, 5}}} {
		if _, err := NewBoard(invalid); !errors.Is(err, ErrInvalidBoard) {
			t.Fatalf("accepted invalid board: %v", invalid)
		}
	}
	if _, err := FromMask(1 << 40); !errors.Is(err, ErrInvalidBoard) {
		t.Fatal("accepted an out-of-bounds mask")
	}
	for _, invalid := range []Port{{"wrong", 1}, {"top", -1}, {"left", Size}} {
		if _, err := board.Trace(invalid); !errors.Is(err, ErrInvalidPort) {
			t.Fatalf("accepted invalid port: %v", invalid)
		}
	}
}

func TestCatalogUniquenessAndReciprocity(t *testing.T) {
	counts := make(map[[4 * Size]byte]int)
	for a := 0; a < Size*Size; a++ {
		for b := a + 1; b < Size*Size; b++ {
			for c := b + 1; c < Size*Size; c++ {
				board := Board{mask: 1<<a | 1<<b | 1<<c}
				key, err := signature(board)
				if err != nil {
					t.Fatal(err)
				}
				counts[key]++
				for _, entry := range Ports() {
					trace, _ := board.Trace(entry)
					if trace.Result.Exit != nil {
						reverse, err := board.Trace(*trace.Result.Exit)
						if err != nil || reverse.Result.Exit == nil || *reverse.Result.Exit != entry {
							t.Fatalf("asymmetric ray: mask=%d entry=%v result=%+v", board.mask, entry, trace.Result)
						}
					}
				}
			}
		}
	}
	if len(catalog()) == 0 {
		t.Fatal("empty catalog")
	}
	for _, board := range catalog() {
		key, err := signature(board)
		if err != nil || counts[key] != 1 {
			t.Fatal("catalog contains an ambiguous layout")
		}
	}
	t.Logf("verified %d distinguishable layouts", len(catalog()))
}

func TestSeededGeneration(t *testing.T) {
	for _, seed := range []int64{0, 1, -1, 42, math.MinInt64, math.MaxInt64} {
		t.Run("seed", func(t *testing.T) {
			t.Parallel()
			first, err := Generate(seed)
			if err != nil {
				t.Fatal(err)
			}
			for range 10 {
				again, err := Generate(seed)
				if err != nil || again != first {
					t.Fatal("generation is not reproducible")
				}
			}
			before := first.Cells()
			cells := first.Cells()
			cells[0] = Cell{Row: -1, Col: -1}
			if !reflect.DeepEqual(first.Cells(), before) {
				t.Fatal("mutating a returned slice changed the board")
			}
		})
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
