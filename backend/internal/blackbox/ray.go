package blackbox

type Result struct {
	Kind string `json:"kind"`
	Exit *Port  `json:"exit,omitempty"`
}

type Trace struct {
	Result Result
	Path   []Cell
}

func (b Board) Trace(entry Port) (Trace, error) {
	if !entry.Valid() {
		return Trace{}, ErrInvalidPort
	}
	row, col, dr, dc := 0, 0, 0, 0
	switch entry.Side {
	case "top":
		row, col, dr = -1, entry.Index, 1
	case "right":
		row, col, dc = entry.Index, Size, -1
	case "bottom":
		row, col, dr = Size, entry.Index, -1
	case "left":
		row, col, dc = entry.Index, -1, 1
	}
	path := []Cell{{Row: row, Col: col}}
	frontRow, frontCol := row+dr, col+dc
	if b.occupied(frontRow, frontCol) {
		return Trace{Result: Result{Kind: "hit"}, Path: append(path, Cell{Row: frontRow, Col: frontCol})}, nil
	}
	// A diagonal element at the entrance reflects the ray before it enters the field.
	if b.occupied(frontRow-dc, frontCol+dr) || b.occupied(frontRow+dc, frontCol-dr) {
		return Trace{Result: Result{Kind: "reflection"}, Path: path}, nil
	}
	row, col = frontRow, frontCol
	path = append(path, Cell{Row: row, Col: col})
	seen := make(map[[4]int]bool)
	for {
		state := [4]int{row, col, dr, dc}
		if seen[state] {
			return Trace{}, ErrRayLoop
		}
		seen[state] = true
		frontRow, frontCol = row+dr, col+dc
		if b.occupied(frontRow, frontCol) {
			return Trace{Result: Result{Kind: "hit"}, Path: append(path, Cell{Row: frontRow, Col: frontCol})}, nil
		}
		left := b.occupied(frontRow-dc, frontCol+dr)
		right := b.occupied(frontRow+dc, frontCol-dr)
		switch {
		case left && right:
			dr, dc = -dr, -dc
		case left:
			dr, dc = dc, -dr
		case right:
			dr, dc = -dc, dr
		}
		row, col = row+dr, col+dc
		path = append(path, Cell{Row: row, Col: col})
		var exit Port
		switch {
		case row < 0:
			exit = Port{Side: "top", Index: col}
		case col >= Size:
			exit = Port{Side: "right", Index: row}
		case row >= Size:
			exit = Port{Side: "bottom", Index: col}
		case col < 0:
			exit = Port{Side: "left", Index: row}
		default:
			continue
		}
		if exit == entry {
			return Trace{Result: Result{Kind: "reflection"}, Path: path}, nil
		}
		return Trace{Result: Result{Kind: "exit", Exit: &exit}, Path: path}, nil
	}
}
