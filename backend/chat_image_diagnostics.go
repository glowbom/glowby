package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
)

func chatImageGenerationTimeouts(sourceID string, count int) (batch, image time.Duration) {
	batch, image = 5*time.Minute, 2*time.Minute
	switch sourceID {
	case higgsfieldSourceID, "openai-subscription":
		image = studioImageGenerationTimeout(sourceID)
		batch = image * time.Duration(max(1, min(count, maxChatImages)))
	case "glowbom-api":
		image = studioImageGenerationTimeout(sourceID)
		batch = image
	}
	return batch, image
}

// Hash the private record path so logs can group image attempts without storing it.
func prototypeImageDebugID(record string, index int) string {
	digest := sha256.Sum256([]byte(record))
	return fmt.Sprintf("prototype-image-%x-%d", digest[:8], index)
}

// Provider text can contain prompts, private URLs, and account details.
// A local timeout does not establish the outcome of a submitted provider job.
func higgsfieldPrototypeImageFailure(err error) string {
	category := studioDebugErrorCategory(err)
	switch category {
	case "timeout":
		return "Glowbom reached its time limit while waiting for this image. Higgsfield may still finish a submitted request. Check Higgsfield before generating it again."
	case "cancelled":
		return "Glowbom stopped waiting for this image. Higgsfield may still finish a submitted request. Check Higgsfield before generating it again."
	case "provider_moderation":
		return "Higgsfield rejected this image during its content checks. Review the scene and reference image before starting a new request."
	case "provider_credits":
		return "Higgsfield reported insufficient credits for this image. Check your Higgsfield balance before starting a new request."
	case "provider_timeout":
		return "Higgsfield reported that rendering this image timed out."
	case "provider_cancelled":
		return "Higgsfield reported that this image was cancelled."
	case "provider_unavailable":
		return "Higgsfield reported a service error while rendering this image."
	case "provider_rejected":
		return "Higgsfield rejected this image request. Its detailed reason is unavailable here."
	case "provider_failed_unknown":
		return "Higgsfield reported that this image failed. Its detailed reason is unavailable here."
	}
	var notStarted *higgsfieldNotStartedError
	if errors.As(err, &notStarted) {
		return "Higgsfield did not accept this image request. Its detailed reason is unavailable here."
	}
	return "Glowbom could not confirm or save this image. Check Higgsfield before generating it again."
}
