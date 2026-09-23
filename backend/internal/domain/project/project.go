package project

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"strings"
	"time"
)

type Draft struct {
	ID                   string               `json:"id"`
	ProjectID            string               `json:"project_id"`
	Name                 string               `json:"name"`
	Version              int64                `json:"version"`
	Story                story.Document       `json:"story"`
	StoryboardSourceBody string               `json:"storyboard_source_body,omitempty"`
	Candidates           []story.Candidate    `json:"candidates,omitempty"`
	StoryboardSnapshots  []StoryboardSnapshot `json:"storyboard_snapshots,omitempty"`
	BodySnapshots        []BodySnapshot       `json:"body_snapshots,omitempty"`
	Bindings             []SceneBinding       `json:"bindings,omitempty"`
	VideoVersions        []VideoVersion       `json:"video_versions,omitempty"`
	SelectedVersions     map[string]string    `json:"selected_versions,omitempty"`
	Compositions         []Composition        `json:"compositions,omitempty"`
	Outputs              []DraftOutput        `json:"outputs,omitempty"`
	CreatedAt            time.Time            `json:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
}

type SceneBinding struct {
	SceneID    string `json:"scene_id"`
	MaterialID string `json:"material_id"`
	Usage      string `json:"usage"`
	Position   int    `json:"position,omitempty"`
}

type VideoVersion struct {
	ID               string    `json:"id"`
	SceneID          string    `json:"scene_id"`
	TaskID           string    `json:"task_id"`
	LocalMediaPath   string    `json:"local_media_path"`
	DurationSeconds  float64   `json:"duration_seconds"`
	InputFingerprint string    `json:"input_fingerprint"`
	CreatedAt        time.Time `json:"created_at"`
}

type Composition struct {
	ID               string    `json:"id"`
	TaskID           string    `json:"task_id"`
	ClipVersionIDs   []string  `json:"clip_version_ids"`
	MusicMaterialID  string    `json:"music_material_id,omitempty"`
	MusicVolume      float64   `json:"music_volume"`
	SourceVolume     float64   `json:"source_volume"`
	VoiceVolume      float64   `json:"voice_volume"`
	AspectRatio      string    `json:"aspect_ratio"`
	Resolution       string    `json:"resolution"`
	LocalMediaPath   string    `json:"local_media_path,omitempty"`
	InputFingerprint string    `json:"input_fingerprint"`
	CreatedAt        time.Time `json:"created_at"`
}

type StoryboardSnapshot struct {
	ID               string            `json:"id"`
	SourceBody       string            `json:"source_body,omitempty"`
	Scenes           []story.Scene     `json:"scenes"`
	Bindings         []SceneBinding    `json:"bindings,omitempty"`
	VideoVersions    []VideoVersion    `json:"video_versions,omitempty"`
	SelectedVersions map[string]string `json:"selected_versions,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
}

type BodySnapshot struct {
	ID          string    `json:"id"`
	Body        string    `json:"body"`
	AppliedBody string    `json:"applied_body"`
	CreatedAt   time.Time `json:"created_at"`
}

var ErrVersionConflict = errors.New("draft version conflict")

func (d *Draft) AddCandidate(candidate story.Candidate) error {
	if candidate.ID == "" || candidate.Target != story.CandidateStory && candidate.Target != story.CandidateStoryboard {
		return errors.New("candidate identity and target are required")
	}
	for _, existing := range d.Candidates {
		if existing.ID == candidate.ID {
			return errors.New("candidate already exists")
		}
	}
	d.Candidates = append(d.Candidates, candidate)
	return nil
}

func (d *Draft) CaptureStoryboardSnapshot(id string, now time.Time) error {
	if id == "" {
		return errors.New("snapshot id is required")
	}
	for _, existing := range d.StoryboardSnapshots {
		if existing.ID == id {
			return errors.New("snapshot already exists")
		}
	}
	selected := make(map[string]string, len(d.SelectedVersions))
	for sceneID, versionID := range d.SelectedVersions {
		selected[sceneID] = versionID
	}
	d.StoryboardSnapshots = append(d.StoryboardSnapshots, StoryboardSnapshot{
		ID: id, SourceBody: d.StoryboardSourceBody, Scenes: append([]story.Scene{}, d.Story.Scenes...),
		Bindings:         append([]SceneBinding(nil), d.Bindings...),
		VideoVersions:    append([]VideoVersion(nil), d.VideoVersions...),
		SelectedVersions: selected, CreatedAt: now,
	})
	return nil
}

func (d *Draft) HasScene(id string) bool {
	for _, scene := range d.Story.Scenes {
		if scene.ID == id {
			return true
		}
	}
	return false
}

func (d *Draft) CopyScene(id string) (story.Scene, error) {
	copied, err := d.Story.CopyScene(id)
	if err != nil {
		return story.Scene{}, err
	}
	for _, binding := range append([]SceneBinding(nil), d.Bindings...) {
		if binding.SceneID == id {
			binding.SceneID = copied.ID
			d.Bindings = append(d.Bindings, binding)
		}
	}
	return copied, nil
}

func (d *Draft) AddBinding(binding SceneBinding) error {
	if !d.HasScene(binding.SceneID) || binding.MaterialID == "" || binding.Usage == "" {
		return errors.New("binding requires a scene, material and usage")
	}
	for _, existing := range d.Bindings {
		if existing.SceneID == binding.SceneID && existing.MaterialID == binding.MaterialID && existing.Usage == binding.Usage {
			return errors.New("binding already exists")
		}
	}
	d.Bindings = append(d.Bindings, binding)
	return nil
}

func (d *Draft) AddVideoVersion(version VideoVersion) error {
	if !d.HasScene(version.SceneID) || version.ID == "" || version.TaskID == "" || version.LocalMediaPath == "" {
		return errors.New("video version requires existing scene, task and saved media")
	}
	for _, existing := range d.VideoVersions {
		if existing.ID == version.ID || existing.TaskID == version.TaskID {
			return errors.New("video version already exists")
		}
	}
	d.VideoVersions = append(d.VideoVersions, version)
	return nil
}

func (d *Draft) SelectVideoVersion(sceneID, versionID string) error {
	if !d.HasScene(sceneID) {
		return errors.New("scene not found")
	}
	for _, version := range d.VideoVersions {
		if version.ID == versionID && version.SceneID == sceneID {
			if d.SelectedVersions == nil {
				d.SelectedVersions = make(map[string]string)
			}
			d.SelectedVersions[sceneID] = versionID
			return nil
		}
	}
	return errors.New("video version not found in scene")
}

func (d *Draft) AddComposition(value Composition) error {
	if value.ID == "" || value.TaskID == "" || len(value.ClipVersionIDs) != len(d.Story.Scenes) {
		return errors.New("composition requires every scene")
	}
	for i, scene := range d.Story.Scenes {
		found := false
		for _, version := range d.VideoVersions {
			if version.ID == value.ClipVersionIDs[i] && version.SceneID == scene.ID {
				found = true
				break
			}
		}
		if !found {
			return errors.New("composition clip does not belong to scene order")
		}
	}
	for _, existing := range d.Compositions {
		if existing.ID == value.ID || existing.TaskID == value.TaskID {
			return errors.New("composition already exists")
		}
	}
	d.Compositions = append(d.Compositions, value)
	return nil
}

// UpdateDraftBody changes a saved draft only when the caller edited its latest
// version. Legacy drafts without a version begin at zero.
func (p *Project) UpdateDraftBody(draftID string, expectedVersion int64, body string, now time.Time) (Draft, error) {
	for i := range p.Drafts {
		draft := &p.Drafts[i]
		if draft.ID != draftID {
			continue
		}
		if draft.Version != expectedVersion {
			return *draft, ErrVersionConflict
		}
		draft.Story.Body = body // An empty editor is valid; generation validates nonblank input.
		draft.Story.UpdatedAt = now
		draft.Version++
		draft.UpdatedAt = now
		return *draft, nil
	}
	return Draft{}, errors.New("draft not found in project")
}

type DraftOutput struct {
	TaskID    string    `json:"task_id"`
	Kind      string    `json:"kind"`
	Text      string    `json:"text,omitempty"`
	URL       string    `json:"url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Drafts    []Draft   `json:"drafts"`
	CreatedAt time.Time `json:"created_at"`
}

func New(id, name string, now time.Time) (Project, error) {
	if id == "" || strings.TrimSpace(name) == "" {
		return Project{}, errors.New("project name is required")
	}
	return Project{ID: id, Name: name, CreatedAt: now}, nil
}

func (p *Project) AddDraft(d Draft) error {
	if d.ID == "" {
		return errors.New("draft id is required")
	}
	d.ProjectID = p.ID
	d.UpdatedAt = p.CreatedAt
	p.Drafts = append(p.Drafts, d)
	return nil
}

func (p *Project) HasDraft(id string) bool {
	for i := range p.Drafts {
		if p.Drafts[i].ID == id {
			return true
		}
	}
	return false
}

// ApplyTaskResult updates exactly one draft and is idempotent by task ID.
func (p *Project) ApplyTaskResult(draftID string, output DraftOutput, now time.Time) error {
	if output.TaskID == "" || output.Kind == "" {
		return errors.New("task result identity is required")
	}
	for i := range p.Drafts {
		draft := &p.Drafts[i]
		if draft.ID != draftID {
			continue
		}
		for _, existing := range draft.Outputs {
			if existing.TaskID == output.TaskID {
				return nil
			}
		}
		if output.Kind == "story" {
			if err := draft.Story.UpdateBody(output.Text, now); err != nil {
				return err
			}
		} else if strings.TrimSpace(output.URL) == "" {
			return errors.New("media task result URL is required")
		}
		output.CreatedAt = now
		draft.Outputs = append(draft.Outputs, output)
		draft.UpdatedAt = now
		return nil
	}
	return errors.New("draft not found in project")
}
