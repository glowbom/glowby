package main

// Video ideas: the project's build agent reads the project and writes a few short suggestions for
// introducing it on video to media/video-ideas.json. Studio shows them under "Words to say" when
// the project is open. Studio never runs the agent itself; it only prepares the request.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	projectVideoIdeasFile    = projectVideoMediaDir + "/video-ideas.json"
	projectVideoIdeasRequest = "video-ideas.request.md"
	projectVideoIdeasLimit   = 64 << 10
	projectVideoIdeasMax     = 12
)

type projectVideoIdea struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Words   string   `json:"words"`
	Scene   string   `json:"scene,omitempty"`
	Seconds int      `json:"seconds,omitempty"`
	Screens []string `json:"screens,omitempty"`
}

func projectVideoText(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if runes := []rune(value); len(runes) > limit {
		value = strings.TrimSpace(string(runes[:limit]))
	}
	return value
}

// Reads the project's ideas, keeping only well-formed entries. ok is false when there is no file yet.
func readProjectVideoIdeas(projectPath string) ([]projectVideoIdea, bool, error) {
	studioProjectMu.Lock()
	project, err := registerStudioProjectLocked(projectPath)
	studioProjectMu.Unlock()
	if err != nil {
		return nil, false, errors.New("Open the project in chat first.")
	}
	if _, err := os.Lstat(filepath.Join(project.Path, filepath.FromSlash(projectVideoIdeasFile))); errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	data, err := readCompanionProjectMedia(project.Path, projectVideoIdeasFile, projectVideoIdeasLimit)
	if err != nil {
		return nil, true, errors.New("The ideas file could not be read. Refresh the ideas to write it again.")
	}
	var file struct {
		Ideas []struct {
			ID      string   `json:"id"`
			Title   string   `json:"title"`
			Words   string   `json:"words"`
			Scene   string   `json:"scene"`
			Seconds float64  `json:"seconds"`
			Screens []string `json:"screens"`
		} `json:"ideas"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, true, errors.New("The ideas file is not valid JSON. Refresh the ideas to write it again.")
	}
	ideas := []projectVideoIdea{}
	for i, raw := range file.Ideas {
		if len(ideas) == projectVideoIdeasMax {
			break
		}
		idea := projectVideoIdea{ID: projectVideoText(raw.ID, 40), Title: projectVideoText(raw.Title, 80), Words: projectVideoText(raw.Words, higgsfieldSpokenTextMaxChars), Scene: projectVideoText(raw.Scene, 600)}
		if idea.Words == "" {
			continue
		}
		if idea.Title == "" {
			idea.Title = fmt.Sprintf("Idea %d", i+1)
		}
		if idea.ID == "" {
			idea.ID = fmt.Sprintf("idea-%d", i+1)
		}
		if raw.Seconds >= 3 && raw.Seconds <= 30 {
			idea.Seconds = int(raw.Seconds + 0.5)
		}
		for _, screen := range raw.Screens {
			if screen = projectVideoText(screen, 120); screen != "" && len(idea.Screens) < 5 {
				idea.Screens = append(idea.Screens, screen)
			}
		}
		ideas = append(ideas, idea)
	}
	return ideas, true, nil
}

const projectVideoIdeasBrief = `# Video ideas for this project

Glowbom Studio shows these ideas under "Words to say" when someone makes a talking video for this project.

Read the project first: the README, glowbom.json, and the app's code and screens if there are any. Then write 3 to 6 short ideas for introducing it on video to ` + "`" + projectVideoIdeasFile + "`" + `. If the file already exists, keep every idea that has ` + "`\"keep\": true`" + ` and replace the rest. Do not change any other file.

## Format

` + "```json" + `
{
  "ideas": [
    {
      "title": "Launch",
      "words": "Meet <product>. <What it does for the person, in one plain sentence>. <What they can do with it right away>.",
      "scene": "The person sits at their desk and talks to the camera, relaxed and smiling.",
      "seconds": 10,
      "screens": ["<the screen that shows the main feature>"]
    }
  ]
}
` + "```" + `

## Rules

- Use the product's name as the README and the app use it. The project folder's name may be a working title.
- ` + "`words`" + ` are said by the person presenting the project, in plain language. Say only what is true of this project: no invented numbers, users, prices or awards. Use "I built" or "I made" only when the project says who made it; otherwise speak about the product.
- Fit the length: about 2.5 words per second, so about 12 words for 5 seconds, 25 for 10 and 37 for 15. ` + "`seconds`" + ` is between 5 and 15.
- Give each idea a different angle, for example the launch, one key feature, the story behind it, or the latest update. Use short titles such as "Launch", "Feature: offline maps" or "Why it exists".
- ` + "`scene`" + ` is one sentence about where the person is and what they do. Never put text on screen in the scene.
- ` + "`screens`" + ` is optional: up to 3 moments in the app that would show the idea best. Name real screens when the project has them; otherwise describe the moment.
- Write in the language of the project's README.
- Reply with the number of ideas you wrote.
`

// Writes the request file and returns the one-line message for the agent.
func prepareProjectVideoIdeas(projectPath string) (string, error) {
	studioProjectMu.Lock()
	project, err := registerStudioProjectLocked(projectPath)
	studioProjectMu.Unlock()
	if err != nil {
		return "", errors.New("Open a Glowbom project in chat first.")
	}
	media, err := chatWriteDirectory(project.Path, projectVideoMediaDir)
	if err != nil {
		return "", errors.New("Could not create the media folder in this project.")
	}
	if err := atomicChatFile(media, projectVideoIdeasRequest, []byte(projectVideoIdeasBrief)); err != nil {
		return "", errors.New("Could not write the request into this project.")
	}
	return fmt.Sprintf("Review this project and suggest how to introduce it on video. Follow `%s/%s` and save the ideas to `%s`. Do not change any other file.", projectVideoMediaDir, projectVideoIdeasRequest, projectVideoIdeasFile), nil
}

// GET /studio/project-video/ideas?path=… → {ideas, exists}. POST {path} → {message} for the agent.
func studioProjectVideoIdeasHandler(w http.ResponseWriter, r *http.Request) {
	method := http.MethodGet
	if r.Method == http.MethodPost {
		method = http.MethodPost
	}
	if !authorizeStudioGeneration(w, r, method) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		ideas, exists, err := readProjectVideoIdeas(r.URL.Query().Get("path"))
		if err != nil {
			writeJSON(w, map[string]any{"ideas": []projectVideoIdea{}, "exists": exists, "error": err.Error()})
			return
		}
		if ideas == nil {
			ideas = []projectVideoIdea{}
		}
		writeJSON(w, map[string]any{"ideas": ideas, "exists": exists})
		return
	}
	var request struct {
		Path string `json:"path"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&request) != nil {
		http.Error(w, "Invalid request.", http.StatusBadRequest)
		return
	}
	message, err := prepareProjectVideoIdeas(request.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"message": message, "file": projectVideoIdeasFile, "requestedAt": time.Now().UnixMilli()})
}
