package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"google.golang.org/genai"
)

// MARK: - Request/Response Types

type VeoImageInput struct {
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

type VeoVideoAsset struct {
	URI         string `json:"uri"`
	AspectRatio string `json:"aspectRatio"`
}

type VeoGenerationRequest struct {
	Prompt          string          `json:"prompt"`
	Images          []VeoImageInput `json:"images"`
	AspectRatio     string          `json:"aspectRatio"`
	UseKeyframes    bool            `json:"useKeyframes"`
	ExtensionSource *VeoVideoAsset  `json:"extensionSource"`
	DurationSeconds int             `json:"durationSeconds,omitempty"`
	Resolution      string          `json:"resolution,omitempty"`
	VideoSource     string          `json:"videoSource,omitempty"`
	GeminiKey       string          `json:"geminiKey"`
	XaiKey          string          `json:"xaiKey,omitempty"`
}

type VeoGenerationResponse struct {
	OperationID string `json:"operationId"`
	Message     string `json:"message"`
}

type VeoPollRequest struct {
	OperationID string `json:"operationId"`
	VideoSource string `json:"videoSource,omitempty"`
	GeminiKey   string `json:"geminiKey"`
	XaiKey      string `json:"xaiKey,omitempty"`
}

type VeoPollResponse struct {
	Done       bool           `json:"done"`
	Status     string         `json:"status"`
	VideoURL   string         `json:"videoUrl,omitempty"`
	VideoAsset *VeoVideoAsset `json:"videoAsset,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// MARK: - Core Functions (Direct translation from TypeScript)

// startVeoVideoGeneration - Direct translation of generateVeoVideo from veoService.ts
func startVeoVideoGeneration(req VeoGenerationRequest) (*VeoGenerationResponse, error) {
	if req.GeminiKey == "" {
		return nil, fmt.Errorf("no Gemini API key provided")
	}

	if len(req.Images) == 0 && req.ExtensionSource == nil {
		return nil, fmt.Errorf("at least one image or extensionSource is required")
	}

	ctx := context.Background()

	// Create client with API key
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  req.GeminiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	isMultiImage := len(req.Images) > 1
	useKeyframesMode := req.UseKeyframes && len(req.Images) == 2

	// Choose model based on feature requirements (same logic as TS)
	model := "veo-3.1-fast-generate-preview"
	if len(req.Images) > 2 || req.ExtensionSource != nil {
		model = "veo-3.1-generate-preview"
	}

	var operation *genai.GenerateVideosOperation

	// VIDEO EXTENSION (Video-to-Video flow)
	if req.ExtensionSource != nil {
		config := &genai.GenerateVideosConfig{
			NumberOfVideos: 1,
			Resolution:     "720p",
			AspectRatio:    req.AspectRatio,
		}

		source := &genai.GenerateVideosSource{
			Prompt: req.Prompt,
			Video: &genai.Video{
				URI: req.ExtensionSource.URI,
			},
		}

		operation, err = client.Models.GenerateVideosFromSource(ctx, "veo-3.1-generate-preview", source, config)

	} else if useKeyframesMode {
		// START -> END KEYFRAME INTERPOLATION
		imageData, decodeErr := base64.StdEncoding.DecodeString(req.Images[0].Data)
		if decodeErr != nil {
			return nil, fmt.Errorf("failed to decode image: %v", decodeErr)
		}

		lastFrameData, decodeErr := base64.StdEncoding.DecodeString(req.Images[1].Data)
		if decodeErr != nil {
			return nil, fmt.Errorf("failed to decode last frame: %v", decodeErr)
		}

		image := &genai.Image{
			ImageBytes: imageData,
			MIMEType:   req.Images[0].MimeType,
		}

		config := &genai.GenerateVideosConfig{
			NumberOfVideos: 1,
			Resolution:     "720p",
			AspectRatio:    req.AspectRatio,
			LastFrame: &genai.Image{
				ImageBytes: lastFrameData,
				MIMEType:   req.Images[1].MimeType,
			},
		}

		operation, err = client.Models.GenerateVideos(ctx, "veo-3.1-fast-generate-preview", req.Prompt, image, config)

	} else if isMultiImage {
		// MULTI-IMAGE ASSET REFERENCE (Max 3)
		var referenceImages []*genai.VideoGenerationReferenceImage
		for i := 0; i < len(req.Images) && i < 3; i++ {
			imageData, decodeErr := base64.StdEncoding.DecodeString(req.Images[i].Data)
			if decodeErr != nil {
				return nil, fmt.Errorf("failed to decode reference image: %v", decodeErr)
			}

			referenceImages = append(referenceImages, &genai.VideoGenerationReferenceImage{
				Image: &genai.Image{
					ImageBytes: imageData,
					MIMEType:   req.Images[i].MimeType,
				},
				ReferenceType: "ASSET",
			})
		}

		config := &genai.GenerateVideosConfig{
			NumberOfVideos:  1,
			Resolution:      "720p",
			AspectRatio:     "16:9", // Requirement for multi-image
			ReferenceImages: referenceImages,
		}

		source := &genai.GenerateVideosSource{
			Prompt: req.Prompt,
		}

		operation, err = client.Models.GenerateVideosFromSource(ctx, "veo-3.1-generate-preview", source, config)

	} else {
		// SINGLE IMAGE START FRAME
		imageData, decodeErr := base64.StdEncoding.DecodeString(req.Images[0].Data)
		if decodeErr != nil {
			return nil, fmt.Errorf("failed to decode image: %v", decodeErr)
		}

		image := &genai.Image{
			ImageBytes: imageData,
			MIMEType:   req.Images[0].MimeType,
		}

		config := &genai.GenerateVideosConfig{
			NumberOfVideos: 1,
			Resolution:     "720p",
			AspectRatio:    req.AspectRatio,
		}

		operation, err = client.Models.GenerateVideos(ctx, model, req.Prompt, image, config)
	}

	if err != nil {
		log.Printf("[VEO Start] Generation error: %v", err)
		if strings.Contains(err.Error(), "Requested entity was not found") {
			return nil, fmt.Errorf("KEY_RESET_REQUIRED")
		}
		// Check for rate limit at generation start
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") || strings.Contains(err.Error(), "rate") || strings.Contains(err.Error(), "quota") {
			log.Printf("[VEO Start] Rate limit detected: %v", err)
			return nil, fmt.Errorf("Rate limit exceeded. Please wait a few minutes and try again. Details: %v", err)
		}
		return nil, fmt.Errorf("API error: %v", err)
	}

	log.Printf("[VEO Start] Generation started successfully. Operation: %s", operation.Name)

	// Extract operation name (format: "operations/{operationId}")
	return &VeoGenerationResponse{
		OperationID: operation.Name,
		Message:     "Video generation started successfully",
	}, nil
}

// pollVeoOperation - Direct translation of polling logic from veoService.ts
func pollVeoOperation(operationID, apiKey string) (*VeoPollResponse, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("no API key provided")
	}

	log.Printf("[VEO Poll] Starting poll for operation: %s", operationID)

	ctx := context.Background()

	// Create client with API key
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Printf("[VEO Poll] Failed to create client: %v", err)
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	// Poll operation status
	operation, err := client.Operations.GetVideosOperation(ctx, &genai.GenerateVideosOperation{
		Name: operationID,
	}, nil)

	if err != nil {
		log.Printf("[VEO Poll] GetVideosOperation error: %v", err)
		if strings.Contains(err.Error(), "Requested entity was not found") {
			return &VeoPollResponse{
				Done:   true,
				Status: "failed",
				Error:  "KEY_RESET_REQUIRED",
			}, nil
		}
		// Check for rate limit errors
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") || strings.Contains(err.Error(), "rate") {
			log.Printf("[VEO Poll] Rate limit detected in error: %v", err)
			return &VeoPollResponse{
				Done:   true,
				Status: "failed",
				Error:  fmt.Sprintf("Rate limit exceeded. Please wait a few minutes and try again. Details: %v", err),
			}, nil
		}
		return nil, fmt.Errorf("poll failed: %v", err)
	}

	// Log full operation state for debugging
	log.Printf("[VEO Poll] Operation Done: %v, Has Error: %v, Has Response: %v",
		operation.Done, operation.Error != nil, operation.Response != nil)

	// Check for errors
	if operation.Error != nil {
		// Log the full error object for debugging
		errorJSON, _ := json.MarshalIndent(operation.Error, "", "  ")
		log.Printf("[VEO Poll] Operation error (full): %s", string(errorJSON))

		errorMsg := "Generation failed"
		// Try to extract more detailed error info
		if msg, ok := operation.Error["message"].(string); ok {
			errorMsg = msg
			log.Printf("[VEO Poll] Error message: %s", msg)
		}
		if code, ok := operation.Error["code"].(float64); ok {
			log.Printf("[VEO Poll] Error code: %v", code)
			// HTTP 429 = rate limit
			if int(code) == 429 {
				errorMsg = "Rate limit exceeded. Please wait a few minutes and try again."
			}
		}
		if status, ok := operation.Error["status"].(string); ok {
			log.Printf("[VEO Poll] Error status: %s", status)
			if status == "RESOURCE_EXHAUSTED" {
				errorMsg = "Rate limit exceeded. Please wait a few minutes and try again."
			}
		}
		// Check for details array
		if details, ok := operation.Error["details"].([]interface{}); ok {
			detailsJSON, _ := json.MarshalIndent(details, "", "  ")
			log.Printf("[VEO Poll] Error details: %s", string(detailsJSON))
		}

		return &VeoPollResponse{
			Done:   true,
			Status: "failed",
			Error:  errorMsg,
		}, nil
	}

	// Check if operation is done
	if !operation.Done {
		log.Printf("[VEO Poll] Operation still processing...")
		return &VeoPollResponse{
			Done:   false,
			Status: "processing",
		}, nil
	}

	// Operation completed - extract video info
	// Debug: log full response
	responseJSON, _ := json.MarshalIndent(operation.Response, "", "  ")
	log.Printf("[VEO Poll] Full response: %s", string(responseJSON))

	if operation.Response == nil || len(operation.Response.GeneratedVideos) == 0 {
		log.Printf("[VEO Poll] Operation completed but no video returned. Response nil: %v, GeneratedVideos count: %d",
			operation.Response == nil,
			func() int {
				if operation.Response != nil {
					return len(operation.Response.GeneratedVideos)
				}
				return 0
			}())

		// Check for RAI (Responsible AI) filter reasons
		errorMsg := "Generation completed but no video was returned. This may be due to safety filters or content policy."
		if operation.Response != nil && len(operation.Response.RAIMediaFilteredReasons) > 0 {
			errorMsg = operation.Response.RAIMediaFilteredReasons[0]
			log.Printf("[VEO Poll] RAI filter reason: %s", errorMsg)
		}

		return &VeoPollResponse{
			Done:   true,
			Status: "failed",
			Error:  errorMsg,
		}, nil
	}

	videoAsset := operation.Response.GeneratedVideos[0].Video
	log.Printf("[VEO Poll] Success! Video URI: %s", videoAsset.URI)

	return &VeoPollResponse{
		Done:     true,
		Status:   "completed",
		VideoURL: videoAsset.URI,
		VideoAsset: &VeoVideoAsset{
			URI:         videoAsset.URI,
			AspectRatio: "16:9", // Default, SDK might not provide this field
		},
	}, nil
}
