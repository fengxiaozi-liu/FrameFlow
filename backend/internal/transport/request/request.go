package request

import (
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/transport/response"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
)

func BindJSON(c *gin.Context, input any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(input)
	if err == nil {
		var extra any
		if e := decoder.Decode(&extra); e != io.EOF {
			if e == nil {
				e = io.ErrUnexpectedEOF
			}
			err = e
		}
	}
	if err != nil {
		response.Set(c, 0, nil, &fault.Error{Kind: "invalid", Code: "invalid_request", Cause: err})
		return false
	}
	return true
}
