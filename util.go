package main

func visiblePanes(panes []pane, width int) []pane {
	if len(panes) == 0 {
		return panes
	}

	used := 0
	start := len(panes) - 1
	for ; start >= 0; start-- {
		paneWidth := paneDisplayWidth(panes[start], width)
		if used > 0 && used+paneWidth > width-2 {
			break
		}
		used += paneWidth
	}

	return panes[start+1:]
}

func truncate(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

func textWidth(s string) int {
	return len([]rune(s))
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
