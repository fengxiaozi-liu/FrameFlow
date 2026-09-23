package protocol

type WanxiangVideoRequest struct {
	Model string `json:"model"`
	Input struct {
		Prompt string `json:"prompt"`
		Media  []struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"media"`
	} `json:"input"`
	Parameters struct {
		Resolution string `json:"resolution,omitempty"`
		Duration   int    `json:"duration,omitempty"`
	} `json:"parameters"`
}

type WanxiangVideoResponse struct {
	Output struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
		VideoURL   string `json:"video_url"`
	} `json:"output"`
}
