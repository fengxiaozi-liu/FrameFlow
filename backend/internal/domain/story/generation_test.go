package story

import "testing"

func TestGenerationTargetsRemainSeparate(t *testing.T) {
	_, system, user, err := BuildGenerationPrompt(CandidateStory, "写一个清晨故事", "")
	if err != nil || system == "" || user == "" {
		t.Fatalf("story prompt: %v", err)
	}
	if _, _, _, err = BuildGenerationPrompt(CandidateStoryboard, "", "  "); err == nil {
		t.Fatal("blank body accepted for storyboard generation")
	}
	storyResult, err := ParseCandidateResult(CandidateStory, `{"body":"正文"}`)
	if err != nil || storyResult.Body != "正文" || len(storyResult.Scenes) != 0 {
		t.Fatalf("story parse: %+v, %v", storyResult, err)
	}
	board, err := ParseCandidateResult(CandidateStoryboard, `{"scenes":[{"id":"model-id","order":99,"title":"镜头","visual_prompt":"光从窗边进入","narration":"","duration_seconds":5}]}`)
	if err != nil || len(board.Scenes) != 1 || board.Scenes[0].ID != "" || board.Scenes[0].Order != 1 {
		t.Fatalf("board parse: %+v, %v", board, err)
	}
	for _, bad := range []string{`{"body":"正文","scenes":[{"title":"x"}]}`, "```json\n{}\n```", `{"scenes":[]}`, `{"body":"正文","unexpected":true}`} {
		if _, err := ParseCandidateResult(CandidateStory, bad); err == nil {
			t.Fatalf("invalid candidate accepted: %s", bad)
		}
	}
}
