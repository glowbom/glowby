package main

// Only descriptive settings are shared. Credentials and uploaded bytes never
// enter the Desktop activity feed.
type companionMediaDetails struct {
	Prompt          string `json:"prompt,omitempty"`
	SourceID        string `json:"sourceId,omitempty"`
	ModelID         string `json:"modelId,omitempty"`
	AspectRatio     string `json:"aspectRatio,omitempty"`
	Resolution      string `json:"resolution,omitempty"`
	Quality         string `json:"quality,omitempty"`
	DurationSeconds int    `json:"durationSeconds,omitempty"`
	SpokenText      string `json:"spokenText,omitempty"`
	ReferenceID     string `json:"referenceId,omitempty"`
	Stage           string `json:"stage"`
}

type companionMediaProgressKey struct{}

func (j *companionJob) mediaStage(stage string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.media != nil && j.status == "running" {
		j.media.Stage = stage
	}
}

func (j *companionJob) mediaImageOptions(options studioImageOptions) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.media != nil {
		j.media.ModelID, j.media.Resolution, j.media.Quality = options.ModelID, options.Resolution, options.Quality
	}
}
