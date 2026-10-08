package application

import (
	"errors"
	"fmt"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"strings"
)

type ResolvedBinding struct {
	Binding     story.EntityBinding
	Entity      project.EntityVersion
	Description string
	References  []project.ReferenceImage
}
type ResolvedContext struct {
	Prompt   string
	Entities []ResolvedBinding
	Voices   map[string]project.VoiceProfile
}

func ResolveBindings(c project.EntityCatalog, scene story.Scene, needSpeech bool) (ResolvedContext, error) {
	out := ResolvedContext{Voices: map[string]project.VoiceProfile{}}
	if err := story.ValidateBindings(scene.EntityBindings); err != nil {
		return out, err
	}
	names := map[string]string{}
	characters := map[string]bool{}
	for _, e := range c.Entities {
		if e.Kind == "character" {
			characters[e.ID] = true
		}
	}
	for _, b := range scene.EntityBindings {
		e, err := c.Entity(b.EntityID, b.EntityRevision)
		if err != nil {
			return out, fmt.Errorf("binding %s: %w", b.ID, err)
		}
		if (e.Kind == "character") != (b.Role == "cast") {
			return out, errors.New("binding role does not match entity type")
		}
		names[b.ID] = e.Name
		description := e.Description
		if b.DescriptionOverride != nil {
			description = *b.DescriptionOverride
		}
		if b.DescriptionSupplement != "" {
			description += "\n本镜头补充：" + b.DescriptionSupplement
		}
		refs := e.References
		if b.SelectedMaterialIDs != nil {
			refs = []project.ReferenceImage{}
			for i, id := range *b.SelectedMaterialIDs {
				found := false
				for _, r := range e.References {
					if r.MaterialID == id {
						r.Order = i
						refs = append(refs, r)
						found = true
						break
					}
				}
				if !found {
					return out, fmt.Errorf("reference %s does not belong to entity %s", id, e.ID)
				}
			}
		}
		if b.ReferenceMaterialIDs != nil {
			refs = []project.ReferenceImage{}
			for i, id := range *b.ReferenceMaterialIDs {
				ref := project.ReferenceImage{MaterialID: id, Order: i, Label: "镜头参考"}
				for _, base := range e.References {
					if base.MaterialID == id {
						ref.Label = base.Label
						break
					}
				}
				refs = append(refs, ref)
			}
		}
		out.Entities = append(out.Entities, ResolvedBinding{Binding: b, Entity: e, Description: description, References: refs})
	}
	out.Prompt = scene.VisualPrompt
	if scene.PromptDocument != nil {
		var err error
		out.Prompt, err = scene.PromptDocument.Render(names)
		if err != nil {
			return out, err
		}
	}
	if err := story.ValidateUtterances(scene.Utterances, characters); err != nil {
		return out, err
	}
	for _, u := range scene.Utterances {
		id, revision := u.VoiceProfileID, u.VoiceProfileRevision
		if u.SpeakerType == "character" {
			e, err := c.Entity(u.SpeakerID, u.SpeakerRevision)
			if err != nil || e.Kind != "character" {
				return out, errors.New("speaker_unresolved")
			}
			if id == "" {
				id, revision = e.VoiceProfileID, e.VoiceProfileRevision
			}
		}
		if id == "" {
			if needSpeech {
				return out, fmt.Errorf("utterance %s: voice_unavailable", u.ID)
			}
			continue
		}
		v, err := c.Voice(id, revision)
		if err != nil {
			return out, err
		}
		out.Voices[u.ID] = v
	}
	if strings.TrimSpace(out.Prompt) == "" && needSpeech && len(scene.Utterances) == 0 {
		return out, errors.New("empty generation input")
	}
	return out, nil
}
