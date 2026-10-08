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
	ReferenceMaterialIDs  *[]string `json:"reference_material_ids,omitempty"`
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

// MergeDescriptionSupplements keeps legacy shot-specific notes editable in the
// main prompt. Clear each source after appending so saves/reloads are idempotent.
func (s *Scene) MergeDescriptionSupplements() bool {
	changed := false
	for i := range s.EntityBindings {
		b := &s.EntityBindings[i]
		if b.DescriptionSupplement == "" {
			continue
		}
		if s.PromptDocument == nil {
			s.PromptDocument = &PromptDocument{SchemaVersion: 1, Nodes: []PromptNode{{Type: "text", Text: s.VisualPrompt}}}
		}
		nodes := s.PromptDocument.Nodes
		if len(nodes) > 0 && nodes[len(nodes)-1].Type == "text" {
			nodes[len(nodes)-1].Text += "\n"
		} else if len(nodes) > 0 {
			nodes = append(nodes, PromptNode{Type: "text", Text: "\n"})
		}
		s.PromptDocument.Nodes = append(nodes,
			PromptNode{Type: "entity_ref", BindingID: b.ID},
			PromptNode{Type: "text", Text: "：" + b.DescriptionSupplement})
		b.DescriptionSupplement = ""
		changed = true
	}
	return changed
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
		for _, refs := range []*[]string{b.SelectedMaterialIDs, b.ReferenceMaterialIDs} {
			if refs != nil {
				m := map[string]bool{}
				for _, id := range *refs {
					if id == "" || m[id] {
						return errors.New("duplicate or empty selected reference")
					}
					m[id] = true
				}
			}
		}
	}
	return nil
}
