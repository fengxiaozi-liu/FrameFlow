package story

import "testing"

func TestSupplementsMigrateOnceAndRemainEditable(t *testing.T) {
	s := Scene{VisualPrompt: "雨夜", EntityBindings: []EntityBinding{{ID: "b", DescriptionSupplement: "湿发"}, {ID: "c", DescriptionSupplement: "暖光"}}}
	if !s.MergeDescriptionSupplements() {
		t.Fatal("not migrated")
	}
	got, err := s.PromptDocument.Render(map[string]string{"b": "林夏", "c": "咖啡馆"})
	if err != nil || got != "雨夜\n林夏：湿发\n咖啡馆：暖光" {
		t.Fatal(got, err)
	}
	if s.MergeDescriptionSupplements() || s.EntityBindings[0].DescriptionSupplement != "" {
		t.Fatal("migration duplicated")
	}
	s.PromptDocument.Nodes = []PromptNode{{Type: "text", Text: "用户改写"}}
	s.MergeDescriptionSupplements()
	got, _ = s.PromptDocument.Render(nil)
	if got != "用户改写" {
		t.Fatal(got)
	}
}
