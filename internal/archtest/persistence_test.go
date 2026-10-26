package archtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"
)

type persistenceInventory struct {
	Stores     []persistenceStore              `json:"stores"`
	Exceptions map[string]string               `json:"exceptions"`
	References map[string]persistenceReference `json:"references"`
}
type persistenceStore struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Lock    string `json:"lock"`
	Owner   string `json:"owner"`
	Failure string `json:"failure"`
	Web     string `json:"web"`
	Source  string `json:"source"`
}
type persistenceReference struct {
	Count int    `json:"count"`
	Owner string `json:"owner"`
}

func TestPersistenceInventory(t *testing.T) {
	tree := os.DirFS("../..")
	data, err := fs.ReadFile(tree, "internal/archtest/persistence_inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog persistenceInventory
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&catalog); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatalf("inventory has trailing data: %v", err)
	}
	actual, err := persistenceScan(tree)
	if err != nil {
		t.Fatal(err)
	}
	if errs := checkPersistence(tree, actual, catalog); len(errs) > 0 {
		t.Fatalf("persistence inventory drift:\n%s\nUpdate the reviewed inventory and docs/persistence.md; do not add broad exemptions. See #146.", strings.Join(errs, "\n"))
	}
}

func checkPersistence(tree fs.FS, actual map[string]int, catalog persistenceInventory) []string {
	var errs []string
	owners := map[string]bool{}
	for _, store := range catalog.Stores {
		if store.ID == "" || owners[store.ID] {
			errs = append(errs, "empty or duplicate store ID: "+store.ID)
		}
		owners[store.ID] = true
		if store.Path == "" || store.Lock == "" || store.Owner == "" || store.Failure == "" || store.Web == "" || store.Source == "" {
			errs = append(errs, "incomplete store: "+store.ID)
		}
		if _, err := fs.Stat(tree, store.Source); err != nil {
			errs = append(errs, "missing store source: "+store.Source)
		}
		// Both data and lock names must be observed in that store's reviewed
		// references, not merely asserted in the human-readable catalog.
		for _, name := range []string{store.Path, store.Lock} {
			if name == "none" || name == "config.toml" {
				continue
			}
			base := name[strings.LastIndex(name, "/")+1:]
			found := false
			for key, ref := range catalog.References {
				if ref.Owner == store.ID && strings.HasSuffix(key, "|path|"+base) && actual[key] > 0 {
					found = true
				}
			}
			if !found {
				errs = append(errs, "unobserved store path: "+store.ID+" "+name)
			}
		}
	}
	for id, reason := range catalog.Exceptions {
		if id == "" || owners[id] || strings.TrimSpace(reason) == "" {
			errs = append(errs, "invalid exception: "+id)
		}
		owners[id] = true
	}
	used := map[string]bool{}
	for key, count := range actual {
		ref, ok := catalog.References[key]
		if !ok || ref.Count != count {
			errs = append(errs, fmt.Sprintf("%s: observed %d, registered %d", key, count, ref.Count))
		}
	}
	for key, ref := range catalog.References {
		if ref.Count <= 0 || actual[key] != ref.Count {
			errs = append(errs, "stale reference: "+key)
		}
		if !owners[ref.Owner] {
			errs = append(errs, "unregistered owner for "+key+": "+ref.Owner)
		}
		used[ref.Owner] = true
	}
	for owner := range owners {
		if !used[owner] {
			errs = append(errs, "unused owner: "+owner)
		}
	}
	doc, err := fs.ReadFile(tree, "docs/persistence.md")
	if err != nil {
		errs = append(errs, err.Error())
	} else if err := checkPersistenceDoc(string(doc), catalog.Stores); err != nil {
		errs = append(errs, err.Error())
	}
	sort.Strings(errs)
	return errs
}

const inventoryStart = "<!-- persistence-inventory:start -->"
const inventoryEnd = "<!-- persistence-inventory:end -->"

func persistenceRows(stores []persistenceStore) []string {
	rows := []string{"ID|路径|锁|所有者|损坏或冲突|网页可改"}
	for _, s := range stores {
		rows = append(rows, strings.Join([]string{s.ID, s.Path, s.Lock, s.Owner, s.Failure, s.Web}, "|"))
	}
	return rows
}

func checkPersistenceDoc(doc string, stores []persistenceStore) error {
	if strings.Count(doc, inventoryStart) != 1 || strings.Count(doc, inventoryEnd) != 1 {
		return fmt.Errorf("docs/persistence.md: inventory markers must occur exactly once")
	}
	_, tail, _ := strings.Cut(doc, inventoryStart)
	block, _, ok := strings.Cut(tail, inventoryEnd)
	if !ok {
		return fmt.Errorf("docs/persistence.md: reversed inventory markers")
	}
	var rows []string
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			return fmt.Errorf("docs/persistence.md: unexpected content in inventory")
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if len(cells) > 0 && strings.Trim(cells[0], "-: ") == "" {
			continue
		}
		rows = append(rows, strings.Join(cells, "|"))
	}
	if strings.Join(rows, "\n") != strings.Join(persistenceRows(stores), "\n") {
		return fmt.Errorf("docs/persistence.md: inventory table differs from persistence_inventory.json (including missing, extra or stale rows)")
	}
	return nil
}
