package application

import (
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

type MaterialService struct {
	Repo      material.Repository
	UploadDir string
}

func (s MaterialService) Save(g *gin.Context) {
	var asset material.Asset
	if err := g.Bind(&asset); err != nil {
		return
	}
	code := 201
	if id := g.Param("id"); id != "" {
		asset.ID = id
		code = 200
	} else if asset.ID == "" {
		asset.ID = time.Now().UTC().Format("20060102150405.000000000")
	}
	if err := s.Repo.Save(g.Request.Context(), asset); err != nil {
		writeError(g, err)
		return
	}
	g.JSON(code, asset)
}
func (s MaterialService) List(g *gin.Context) {
	items, err := s.Repo.List(g.Request.Context(), material.Kind(g.Query("kind")))
	if err != nil {
		writeError(g, err)
		return
	}
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
	err := s.Repo.Delete(g.Request.Context(), g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	g.Status(http.StatusNoContent)
}
