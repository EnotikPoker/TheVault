package blackbox

import (
	"errors"
	"math/bits"
)

const (
	Size         = 6
	ElementCount = 3
	RulesVersion = 1
)

var (
	ErrInvalidBoard = errors.New("blackbox: invalid board")
	ErrInvalidPort  = errors.New("blackbox: invalid port")
	ErrRayLoop      = errors.New("blackbox: ray entered a cycle")
)

type Cell struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

func (c Cell) Valid() bool { return c.Row >= 0 && c.Row < Size && c.Col >= 0 && c.Col < Size }

type Port struct {
	Side  string `json:"side"`
	Index int    `json:"index"`
}

var sides = [4]string{"top", "right", "bottom", "left"}

func (p Port) Valid() bool {
	if p.Index < 0 || p.Index >= Size {
		return false
	}
	for _, side := range sides {
		if p.Side == side {
			return true
		}
	}
	return false
}

func Ports() [4 * Size]Port {
	var ports [4 * Size]Port
	for side, name := range sides {
		for index := range Size {
			ports[side*Size+index] = Port{Side: name, Index: index}
		}
	}
	return ports
}

// Board stores an immutable layout. Persistence uses Mask; public responses use separate DTOs.
type Board struct{ mask uint64 }

func FromMask(mask uint64) (Board, error) {
	if mask>>(Size*Size) != 0 || bits.OnesCount64(mask) != ElementCount {
		return Board{}, ErrInvalidBoard
	}
	return Board{mask: mask}, nil
}

func NewBoard(cells []Cell) (Board, error) {
	if len(cells) != ElementCount {
		return Board{}, ErrInvalidBoard
	}
	var mask uint64
	for _, cell := range cells {
		if !cell.Valid() {
			return Board{}, ErrInvalidBoard
		}
		mask |= 1 << (cell.Row*Size + cell.Col)
	}
	return FromMask(mask)
}

func (b Board) Mask() uint64 { return b.mask }

func (b Board) Cells() []Cell {
	cells := make([]Cell, 0, ElementCount)
	for index := range Size * Size {
		if b.mask&(1<<index) != 0 {
			cells = append(cells, Cell{Row: index / Size, Col: index % Size})
		}
	}
	return cells
}

func (b Board) Matches(cells []Cell) bool {
	other, err := NewBoard(cells)
	return err == nil && other == b
}

func (b Board) occupied(row, col int) bool {
	return row >= 0 && row < Size && col >= 0 && col < Size && b.mask&(1<<(row*Size+col)) != 0
}
