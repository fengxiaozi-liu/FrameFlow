package story

import (
	"errors"
	"strings"
)

type EntityBinding struct {
	ID                    string    `json:"id"`
	EntityID              string    `json:"entity_id"`
	EntityRevision        int64     `json:"entity_revision"`
	DescriptionOverride   *string   `json:"description_override,omitempty"`
	DescriptionSupplement string    `json:"description_supplement,omitempty"`
	SelectedMaterialIDs   *[]string `json:"selected_material_ids,omitempty"`
	Role                  string    `json:"role"`
}
type PromptNode struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	BindingID string `json:"binding_id,omitempty"`
}
type PromptDocument struct {
	SchemaVersion int          `json:"schema_version"`
	Nodes         []PromptNode `json:"nodes"`
}

func (d PromptDocument) Render(names map[string]string) (string, error) {
	if d.SchemaVersion != 1 {
		return "", errors.New("unsupported prompt schema")
	}
	var b strings.Builder
	for _, n := range d.Nodes {
		switch n.Type {
		case "text":
			if n.BindingID != "" {
				return "", errors.New("text cannot reference binding")
			}
			b.WriteString(n.Text)
		case "entity_ref":
			name, ok := names[n.BindingID]
			if !ok || n.Text != "" {
				return "", errors.New("reference_missing")
			}
			b.WriteString(name)
		default:
			return "", errors.New("unknown prompt node")
		}
	}
	return b.String(), nil
}
func ValidateBindings(bindings []EntityBinding) error {
	seen := map[string]bool{}
	for _, b := range bindings {
		if b.ID == "" || b.EntityID == "" || b.EntityRevision < 1 || seen[b.ID] {
			return errors.New("invalid or duplicate entity binding")
		}
		seen[b.ID] = true
		if b.Role != "primary" && b.Role != "supplement" && b.Role != "cast" {
			return errors.New("invalid binding role")
		}
		if b.SelectedMaterialIDs != nil {
			m := map[string]bool{}
			for _, id := range *b.SelectedMaterialIDs {
				if id == "" || m[id] {
					return errors.New("duplicate or empty selected reference")
				}
				m[id] = true
			}
		}
	}
	return nil
}
