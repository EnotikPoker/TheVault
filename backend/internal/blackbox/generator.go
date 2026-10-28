package blackbox

import (
	"errors"
	"math/rand/v2"
	"sync"
)

var catalog = sync.OnceValue(buildCatalog)

func Generate(seed int64) (Board, error) {
	boards := catalog()
	if len(boards) == 0 {
		return Board{}, errors.New("blackbox: no distinguishable layouts")
	}
	rng := rand.New(rand.NewPCG(uint64(seed), 0))
	return boards[rng.IntN(len(boards))], nil
}

func signature(board Board) ([4 * Size]byte, error) {
	var result [4 * Size]byte
	for index, port := range Ports() {
		trace, err := board.Trace(port)
		if err != nil {
			return result, err
		}
		switch trace.Result.Kind {
		case "hit":
			result[index] = 0
		case "reflection":
			result[index] = 1
		case "exit":
			for exitIndex, exit := range Ports() {
				if exit == *trace.Result.Exit {
					result[index] = byte(exitIndex + 2)
					break
				}
			}
		}
	}
	return result, nil
}

func buildCatalog() []Board {
	type candidate struct {
		board Board
		key   [4 * Size]byte
	}
	var candidates []candidate
	counts := make(map[[4 * Size]byte]int)
	for a := 0; a < Size*Size; a++ {
		for b := a + 1; b < Size*Size; b++ {
			for c := b + 1; c < Size*Size; c++ {
				board := Board{mask: 1<<a | 1<<b | 1<<c}
				key, err := signature(board)
				if err == nil {
					candidates = append(candidates, candidate{board, key})
					counts[key]++
				}
			}
		}
	}
	var boards []Board
	for _, candidate := range candidates {
		// Exclude every member of an ambiguous group, not just its duplicates.
		if counts[candidate.key] == 1 {
			boards = append(boards, candidate.board)
		}
	}
	return boards
}
