package story

import (
	"errors"
	"strings"
)

type Utterance struct {
	ID                   string `json:"id"`
	SpeakerType          string `json:"speaker_type"`
	SpeakerID            string `json:"speaker_id,omitempty"`
	SpeakerRevision      int64  `json:"speaker_revision,omitempty"`
	Text                 string `json:"text"`
	Order                int    `json:"order"`
	Emotion              string `json:"emotion,omitempty"`
	VoiceProfileID       string `json:"voice_profile_id,omitempty"`
	VoiceProfileRevision int64  `json:"voice_profile_revision,omitempty"`
	PauseAfterMS         int    `json:"pause_after_ms,omitempty"`
}

func ValidateUtterances(lines []Utterance, characters map[string]bool) error {
	seen := map[string]bool{}
	for i, u := range lines {
		if u.ID == "" || seen[u.ID] || u.Order != i || strings.TrimSpace(u.Text) == "" || u.PauseAfterMS < 0 || u.PauseAfterMS > 10000 {
			return errors.New("invalid utterance")
		}
		seen[u.ID] = true
		switch u.SpeakerType {
		case "character":
			if !characters[u.SpeakerID] || u.SpeakerRevision < 1 {
				return errors.New("speaker_unresolved")
			}
		case "narrator":
			if u.SpeakerID != "" {
				return errors.New("narrator cannot reference character")
			}
		default:
			return errors.New("unknown speaker type")
		}
		if (u.VoiceProfileID == "") != (u.VoiceProfileRevision == 0) {
			return errors.New("voice identity and revision required together")
		}
	}
	return nil
}
