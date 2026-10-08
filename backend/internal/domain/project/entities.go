package project

import (
	"encoding/json"
	"errors"
	"strings"
)

type ReferenceImage struct {
	MaterialID  string `json:"material_id"`
	ContentHash string `json:"content_hash,omitempty"`
	Label       string `json:"label,omitempty"`
	Order       int    `json:"order"`
}
type EntityVersion struct {
	ID                   string           `json:"id"`
	Kind                 string           `json:"kind"`
	Name                 string           `json:"name"`
	Description          string           `json:"description"`
	Revision             int64            `json:"revision"`
	References           []ReferenceImage `json:"references"`
	VoiceMaterialID      string           `json:"voice_material_id,omitempty"`
	VoiceProfileID       string           `json:"voice_profile_id,omitempty"`
	VoiceProfileRevision int64            `json:"voice_profile_revision,omitempty"`
	Archived             bool             `json:"archived,omitempty"`
}
type VoiceProfile struct {
	ID            string `json:"id"`
	Revision      int64  `json:"revision"`
	Name          string `json:"name"`
	ConnectionID  string `json:"connection_id"`
	ModelID       string `json:"model_id"`
	RemoteVoiceID string `json:"remote_voice_id"`
	Language      string `json:"language"`
	Instructions  string `json:"instructions,omitempty"`
	Archived      bool   `json:"archived,omitempty"`
}
type EntityCatalog struct {
	Version  int64           `json:"catalog_version"`
	Entities []EntityVersion `json:"entities"`
	Voices   []VoiceProfile  `json:"voice_profiles"`
}

var ErrCatalogConflict = errors.New("entity_revision_conflict")

func (e EntityVersion) Validate() error {
	if e.ID == "" || strings.TrimSpace(e.Name) == "" || e.Revision < 1 || (e.Kind != "location" && e.Kind != "character") {
		return errors.New("invalid entity identity, name, revision or kind")
	}
	seen := map[string]bool{}
	for i, r := range e.References {
		if r.MaterialID == "" || seen[r.MaterialID] || r.Order != i {
			return errors.New("reference images must be unique and in contiguous order")
		}
		seen[r.MaterialID] = true
	}
	if e.Kind == "location" && (e.VoiceProfileID != "" || e.VoiceMaterialID != "") {
		return errors.New("locations cannot have voices")
	}
	if (e.VoiceProfileID == "") != (e.VoiceProfileRevision == 0) {
		return errors.New("voice identity and revision must be specified together")
	}
	return nil
}
func (v VoiceProfile) Validate() error {
	if v.ID == "" || v.Revision < 1 || strings.TrimSpace(v.Name) == "" || v.ConnectionID == "" || v.ModelID == "" || v.RemoteVoiceID == "" || v.Language == "" {
		return errors.New("voice profile requires identity, model, voice and language")
	}
	return nil
}
func (c EntityCatalog) Entity(id string, revision int64) (EntityVersion, error) {
	for _, e := range c.Entities {
		if e.ID == id && e.Revision == revision {
			b, _ := json.Marshal(e)
			var copy EntityVersion
			_ = json.Unmarshal(b, &copy)
			return copy, nil
		}
	}
	return EntityVersion{}, errors.New("entity version not found")
}
func (c EntityCatalog) Voice(id string, revision int64) (VoiceProfile, error) {
	for _, v := range c.Voices {
		if v.ID == id && v.Revision == revision {
			return v, nil
		}
	}
	return VoiceProfile{}, errors.New("voice profile version not found")
}
func (c *EntityCatalog) AddEntity(e EntityVersion, expected int64) error {
	if c.Version != expected {
		return ErrCatalogConflict
	}
	if err := e.Validate(); err != nil {
		return err
	}
	var latest int64
	for _, old := range c.Entities {
		if old.ID == e.ID {
			if old.Kind != e.Kind {
				return errors.New("entity type cannot change")
			}
			if old.Revision > latest {
				latest = old.Revision
			}
		}
	}
	if e.Revision != latest+1 {
		return ErrCatalogConflict
	}
	if e.VoiceProfileID != "" {
		if _, err := c.Voice(e.VoiceProfileID, e.VoiceProfileRevision); err != nil {
			return err
		}
	}
	b, _ := json.Marshal(e)
	var copy EntityVersion
	_ = json.Unmarshal(b, &copy)
	c.Entities = append(c.Entities, copy)
	c.Version++
	return nil
}
func (c *EntityCatalog) AddVoice(v VoiceProfile, expected int64) error {
	if c.Version != expected {
		return ErrCatalogConflict
	}
	if err := v.Validate(); err != nil {
		return err
	}
	var latest int64
	for _, old := range c.Voices {
		if old.ID == v.ID && old.Revision > latest {
			latest = old.Revision
		}
	}
	if v.Revision != latest+1 {
		return ErrCatalogConflict
	}
	c.Voices = append(c.Voices, v)
	c.Version++
	return nil
}
