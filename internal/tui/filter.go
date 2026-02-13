package tui

import "strings"

func (m *Model) ensureFilterIdx() {
	if m.filteredIdx == nil && len(m.files) > 0 {
		m.filteredIdx = make([]int, len(m.files))
		for i := range m.files {
			m.filteredIdx[i] = i
		}
	}
}

func (m *Model) rebuildFilter() {
	query := strings.ToLower(m.filterInput.Value())
	m.filteredIdx = m.filteredIdx[:0]
	for i, f := range m.files {
		if query == "" || strings.Contains(strings.ToLower(f.path), query) ||
			strings.Contains(strings.ToLower(string(f.status)), query) {
			m.filteredIdx = append(m.filteredIdx, i)
		}
	}
	if m.cursor >= len(m.filteredIdx) {
		m.cursor = max(0, len(m.filteredIdx)-1)
	}
}

func (m *Model) clearFilter() {
	m.filterInput.SetValue("")
	m.rebuildFilter()
}
