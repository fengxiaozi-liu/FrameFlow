package application

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/gin-gonic/gin"
)

func (s ProjectService) WriteVoice(g *gin.Context) {
	var in struct {
		Expected *int64               `json:"expected_catalog_version"`
		Voice    project.VoiceProfile `json:"voice_profile"`
	}
	if err := g.ShouldBindJSON(&in); err != nil || in.Expected == nil {
		writeError(g, Invalid("invalid_voice", errors.New("expected_catalog_version required")))
		return
	}
	if id := g.Param("voiceId"); id != "" && id != in.Voice.ID {
		writeError(g, Invalid("invalid_voice", errors.New("voice identity mismatch")))
		return
	}
	if s.Models == nil {
		writeError(g, errors.New("model repository unavailable"))
		return
	}
	m, modelErr := s.Models.GetModel(g.Request.Context(), in.Voice.ModelID)
	if modelErr != nil || m.ConnectionID != in.Voice.ConnectionID || !m.Supports(provider.Speech) {
		writeError(g, Invalid("voice_unavailable", errors.New("语音模型或连接不可用")))
		return
	}
	cap, known := provider.InputCapabilityFor(m.RemoteID)
	valid := false
	if known {
		for _, v := range cap.Voices {
			if v.ID == in.Voice.RemoteVoiceID {
				for _, l := range v.Languages {
					if l == in.Voice.Language {
						valid = true
					}
				}
			}
		}
	}
	if !valid || in.Voice.Instructions != "" {
		writeError(g, Invalid("voice_unavailable", errors.New("音色、语言或指令不受当前模型支持")))
		return
	}
	atomic, ok := s.Repo.(project.AtomicRepository)
	if !ok {
		writeError(g, errors.New("atomic repository required"))
		return
	}
	p, err := atomic.Update(g.Request.Context(), g.Param("id"), func(p *project.Project) error { return p.AddVoice(in.Voice, *in.Expected) })
	if errors.Is(err, project.ErrCatalogConflict) {
		g.JSON(409, gin.H{"code": "entity_revision_conflict", "message": "音色档案已更新", "catalog": p.EntityCatalog})
		return
	}
	if err != nil {
		writeError(g, Invalid("invalid_voice", err))
		return
	}
	g.JSON(200, p.EntityCatalog)
}
func (s CatalogService) InputCapability(g *gin.Context) {
	m, err := s.Models.GetModel(g.Request.Context(), g.Param("model"))
	if err != nil {
		writeError(g, err)
		return
	}
	c, ok := provider.InputCapabilityFor(m.RemoteID)
	if !ok || !m.Enabled || !m.Supported {
		writeError(g, Invalid("capability_unsupported", errors.New("模型输入能力尚未验证")))
		return
	}
	c.ModelID = m.ID
	g.JSON(200, c)
}
