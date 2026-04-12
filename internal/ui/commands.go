package ui

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/atotto/clipboard"
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

// loadTreeGroupCountsCmd builds a tree-ordered flat list from the store's prefix→count map.
// The result is a pre-order traversal: each level sorted by count desc.
func loadTreeGroupCountsCmd(store Store, maxDepth int) tea.Cmd {
	return func() tea.Msg {
		flat, err := store.TreeGroupCounts(maxDepth)
		if err != nil {
			return groupCountsMsg{err: err}
		}

		// Build a tree of nodes, then flatten into pre-order.
		type node struct {
			name     string
			prefix   string
			count    int
			depth    int
			children []*node
		}
		root := &node{children: make([]*node, 0)}
		lookup := map[string]*node{"": root}

		// Collect all prefixes and sort by depth then name for deterministic insertion.
		type prefixEntry struct {
			prefix string
			count  int
			depth  int
			parent string
			name   string
		}
		entries := make([]prefixEntry, 0, len(flat))
		for prefix, count := range flat {
			depth := strings.Count(prefix, ":") + 1
			parent := ""
			name := prefix
			if idx := strings.LastIndex(prefix, ":"); idx >= 0 {
				parent = prefix[:idx]
				name = prefix[idx+1:]
			}
			entries = append(entries, prefixEntry{prefix, count, depth, parent, name})
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].depth != entries[j].depth {
				return entries[i].depth < entries[j].depth
			}
			return entries[i].prefix < entries[j].prefix
		})

		for _, e := range entries {
			n := &node{
				name:     e.name,
				prefix:   e.prefix,
				count:    e.count,
				depth:    e.depth - 1,
				children: make([]*node, 0),
			}
			lookup[e.prefix] = n
			if p, ok := lookup[e.parent]; ok {
				p.children = append(p.children, n)
			}
		}

		// Sort children at each level by count desc, then name asc.
		var sortChildren func(n *node)
		sortChildren = func(n *node) {
			sort.Slice(n.children, func(i, j int) bool {
				if n.children[i].count != n.children[j].count {
					return n.children[i].count > n.children[j].count
				}
				return n.children[i].prefix < n.children[j].prefix
			})
			for _, c := range n.children {
				sortChildren(c)
			}
		}
		sortChildren(root)

		// Prune nodes that are not real groups:
		// - A child with count == 1 is a single record, not a group.
		// - A node whose only child has the same count adds no information.
		var prune func(n *node)
		prune = func(n *node) {
			for _, c := range n.children {
				prune(c)
			}
			filtered := make([]*node, 0, len(n.children))
			for _, c := range n.children {
				if c.count <= 1 {
					continue
				}
				// Skip a child that is the sole child and has the same count
				// as its parent — it adds no useful breakdown.
				if len(n.children) == 1 && c.count == n.count && n != root {
					continue
				}
				filtered = append(filtered, c)
			}
			n.children = filtered
		}
		prune(root)

		// Flatten into pre-order traversal.
		var out []groupCount
		var walk func(n *node)
		walk = func(n *node) {
			for _, c := range n.children {
				out = append(out, groupCount{
					group: c.prefix,
					count: c.count,
					depth: c.depth,
				})
				walk(c)
			}
		}
		walk(root)

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
		oldVal, _ := store.Get(key)
		err := store.Delete(key)
		return deleteResultMsg{key: key, oldValue: oldVal, err: err}
	}
}

func deletePatternCmd(store Store, pattern string) tea.Cmd {
	return func() tea.Msg {
		var deleted []string
		oldValues := make(map[string][]byte)
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
					if oldVal, getErr := store.Get(k); getErr == nil {
						oldValues[k] = oldVal
					}
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
		return deletePatternResultMsg{pattern: pattern, keys: deleted, oldValues: oldValues}
	}
}

func matchPattern(pattern, key string) (bool, error) {
	return path.Match(pattern, key)
}

func exportSingleCmd(store Store, key string) tea.Cmd {
	return func() tea.Msg {
		value, err := store.Get(key)
		if err != nil {
			return exportResultMsg{err: fmt.Errorf("key %q: %w", key, err)}
		}
		data := map[string]interface{}{
			key: smartJSONValue(value),
		}
		return writeExportFile(data, 1)
	}
}

func exportVisibleCmd(store Store, keys []string) tea.Cmd {
	return func() tea.Msg {
		data := make(map[string]interface{}, len(keys))
		for _, k := range keys {
			v, err := store.Get(k)
			if err != nil {
				return exportResultMsg{err: fmt.Errorf("key %q: %w", k, err)}
			}
			data[k] = smartJSONValue(v)
		}
		return writeExportFile(data, len(keys))
	}
}

func exportAllCmd(store Store) tea.Cmd {
	return func() tea.Msg {
		data := make(map[string]interface{})
		startAfter := ""
		for {
			keys, lastKey, hasMore, err := store.ListKeysPage(startAfter, 1000)
			if err != nil {
				return exportResultMsg{err: fmt.Errorf("list keys: %w", err)}
			}
			for _, k := range keys {
				v, err := store.Get(k)
				if err != nil {
					return exportResultMsg{err: fmt.Errorf("key %q: %w", k, err)}
				}
				data[k] = smartJSONValue(v)
			}
			if !hasMore || len(keys) == 0 {
				break
			}
			startAfter = lastKey
		}
		return writeExportFile(data, len(data))
	}
}

func smartJSONValue(v []byte) interface{} {
	var parsed interface{}
	if json.Unmarshal(v, &parsed) == nil {
		return parsed
	}
	if utf8.Valid(v) {
		return string(v)
	}
	return base64.StdEncoding.EncodeToString(v)
}

func writeExportFile(data map[string]interface{}, count int) exportResultMsg {
	ts := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("badger_export_%s.json", ts)
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return exportResultMsg{err: err}
	}
	if err := os.WriteFile(filename, b, 0644); err != nil {
		return exportResultMsg{err: err}
	}
	return exportResultMsg{filePath: filename, count: count}
}

func undoCmd(store Store, entry undoEntry) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch entry.op {
		case undoEdit, undoDelete:
			err = store.Set(entry.key, entry.oldValue)
		case undoCreate:
			err = store.Delete(entry.key)
		}
		return undoResultMsg{op: entry.op, key: entry.key, err: err}
	}
}

func createKeyCmd(store Store, key string, value []byte) tea.Cmd {
	return func() tea.Msg {
		err := store.Set(key, value)
		return saveResultMsg{key: key, err: err, isNew: true}
	}
}

func copyToClipboardCmd(content string, what string) tea.Cmd {
	return func() tea.Msg {
		err := clipboard.WriteAll(content)
		return clipboardResultMsg{what: what, err: err}
	}
}
