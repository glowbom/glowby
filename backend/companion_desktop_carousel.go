package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const companionCarouselInstructionBytes = 4 << 10

type companionDesktopCarousel struct {
	Version int                            `json:"version"`
	Cards   []companionDesktopCarouselCard `json:"cards"`
}

type companionDesktopCarouselCard struct {
	ID                         string `json:"id"`
	Instructions               string `json:"instructions,omitempty"`
	AttachmentID               string `json:"attachmentId,omitempty"`
	PersonalizationReferenceID string `json:"personalizationReferenceId,omitempty"`
}

func normalizeCompanionDesktopCarousel(request *companionDesktopPrototypeRequest) error {
	carousel := request.Carousel
	if carousel == nil {
		return nil
	}
	if carousel.Version != 1 || len(carousel.Cards) < 1 || len(carousel.Cards) > maxCarouselCards {
		return errors.New("Use between one and ten carousel images.")
	}
	if request.AttachmentID != "" || request.Images != nil && request.Images.PersonalizationReferenceID != "" {
		return errors.New("Choose the image and personalization photo on each carousel item.")
	}
	seen := map[string]bool{}
	total := 0
	for index := range carousel.Cards {
		card := &carousel.Cards[index]
		if !carouselCardID.MatchString(card.ID) || seen[card.ID] {
			return errors.New("Each carousel image needs a unique valid ID.")
		}
		seen[card.ID] = true
		if len(card.Instructions) > companionCarouselInstructionBytes || strings.ContainsRune(card.Instructions, '\x00') {
			return errors.New("Describe each carousel image in up to 4 KB of text.")
		}
		total += len(card.Instructions)
		if total > companionDesktopPrototypePromptBytes {
			return errors.New("Keep the carousel descriptions within 32 KB of text.")
		}
		for _, value := range []*string{&card.AttachmentID, &card.PersonalizationReferenceID} {
			if *value != "" {
				*value = strings.ToLower(normalizedStudioUUID(*value))
				if *value == "" {
					return errors.New("Attach the carousel image again.")
				}
			}
		}
		if card.AttachmentID == "" && strings.TrimSpace(card.Instructions) == "" {
			return errors.New("Add a drawing, photo, or description to each carousel item.")
		}
		if card.PersonalizationReferenceID != "" && (card.AttachmentID == "" || request.Images == nil) {
			return errors.New("Choose image generation and attach a canvas before personalizing a carousel image.")
		}
	}
	return nil
}

func (request companionDesktopPrototypeRequest) hasCanvasImages() bool {
	if request.AttachmentID != "" {
		return true
	}
	if request.Carousel != nil {
		for _, card := range request.Carousel.Cards {
			if card.AttachmentID != "" {
				return true
			}
		}
	}
	return false
}

// Resolve one owned upload at a time. The four-image Build limit remains intact;
// a prototype carousel has a separate ten-item input limit and four output limit.
func (s *companionSession) prepareDesktopCarousel(ctx context.Context, request companionDesktopPrototypeRequest) (*preparedChatCarousel, error) {
	if request.Carousel == nil {
		return nil, nil
	}
	project, err := s.localChatProject(ctx)
	if err != nil {
		return nil, err
	}
	parts := map[string]map[string]any{}
	references := map[string]map[string]any{}
	manifest := &chatCarousel{Version: 1}
	load := func(id string) (map[string]any, error) {
		if part, ok := parts[id]; ok {
			return part, nil
		}
		images, err := s.chatImages(ctx, project, []string{id})
		if err != nil || len(images) != 1 {
			return nil, errCompanionAttachmentUnavailable
		}
		parts[id] = images[0]
		return images[0], nil
	}
	for index, card := range request.Carousel.Cards {
		if card.AttachmentID != "" {
			if _, err := load(card.AttachmentID); err != nil {
				return nil, err
			}
		}
		if card.PersonalizationReferenceID != "" {
			part, err := load(card.PersonalizationReferenceID)
			if err != nil {
				return nil, err
			}
			references[card.ID] = part
		}
		manifest.Cards = append(manifest.Cards, chatCarouselCard{
			ID: card.ID, Role: "sketch", Title: fmt.Sprintf("Image %d", index+1),
			Instructions: card.Instructions, AttachmentPath: card.AttachmentID,
			Personalization: card.PersonalizationReferenceID != "",
		})
	}
	source := ""
	if request.Images != nil {
		source = request.Images.SourceID
	}
	return prepareChatCarouselImages(manifest, source, ctx, func(id string) (map[string]any, error) {
		part, ok := parts[id]
		if !ok {
			return nil, errCompanionAttachmentUnavailable
		}
		return part, nil
	}, references)
}
