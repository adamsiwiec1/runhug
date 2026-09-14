package packs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Installed tracks which category packs the user selected and local watermarks.
type Installed struct {
	UpdatedAt  string                    `json:"updated_at"`
	Categories map[string]InstalledPack  `json:"categories"`
}

// InstalledPack is local provenance for one category.
type InstalledPack struct {
	ID            string `json:"id"`
	Title         string `json:"title,omitempty"`
	Watermark     string `json:"watermark,omitempty"` // RFC3339 max lastModified
	InstalledAt   string `json:"installed_at,omitempty"`
	SourceRelease string `json:"source_release,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	Rows          int    `json:"rows,omitempty"`
}

// LoadInstalled reads packs/installed.json (empty struct if missing).
func LoadInstalled(path string) (*Installed, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Installed{Categories: map[string]InstalledPack{}}, nil
		}
		return nil, err
	}
	var inst Installed
	if err := json.Unmarshal(data, &inst); err != nil {
		return nil, err
	}
	if inst.Categories == nil {
		inst.Categories = map[string]InstalledPack{}
	}
	return &inst, nil
}

// SaveInstalled writes packs/installed.json.
func SaveInstalled(path string, inst *Installed) error {
	if inst.Categories == nil {
		inst.Categories = map[string]InstalledPack{}
	}
	inst.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(inst, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}


// SelectedIDs returns installed category ids in stable DefaultCategories order,
// then any extras alphabetically omitted (only defaults for now).
func (inst *Installed) SelectedIDs() []string {
	var out []string
	for _, c := range DefaultCategories() {
		if _, ok := inst.Categories[c.ID]; ok {
			out = append(out, c.ID)
		}
	}
	return out
}
