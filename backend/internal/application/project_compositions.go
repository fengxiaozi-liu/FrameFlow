package application

import (
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (s ProjectService) ListCompositions(g *gin.Context) {
	p, err := s.Repo.Get(g.Request.Context(), g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	d, err := draftByID(&p, g.Param("draftId"))
	if err != nil {
		writeError(g, err)
		return
	}
	values := d.Compositions
	if values == nil {
		values = []project.Composition{}
	}
	g.JSON(http.StatusOK, gin.H{"compositions": values})
}
