package ui

import (
	"path"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

func loadValueCmd(store Store, key string) tea.Cmd {
	return func() tea.Msg {
		out, err := store.Get(key)
		return loadValueMsg{key: key, value: out, err: err}
	}
}

func loadKeysCmd(store Store, startAfter string, limit int) tea.Cmd {
	return func() tea.Msg {
		keys, lastKey, hasMore, err := store.ListKeysPage(startAfter, limit)
		return loadKeysMsg{
			keys:       keys,
			lastKey:    lastKey,
			hasMore:    hasMore,
			startAfter: startAfter,
			err:        err,
		}
	}
}

func countFilterCmd(store Store, term string) tea.Cmd {
	return func() tea.Msg {
		count, err := store.CountKeysMatching(term)
		return filterCountMsg{term: term, count: count, err: err}
	}
}

func loadGroupCountsCmd(store Store) tea.Cmd {
	return func() tea.Msg {
		counts, err := store.GroupKeyCounts()
		if err != nil {
			return groupCountsMsg{err: err}
		}
		out := make([]groupCount, 0, len(counts))
		for k, v := range counts {
			out = append(out, groupCount{group: k, count: v})
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].count == out[j].count {
				return out[i].group < out[j].group
			}
			return out[i].count > out[j].count
		})
		return groupCountsMsg{counts: out}
	}
}

func saveValueCmd(store Store, key string, value []byte) tea.Cmd {
	return func() tea.Msg {
		err := store.Set(key, value)
		return saveResultMsg{key: key, err: err}
	}
}

func deleteKeyCmd(store Store, key string) tea.Cmd {
	return func() tea.Msg {
		err := store.Delete(key)
		return deleteResultMsg{key: key, err: err}
	}
}

func deletePatternCmd(store Store, pattern string) tea.Cmd {
	return func() tea.Msg {
		var deleted []string
		startAfter := ""
		for {
			keys, lastKey, hasMore, err := store.ListKeysPage(startAfter, 1000)
			if err != nil {
				return deletePatternResultMsg{pattern: pattern, err: err}
			}
			for _, k := range keys {
				ok, err := matchPattern(pattern, k)
				if err != nil {
					return deletePatternResultMsg{pattern: pattern, err: err}
				}
				if ok {
					if err := store.Delete(k); err != nil {
						return deletePatternResultMsg{pattern: pattern, err: err}
					}
					deleted = append(deleted, k)
				}
			}
			if !hasMore || len(keys) == 0 {
				break
			}
			startAfter = lastKey
		}
		return deletePatternResultMsg{pattern: pattern, keys: deleted}
	}
}

func matchPattern(pattern, key string) (bool, error) {
	return path.Match(pattern, key)
}
