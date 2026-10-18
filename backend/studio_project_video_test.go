package main

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProjectVideoAnalysisFindsSpeechAndTextColor(t *testing.T) {
	ffmpeg, _, err := studioClipTools()
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	clip := filepath.Join(t.TempDir(), "clip.mp4")
	// One second of tone, one of silence, one of tone, on a bright yellow frame.
	err = exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=c=0xF2E27A:size=320x180:rate=24:d=3",
		"-f", "lavfi", "-i", "sine=frequency=300:duration=3", "-filter_complex", "[1]volume='if(between(t,1,2),0,1)':eval=frame[a]",
		"-map", "0:v", "-map", "[a]", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", clip).Run()
	if err != nil {
		t.Skipf("could not make a test clip: %v", err)
	}
	analysis := analyzeProjectVideo(context.Background(), clip, "Introducing Glowbom.")
	if analysis.Width != 320 || analysis.Height != 180 || !analysis.HasAudio || analysis.DurationSeconds < 2.9 {
		t.Fatalf("facts = %+v", analysis)
	}
	if len(analysis.Speech) != 2 || analysis.Speech[0].End > 1.2 || analysis.Speech[1].Start < 1.8 {
		t.Fatalf("expected two speech stretches around the silence, got %+v", analysis.Speech)
	}
	if analysis.Ink != "black" {
		t.Fatalf("a bright background should get black words: %+v", analysis)
	}
	brief := projectVideoBrief("media/clip.mp4", "media/clip-launch.mp4", "media/.glowbom", "launch", "End on the App Store link.", analysis)
	for _, want := range []string{"`media/clip.mp4`", "\u201cIntroducing Glowbom.\u201d", "**black** words", "tile=6x2", "about 5 px", "Template: Launch video", "End on the App Store link.", "media/.glowbom/person_matte.swift", "`media/clip-launch.mp4`"} {
		if !strings.Contains(brief, want) {
			t.Fatalf("brief is missing %q:\n%s", want, brief)
		}
	}
}

func TestProjectVideoWordsComeFromTheQuotedTitle(t *testing.T) {
	if got := projectVideoWords("\u201cHi, I'm Maria.\u201d"); got != "Hi, I'm Maria." {
		t.Fatalf("words = %q", got)
	}
	if got := projectVideoWords("A man relaxes on a couch"); got != "" {
		t.Fatalf("a description is not spoken words: %q", got)
	}
}

func TestProjectVideoPreparesTheProjectAndImportsTheResultOnce(t *testing.T) {
	t.Setenv("GLOWBOM_STUDIO_DIR", t.TempDir())
	root := studioTestProject(t, "Alien Leap")
	clip, err := saveStudioAsset(studioSaveOptions{Prompt: "“Introducing Glowbom. Draw your idea.”", DataURI: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString([]byte("fake-mp4-bytes")), MediaType: "video", Source: "Higgsfield"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepareProjectVideo(context.Background(), clip.ID, root, "launch", strings.Repeat("x", 2001)); err == nil {
		t.Fatalf("an oversized request should be refused")
	}
	plan, err := prepareProjectVideo(context.Background(), clip.ID, root, "launch", "End on the App Store link.")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(plan.Clip, "media/introducing-glowbom-draw-your-idea-") || !strings.HasSuffix(plan.Output, "-launch.mp4") {
		t.Fatalf("plan = %+v", plan)
	}
	for _, relative := range []string{plan.Clip, plan.Brief, "media/.glowbom-tools/person_matte.swift", "media/.glowbom-tools/render_text.swift"} {
		if _, err := os.Stat(filepath.Join(root, relative)); err != nil {
			t.Fatalf("%s was not written: %v", relative, err)
		}
	}
	brief, _ := os.ReadFile(filepath.Join(root, plan.Brief))
	if !strings.Contains(string(brief), "“Introducing Glowbom. Draw your idea.”") || !strings.Contains(string(brief), "End on the App Store link.") {
		t.Fatalf("brief is missing the words or the request:\n%s", brief)
	}
	if !strings.Contains(plan.Message, plan.Brief) || !strings.Contains(plan.Message, plan.Output) {
		t.Fatalf("message should name the brief and the output: %q", plan.Message)
	}

	if asset, err := importProjectVideoResult(context.Background(), root, plan.Output, 0); err != nil || asset != nil {
		t.Fatalf("before the agent saves the video the result is pending: %+v %v", asset, err)
	}
	output := filepath.Join(root, filepath.FromSlash(plan.Output))
	// Without ffmpeg the probe is skipped and any bytes count; with it, only a real video does.
	if ffmpeg, _, err := studioClipTools(); err == nil {
		if err := os.WriteFile(output, []byte("half-written"), 0o600); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-time.Minute)
		_ = os.Chtimes(output, old, old)
		if asset, err := importProjectVideoResult(context.Background(), root, plan.Output, 0); err != nil || asset != nil {
			t.Fatalf("a file that is not a playable video must stay pending: %+v %v", asset, err)
		}
		if err := exec.Command(ffmpeg, "-v", "error", "-y", "-f", "lavfi", "-i", "color=c=black:size=180x320:rate=24:d=1", "-c:v", "libx264", "-pix_fmt", "yuv420p", output).Run(); err != nil {
			t.Fatal(err)
		}
	} else if err := os.WriteFile(output, []byte("finished-mp4"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	_ = os.Chtimes(output, old, old)
	first, err := importProjectVideoResult(context.Background(), root, plan.Output, 0)
	if err != nil || first == nil || first.SourceService != "Project video" || len(first.UsedInProjects) != 1 {
		t.Fatalf("the finished video should be added to Studio: %+v %v", first, err)
	}
	if _, _, err := studioClipTools(); err == nil && (first.Duration == nil || first.Dimensions == nil) {
		t.Fatalf("the finished video should be added to Studio: %+v %v", first, err)
	}
	second, err := importProjectVideoResult(context.Background(), root, plan.Output, 0)
	if err != nil || second == nil || second.ID != first.ID {
		t.Fatalf("checking again must not add a second copy: %+v %v", second, err)
	}
	// A second run on the same clip gets its own output, and the earlier file does not count for it.
	again, err := prepareProjectVideo(context.Background(), clip.ID, root, "launch", "")
	if err != nil || again.Output == plan.Output || !strings.HasSuffix(again.Output, "-launch-2.mp4") {
		t.Fatalf("second run should get a new output: %+v %v", again, err)
	}
	if asset, err := importProjectVideoResult(context.Background(), root, plan.Output, again.PreparedAt); err != nil || asset != nil {
		t.Fatalf("a file from before the request must not be picked up: %+v %v", asset, err)
	}
	for _, bad := range []string{"media/../secret.mp4", "../media/x.mp4", "media/x.mov", "other/x.mp4"} {
		if _, err := importProjectVideoResult(context.Background(), root, bad, 0); err == nil {
			t.Fatalf("%q must be refused", bad)
		}
	}
}

func TestProjectVideoAnalysisFollowsPhoneRotation(t *testing.T) {
	ffmpeg, _, err := studioClipTools()
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	dir := t.TempDir()
	flat, rotated := filepath.Join(dir, "flat.mp4"), filepath.Join(dir, "rotated.mp4")
	if err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=c=gray:size=320x180:rate=24:d=1", "-c:v", "libx264", "-pix_fmt", "yuv420p", flat).Run(); err != nil {
		t.Skipf("could not make a test clip: %v", err)
	}
	if err := exec.Command(ffmpeg, "-v", "error", "-display_rotation:v:0", "90", "-i", flat, "-c", "copy", rotated).Run(); err != nil {
		t.Skipf("this ffmpeg cannot tag a rotation: %v", err)
	}
	analysis := analyzeProjectVideo(context.Background(), rotated, "")
	if analysis.Width != 180 || analysis.Height != 320 {
		t.Fatalf("a portrait phone clip should be described as it plays: %dx%d", analysis.Width, analysis.Height)
	}
}
