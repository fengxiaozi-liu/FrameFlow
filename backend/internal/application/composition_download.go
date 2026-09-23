package application

import (
	"errors"
	"github.com/gin-gonic/gin"
)

func (s TaskService) DownloadComposition(g *gin.Context) {
	p, err := s.Projects.Get(g.Request.Context(), g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	d, err := draftByID(&p, g.Param("draftId"))
	if err != nil {
		writeError(g, err)
		return
	}
	for _, value := range d.Compositions {
		if value.ID != g.Param("compositionId") {
			continue
		}
		path, err := localMediaPath(s.UploadDir, value.LocalMediaPath)
		if err != nil {
			writeError(g, Conflict("composition_file_missing", err))
			return
		}
		g.Header("X-Content-Type-Options", "nosniff")
		g.FileAttachment(path, "frameflow-"+value.ID+".mp4")
		return
	}
	writeError(g, Invalid("composition_not_found", errors.New("composition not found")))
}
