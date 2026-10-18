package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectVideoIdeasReadAndRequest(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	root := studioTestProject(t, "Alien Leap")

	if ideas, exists, err := readProjectVideoIdeas(root); err != nil || exists || len(ideas) != 0 {
		t.Fatalf("a project without ideas has none: %v %v %v", ideas, exists, err)
	}
	message, err := prepareProjectVideoIdeas(root)
	if err != nil || !strings.Contains(message, "media/video-ideas.request.md") || !strings.Contains(message, "media/video-ideas.json") {
		t.Fatalf("message = %q, %v", message, err)
	}
	request, err := os.ReadFile(filepath.Join(root, "media", "video-ideas.request.md"))
	if err != nil || !strings.Contains(string(request), "about 2.5 words per second") || !strings.Contains(string(request), `"keep": true`) || !strings.Contains(string(request), "between 5 and 15") {
		t.Fatalf("request brief missing or incomplete: %v\n%s", err, request)
	}

	file := filepath.Join(root, "media", "video-ideas.json")
	if err := os.WriteFile(file, []byte(`{"ideas":[
		{"title":"Launch","words":"  Meet   Alien Leap. Draw a level, play it. ","scene":"At the desk.","seconds":10,"screens":["Editor","Play","Menu","Shop","Map","Extra"]},
		{"title":"","words":"No title here.","seconds":99},
		{"title":"Empty words","words":""}
	]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ideas, exists, err := readProjectVideoIdeas(root)
	if err != nil || !exists || len(ideas) != 2 {
		t.Fatalf("ideas = %+v %v %v", ideas, exists, err)
	}
	if ideas[0].Words != "Meet Alien Leap. Draw a level, play it." || ideas[0].Seconds != 10 || len(ideas[0].Screens) != 5 || ideas[0].ID == "" {
		t.Fatalf("first idea should be cleaned up: %+v", ideas[0])
	}
	if ideas[1].Title != "Idea 2" || ideas[1].Seconds != 0 {
		t.Fatalf("missing title gets a name and an out-of-range length is dropped: %+v", ideas[1])
	}

	if err := os.WriteFile(file, []byte(`{"ideas": [`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := readProjectVideoIdeas(root); err == nil || !exists || !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("a broken file should say so: %v %v", exists, err)
	}
	if _, _, err := readProjectVideoIdeas(filepath.Join(t.TempDir(), "not-a-project")); err == nil {
		t.Fatalf("a folder that is not a project must be refused")
	}
}
