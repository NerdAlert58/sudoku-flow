package solver

// Simple colouring (ADR-0002 ladder index 12). For one digit, the conjugate pairs (units where
// the digit sits in exactly two cells) form strong links. Two-colouring a connected component of
// these links yields two classes, exactly one of which is the true set of that digit in any
// solution. Two SOUND eliminations follow — no uniqueness assumed:
//
//	Rule 2 (colour trap):  two same-colour cells that see each other cannot both be the digit,
//	                       so that whole colour is false — remove the digit from every cell of it.
//	Rule 4 (colour wrap):  a cell outside the chain that sees both colours must see the digit in
//	                       one of them, so the digit is removed from that cell.
//
// Digits are scanned ascending, components in cell order, so the first productive pattern fires.
func simpleColouring(e *engine) (Event, bool) {
	for d := 1; d <= 9; d++ {
		bit := uint16(1) << d

		var adj [81][]int
		for u := 0; u < 27; u++ {
			var pos []int
			for _, idx := range units[u] {
				if e.board[idx] == 0 && e.cand[idx]&bit != 0 {
					pos = append(pos, idx)
				}
			}
			if len(pos) == 2 {
				adj[pos[0]] = append(adj[pos[0]], pos[1])
				adj[pos[1]] = append(adj[pos[1]], pos[0])
			}
		}

		var color [81]int8
		for i := range color {
			color[i] = -1
		}
		for start := 0; start < 81; start++ {
			if color[start] != -1 || len(adj[start]) == 0 {
				continue
			}
			// BFS two-colour the component rooted at start.
			color[start] = 0
			queue := []int{start}
			var comp []int
			for len(queue) > 0 {
				cur := queue[0]
				queue = queue[1:]
				comp = append(comp, cur)
				for _, nb := range adj[cur] {
					if color[nb] == -1 {
						color[nb] = color[cur] ^ 1
						queue = append(queue, nb)
					}
				}
			}

			if ev, ok := colourRule2(e, d, comp, &color); ok {
				return ev, true
			}
			if ev, ok := colourRule4(e, d, comp, &color); ok {
				return ev, true
			}
		}
	}
	return Event{}, false
}

// colourRule2 removes digit d from an entire colour class when two of its cells see each other.
func colourRule2(e *engine, d int, comp []int, color *[81]int8) (Event, bool) {
	bit := uint16(1) << d
	for i := 0; i < len(comp); i++ {
		for j := i + 1; j < len(comp); j++ {
			a, b := comp[i], comp[j]
			if color[a] != color[b] || !sees(a, b) {
				continue
			}
			bad := color[a]
			var elims []Elimination
			for _, c := range comp {
				if color[c] == bad && e.board[c] == 0 && e.cand[c]&bit != 0 {
					elims = append(elims, Elimination{Cell: Cell{c / 9, c % 9}, Candidate: d})
				}
			}
			if len(elims) > 0 {
				return e.elimEvent("simple_colouring", cellsOf([]int{a, b}), elims), true
			}
		}
	}
	return Event{}, false
}

// colourRule4 removes digit d from any candidate cell outside the component that sees both
// colours of the component.
func colourRule4(e *engine, d int, comp []int, color *[81]int8) (Event, bool) {
	bit := uint16(1) << d
	inComp := [81]bool{}
	for _, c := range comp {
		inComp[c] = true
	}
	var elims []Elimination
	w0, w1 := -1, -1
	for i := 0; i < 81; i++ {
		if e.board[i] != 0 || inComp[i] || e.cand[i]&bit == 0 {
			continue
		}
		s0, s1 := -1, -1
		for _, c := range comp {
			if !sees(i, c) {
				continue
			}
			if color[c] == 0 && s0 < 0 {
				s0 = c
			}
			if color[c] == 1 && s1 < 0 {
				s1 = c
			}
		}
		if s0 >= 0 && s1 >= 0 {
			elims = append(elims, Elimination{Cell: Cell{i / 9, i % 9}, Candidate: d})
			if w0 < 0 {
				w0, w1 = s0, s1
			}
		}
	}
	if len(elims) > 0 {
		return e.elimEvent("simple_colouring", cellsOf([]int{w0, w1}), elims), true
	}
	return Event{}, false
}
