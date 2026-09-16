package application

import (
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/transport/request"
	"github.com/fengxiaozi-liu/FrameFlow/internal/transport/response"
	"github.com/gin-gonic/gin"
	"time"
)

type MaterialService struct {
	Repo      material.Repository
	UploadDir string
}

func (s MaterialService) Save(g *gin.Context) {
	var asset material.Asset
	if !request.BindJSON(g, &asset) {
		return
	}
	code := 201
	if id := g.Param("id"); id != "" {
		asset.ID = id
		code = 200
	} else if asset.ID == "" {
		asset.ID = time.Now().UTC().Format("20060102150405.000000000")
	}
	err := s.Repo.Save(g.Request.Context(), asset)
	response.Set(g, code, asset, err)
}
func (s MaterialService) List(g *gin.Context) {
	items, err := s.Repo.List(g.Request.Context(), material.Kind(g.Query("kind")))
	response.Set(g, 200, gin.H{"materials": items}, err)
}
func (s MaterialService) Get(g *gin.Context) {
	item, err := s.Repo.Get(g.Request.Context(), g.Param("id"))
	response.Set(g, 200, item, err)
}
func (s MaterialService) Delete(g *gin.Context) {
	err := s.Repo.Delete(g.Request.Context(), g.Param("id"))
	response.Set(g, 204, nil, err)
}
