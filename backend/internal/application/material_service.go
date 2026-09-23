package application

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/gin-gonic/gin"
	"net/http"
	"sort"
	"strings"
	"time"
)

type MaterialService struct {
	Repo      material.Repository
	Projects  project.Repository
	Tasks     task.Repository
	UploadDir string
}

func (s MaterialService) Save(g *gin.Context) {
	var asset material.Asset
	if err := g.Bind(&asset); err != nil {
		return
	}
	code := 201
	if id := g.Param("id"); id != "" {
		stored, err := s.Repo.Get(g.Request.Context(), id)
		if err != nil {
			writeError(g, err)
			return
		}
		asset.ID = id
		asset.URL = stored.URL
		asset.CreatedAt = stored.CreatedAt
		asset.Media = stored.Media
		asset.LegacyKind = stored.LegacyKind
		code = 200
	} else if asset.ID == "" {
		asset.ID = time.Now().UTC().Format("20060102150405.000000000")
	}
	if code == 201 {
		asset.NormalizeLegacyKind()
	}
	if strings.TrimSpace(asset.Name) == "" || !validMaterialKind(asset.Kind) {
		writeError(g, Invalid("invalid_material", errors.New("name and a supported category are required")))
		return
	}
	if (code == 200 && (asset.Kind == material.Visual || asset.Kind == material.Frame)) ||
		(strings.HasPrefix(asset.Media.Format, "image/") && (asset.Kind == material.Voice || asset.Kind == material.Music)) ||
		(strings.HasPrefix(asset.Media.Format, "audio/") && (asset.Kind == material.Scene || asset.Kind == material.Character || asset.Kind == material.Prop)) {
		writeError(g, Invalid("invalid_material", errors.New("category must match stored media type")))
		return
	}
	if err := s.Repo.Save(g.Request.Context(), asset); err != nil {
		writeError(g, err)
		return
	}
	g.JSON(code, asset)
}
func (s MaterialService) List(g *gin.Context) {
	requestedKind := material.Kind(g.Query("kind"))
	if requestedKind != "" && !validMaterialKind(requestedKind) {
		writeError(g, Invalid("invalid_material_kind", errors.New("unsupported material category")))
		return
	}
	items, err := s.Repo.List(g.Request.Context(), requestedKind)
	if err != nil {
		writeError(g, err)
		return
	}
	query := strings.ToLower(strings.TrimSpace(g.Query("q")))
	mediaType := g.Query("media_type")
	filtered := items[:0]
	for _, item := range items {
		if mediaType == "image" && item.Kind != material.Scene && item.Kind != material.Character && item.Kind != material.Prop || mediaType == "audio" && item.Kind != material.Voice && item.Kind != material.Music {
			continue
		}
		matched := query == "" || strings.Contains(strings.ToLower(item.Name), query)
		for _, tag := range item.Tags {
			matched = matched || strings.Contains(strings.ToLower(tag), query)
		}
		if matched {
			filtered = append(filtered, item)
		}
	}
	items = filtered
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	g.JSON(http.StatusOK, gin.H{
		"materials": items,
	})
}
func (s MaterialService) Get(g *gin.Context) {
	item, err := s.Repo.Get(g.Request.Context(), g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	g.JSON(http.StatusOK, item)
}
func (s MaterialService) Delete(g *gin.Context) {
	item, err := s.Repo.Get(g.Request.Context(), g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	uses, err := s.usageLocations(g, item)
	if err != nil {
		writeError(g, err)
		return
	}
	if len(uses) > 0 {
		g.JSON(http.StatusConflict, gin.H{"code": "material_in_use", "uses": uses})
		return
	}
	err = s.Repo.Delete(g.Request.Context(), g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	g.Status(http.StatusNoContent)
}

func validMaterialKind(kind material.Kind) bool {
	switch kind {
	case material.Scene, material.Prop, material.Character, material.Voice, material.Music, material.Visual, material.Frame:
		return true
	default:
		return false
	}
}

func (s MaterialService) usageLocations(g *gin.Context, item material.Asset) ([]string, error) {
	uses := []string{}
	if s.Projects != nil {
		projects, err := s.Projects.List(g.Request.Context())
		if err != nil {
			return nil, err
		}
		for _, p := range projects {
			for _, d := range p.Drafts {
				for _, binding := range d.Bindings {
					if binding.MaterialID == item.ID {
						uses = append(uses, p.ID+"/"+d.ID+"/"+binding.SceneID)
					}
				}
				for _, snapshot := range d.StoryboardSnapshots {
					for _, binding := range snapshot.Bindings {
						if binding.MaterialID == item.ID {
							uses = append(uses, p.ID+"/"+d.ID+"/snapshot/"+snapshot.ID)
						}
					}
				}
				for _, composition := range d.Compositions {
					if composition.MusicMaterialID == item.ID {
						uses = append(uses, p.ID+"/"+d.ID+"/composition/"+composition.ID)
					}
				}
			}
		}
	}
	if s.Tasks != nil {
		tasks, err := s.Tasks.List(g.Request.Context())
		if err != nil {
			return nil, err
		}
		for _, value := range tasks {
			if value.Input.SourceImageURL == item.URL {
				uses = append(uses, "task/"+value.ID)
			}
		}
	}
	return uses, nil
}

func (s MaterialService) Usage(g *gin.Context) {
	item, err := s.Repo.Get(g.Request.Context(), g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	uses, err := s.usageLocations(g, item)
	if err != nil {
		writeError(g, err)
		return
	}
	g.JSON(http.StatusOK, gin.H{"uses": uses})
}
