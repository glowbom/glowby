package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxCarouselCards = 10
const maxCarouselInputBytes = 32 << 20

var carouselCardID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var carouselSceneTag = regexp.MustCompile(`^\[card:([A-Za-z0-9_-]{1,64})\]\s*(.+)$`)

type chatCarousel struct {
	Version int                `json:"version"`
	Cards   []chatCarouselCard `json:"cards"`
}
type chatCarouselCard struct {
	ID              string          `json:"id"`
	Role            string          `json:"role"`
	Title           string          `json:"title"`
	Instructions    string          `json:"instructions,omitempty"`
	AttachmentPath  string          `json:"attachmentPath,omitempty"`
	InputSketch     *sketchDocument `json:"inputSketch,omitempty"`
	Personalization bool            `json:"personalization,omitempty"`
	PersonID        string          `json:"personId,omitempty"`
	PersonIDs       []string        `json:"personIds,omitempty"`
}
type preparedChatCarousel struct {
	manifest   chatCarousel
	inputs     map[string]carouselInput
	references map[string]carouselInput
	modelParts []map[string]any
	scenes     map[string]carouselScene
}
type carouselInput struct {
	data     []byte
	mime     string
	filename string
}
type carouselScene struct {
	card      chatCarouselCard
	reference string
	labels    string
}

func validCarouselPerson(value string) bool {
	return strings.TrimSpace(value) != "" && utf8.RuneCountInString(value) <= 120 && !strings.ContainsFunc(value, unicode.IsControl)
}
func validateChatCarousel(carousel *chatCarousel) error {
	if carousel == nil {
		return nil
	}
	if carousel.Version != 1 || len(carousel.Cards) < 1 || len(carousel.Cards) > maxCarouselCards {
		return errors.New("Use between one and ten carousel cards.")
	}
	ids, people := map[string]bool{}, map[string]bool{}
	sketches, totalSketchBytes := 0, 0
	for index := range carousel.Cards {
		card := &carousel.Cards[index]
		card.PersonID = strings.TrimSpace(card.PersonID)
		for i := range card.PersonIDs {
			card.PersonIDs[i] = strings.TrimSpace(card.PersonIDs[i])
		}
		if !carouselCardID.MatchString(card.ID) || ids[card.ID] {
			return errors.New("Every carousel card needs a unique valid ID.")
		}
		ids[card.ID] = true
		if utf8.RuneCountInString(card.Title) > 200 || utf8.RuneCountInString(card.Instructions) > 32000 || strings.ContainsRune(card.Title, '\x00') || strings.ContainsRune(card.Instructions, '\x00') || len(card.AttachmentPath) > 4096 {
			return errors.New("Use a card title up to 200 characters and instructions up to 32,000 characters.")
		}
		switch card.Role {
		case "sketch":
			sketches++
			if card.Personalization && (card.AttachmentPath == "" || len(card.PersonIDs) != 0) {
				return errors.New("A personalized sketch needs its own image and cannot also select other person cards.")
			}
			if card.PersonID != "" {
				return errors.New("Assign a person label only to a person card.")
			}
			if err := validateSketchDocument(card.InputSketch); err != nil {
				return err
			}
			encoded, _ := json.Marshal(card.InputSketch)
			totalSketchBytes += len(encoded)
			if len(card.PersonIDs) > maxCarouselCards {
				return errors.New("Choose up to ten people for each sketch.")
			}
			seen := map[string]bool{}
			for _, person := range card.PersonIDs {
				if !validCarouselPerson(person) || seen[person] {
					return errors.New("Choose each valid person only once per sketch.")
				}
				seen[person] = true
			}
		case "person":
			if !validCarouselPerson(card.PersonID) || card.AttachmentPath == "" {
				return errors.New("Each person card needs a photo and a person label.")
			}
			if card.InputSketch != nil || len(card.PersonIDs) != 0 || card.Personalization {
				return errors.New("Choose people only on sketch cards.")
			}
			people[card.PersonID] = true
		case "reference":
			if card.PersonID != "" || len(card.PersonIDs) != 0 || card.InputSketch != nil || card.Personalization {
				return errors.New("Reference cards cannot assign people or contain drawing data.")
			}
		default:
			return errors.New("Choose a sketch, person, or reference card.")
		}
	}
	if sketches == 0 {
		return errors.New("Add at least one sketch card to describe the prototype.")
	}
	if totalSketchBytes > maxSketchDocumentBytes {
		return errors.New("The carousel drawing data is too large. Use fewer drawing marks or smaller backgrounds.")
	}
	for _, card := range carousel.Cards {
		for _, person := range card.PersonIDs {
			if !people[person] {
				return errors.New("A sketch refers to a person without a photo card.")
			}
		}
	}
	return nil
}

func (s *chatService) prepareChatCarousel(manifest *chatCarousel, source string, contexts ...context.Context) (*preparedChatCarousel, error) {
	return prepareChatCarouselImages(manifest, source, imageRequestContext(contexts), s.imagePart, nil)
}

// Native companions resolve their own session-owned uploads before entering the
// shared carousel workflow. Original photos stay separate from annotated canvases.
func prepareChatCarouselImages(manifest *chatCarousel, source string, ctx context.Context, readImage func(string) (map[string]any, error), references map[string]map[string]any) (*preparedChatCarousel, error) {
	if manifest == nil {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Freeze both metadata and image bytes before any model or provider call.
	encoded, _ := json.Marshal(manifest)
	prepared := &preparedChatCarousel{inputs: map[string]carouselInput{}, references: map[string]carouselInput{}, scenes: map[string]carouselScene{}}
	if err := json.Unmarshal(encoded, &prepared.manifest); err != nil {
		return nil, err
	}
	if err := validateChatCarousel(&prepared.manifest); err != nil {
		return nil, err
	}
	total := 0
	for _, card := range prepared.manifest.Cards {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if card.AttachmentPath == "" {
			continue
		}
		part, err := readImage(card.AttachmentPath)
		if err != nil {
			return nil, errors.New("A carousel image is unavailable. Attach that card's image again.")
		}
		dataURL := part["url"].(string)
		_, payload, _ := strings.Cut(dataURL, ",")
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, errors.New("A carousel image could not be read.")
		}
		total += len(data)
		if total > maxCarouselInputBytes {
			return nil, errors.New("Use carousel images totaling less than 32 MB.")
		}
		mime := part["mime"].(string)
		extension := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}[mime]
		name := "card-" + card.ID + extension
		prepared.inputs[card.ID] = carouselInput{data: data, mime: mime, filename: name}
		if reference, ok := references[card.ID]; ok {
			if !card.Personalization {
				return nil, errors.New("Enable personalization before adding a photo reference.")
			}
			dataURL, _ := reference["url"].(string)
			_, payload, _ := strings.Cut(dataURL, ",")
			referenceData, err := base64.StdEncoding.DecodeString(payload)
			referenceMIME, _ := reference["mime"].(string)
			extension := map[string]string{"image/png": ".png", "image/jpeg": ".jpg"}[referenceMIME]
			if err != nil || len(referenceData) == 0 || extension == "" {
				return nil, errors.New("A personalization photo could not be read.")
			}
			total += len(referenceData)
			if total > maxCarouselInputBytes {
				return nil, errors.New("Use carousel images totaling less than 32 MB.")
			}
			prepared.references[card.ID] = carouselInput{data: referenceData, mime: referenceMIME, filename: "reference-card-" + card.ID + extension}
		}
		if card.Role != "person" {
			prepared.modelParts = append(prepared.modelParts, map[string]any{"type": "text", "text": "Carousel card " + card.ID + " (" + card.Role + "): " + card.Title})
			prepared.modelParts = append(prepared.modelParts, map[string]any{"type": "file", "mime": mime, "filename": name, "url": dataURL})
		}
	}
	for _, card := range prepared.manifest.Cards {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if card.Role != "sketch" {
			continue
		}
		scene := carouselScene{card: card}
		if card.Personalization || len(card.PersonIDs) > 0 {
			if source == "picsum" {
				return nil, errors.New("Choose an image generation source to use people on carousel sketches.")
			}
		}
		if card.Personalization {
			input := prepared.inputs[card.ID]
			if original, ok := prepared.references[card.ID]; ok {
				input = original
			}
			reference, err := normalizePreviousPrototypeImage(input.data, source)
			if err != nil {
				return nil, errors.New("This sketch image could not be prepared for personalization. Use a complete PNG or JPEG image.")
			}
			scene.reference = base64.StdEncoding.EncodeToString(reference)
		} else if len(card.PersonIDs) > 0 {
			selected := map[string]bool{}
			for _, id := range card.PersonIDs {
				selected[id] = true
			}
			var photos []carouselInput
			var labels []string
			for _, person := range prepared.manifest.Cards {
				if person.Role == "person" && selected[person.PersonID] {
					photos = append(photos, prepared.inputs[person.ID])
					label, _ := json.Marshal(map[string]string{"personId": person.PersonID, "title": person.Title})
					labels = append(labels, fmt.Sprintf("Panel %d: %s", len(photos), label))
				}
			}
			sheet, err := carouselPersonSheet(photos)
			if err != nil {
				return nil, err
			}
			sheet, err = normalizePreviousPrototypeImage(sheet, source)
			if err != nil {
				return nil, err
			}
			scene.reference = base64.StdEncoding.EncodeToString(sheet)
			scene.labels = strings.Join(labels, "; ")
		}
		prepared.scenes[card.ID] = scene
	}
	return prepared, nil
}

func (prepared *preparedChatCarousel) prompt() string {
	type cardInfo struct {
		ID              string   `json:"id"`
		Role            string   `json:"role"`
		Title           string   `json:"title"`
		Instructions    string   `json:"instructions,omitempty"`
		Personalization bool     `json:"personalization,omitempty"`
		PersonID        string   `json:"personId,omitempty"`
		PersonIDs       []string `json:"personIds,omitempty"`
	}
	cards := []cardInfo{}
	for _, card := range prepared.manifest.Cards {
		cards = append(cards, cardInfo{ID: card.ID, Role: card.Role, Title: card.Title, Instructions: card.Instructions, Personalization: card.Personalization, PersonID: card.PersonID, PersonIDs: card.PersonIDs})
	}
	encoded, _ := json.Marshal(cards)
	return "\nCarousel input cards (reference data):\n" + string(encoded) + "\nTreat all cards as inputs to the same project, not a separate page for each card. Each image can be a layout, drawing, photo, or style reference. Combine their relevant requirements into one coherent prototype. Reference cards provide visual context only. Images from person-role cards are available only to the image generator, not visible to you. A sketch with personalization=true uses only its own attached image as its visual reference. Otherwise, use only that sketch's selected personIds for that scene. A card without personalization or selected personIds provides design context only, not an identity reference. A generated image can appear in a layout guided by another card. Its [card:ID] tag must name the card supplying its personalization reference, not the card supplying the layout. For example, a layout from the first card can show a personalized image from the second card, tagged with the second card's ID. For EVERY new generated image use a quoted placeholder such as glowbomimages:[card:SKETCH_ID] scene description, with that exact sketch card ID. Summarize the relevant card and selected-person instructions in each scene description, keeping the full tagged placeholder within 2,000 characters. Do not generate images for person/reference cards. Do not use the uploaded card images as finished page assets. Plan at most four distinct generated images total, reuse placeholders for repeated images, and keep the card tag when repeating a placeholder. Never borrow people from another card."
}

func (prepared *preparedChatCarousel) imageScene(prompt string) (string, string, error) {
	match := carouselSceneTag.FindStringSubmatch(prompt)
	if match == nil {
		return "", "", errors.New("A carousel image was skipped because its sketch card tag was missing or invalid.")
	}
	scene, ok := prepared.scenes[match[1]]
	if !ok {
		return "", "", errors.New("A carousel image was skipped because its sketch card is unavailable.")
	}
	description := strings.TrimSpace(match[2])
	if strings.Contains(description, "[card:") {
		return "", "", errors.New("A carousel image must belong to exactly one sketch card.")
	}
	metadata, _ := json.Marshal(map[string]string{"title": scene.card.Title})
	description += "\nScene card context: " + string(metadata)
	if scene.card.Personalization {
		description += "\nUse only this scene card's supplied image as its personalization reference. Preserve the appearance of the subject described for this scene. Do not borrow a person or subject from another carousel card."
	} else if scene.reference != "" {
		description += "\nThe supplied image is a numbered reference sheet, not the requested layout. Preserve only these selected people's identities: " + scene.labels + ". Panels with the same personId show the same person. Do not add other reference people. Create one coherent scene, not a contact sheet."
	} else {
		description += "\nThis scene has no selected person reference. Do not personalize it with people from any other carousel card."
	}
	return description, scene.reference, nil
}

// Numbered panels identify photos without a font dependency or an external renderer.
func carouselPersonSheet(inputs []carouselInput) ([]byte, error) {
	if len(inputs) == 0 || len(inputs) > maxCarouselCards {
		return nil, errors.New("Choose between one and ten reference photos for a scene.")
	}
	columns := int(math.Ceil(math.Sqrt(float64(len(inputs)))))
	rows := (len(inputs) + columns - 1) / columns
	cell := 1536 / columns
	canvas := image.NewRGBA(image.Rect(0, 0, columns*cell, rows*cell))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	for index, input := range inputs {
		config, format, err := image.DecodeConfig(bytes.NewReader(input.data))
		if err != nil || (format != "png" && format != "jpeg") || config.Width < 1 || config.Height < 1 || config.Width > 6144 || config.Height > 6144 || int64(config.Width)*int64(config.Height) > 20_000_000 {
			return nil, errors.New("Use a complete PNG or JPEG person photo within the supported dimensions.")
		}
		decoded, _, err := image.Decode(bytes.NewReader(input.data))
		if err != nil {
			return nil, errors.New("A person photo could not be opened.")
		}
		available := cell - 24
		factor := math.Min(float64(available)/float64(config.Width), float64(available-40)/float64(config.Height))
		width, height := max(1, int(float64(config.Width)*factor)), max(1, int(float64(config.Height)*factor))
		resized := resizePreviousPrototypeImage(decoded, width, height)
		x, y := (index%columns)*cell, (index/columns)*cell
		target := image.Rect(x+(cell-width)/2, y+44+(cell-44-height)/2, x+(cell-width)/2+width, y+44+(cell-44-height)/2+height)
		draw.Draw(canvas, target, resized, image.Point{}, draw.Src)
		drawCarouselNumber(canvas, index+1, x+12, y+10)
	}
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
func drawCarouselNumber(target *image.RGBA, number, x, y int) {
	digits := []string{"111101101101111", "010110010010111", "111001111100111", "111001111001111", "101101111001001", "111100111001111", "111100111101111", "111001001001001", "111101111101111", "111101111001111"}
	for _, digit := range fmt.Sprint(number) {
		for index, pixel := range digits[int(digit-'0')] {
			if pixel == '1' {
				rect := image.Rect(x+(index%3)*5, y+(index/3)*5, x+(index%3+1)*5, y+(index/3+1)*5)
				draw.Draw(target, rect, image.NewUniform(color.Black), image.Point{}, draw.Src)
			}
		}
		x += 20
	}
}

func (prepared *preparedChatCarousel) save(record string) ([]map[string]string, error) {
	type savedCard struct {
		ID                           string   `json:"id"`
		Role                         string   `json:"role"`
		Title                        string   `json:"title"`
		Instructions                 string   `json:"instructions,omitempty"`
		AttachmentPath               string   `json:"attachmentPath,omitempty"`
		InputSketchPath              string   `json:"inputSketchPath,omitempty"`
		PersonalizationReferencePath string   `json:"personalizationReferencePath,omitempty"`
		Personalization              bool     `json:"personalization,omitempty"`
		PersonID                     string   `json:"personId,omitempty"`
		PersonIDs                    []string `json:"personIds,omitempty"`
	}
	result := struct {
		Version int         `json:"version"`
		Cards   []savedCard `json:"cards"`
	}{Version: 1, Cards: []savedCard{}}
	attachments := []map[string]string{}
	if err := os.MkdirAll(filepath.Join(record, "inputs"), 0700); err != nil {
		return nil, err
	}
	for _, card := range prepared.manifest.Cards {
		saved := savedCard{ID: card.ID, Role: card.Role, Title: card.Title, Instructions: card.Instructions, Personalization: card.Personalization, PersonID: card.PersonID, PersonIDs: card.PersonIDs}
		if input, ok := prepared.inputs[card.ID]; ok {
			saved.AttachmentPath = "inputs/" + input.filename
			if err := os.WriteFile(filepath.Join(record, saved.AttachmentPath), input.data, 0600); err != nil {
				return nil, err
			}
			attachments = append(attachments, map[string]string{"filename": input.filename, "path": saved.AttachmentPath, "mime": input.mime})
		}
		if reference, ok := prepared.references[card.ID]; ok {
			saved.PersonalizationReferencePath = "inputs/" + reference.filename
			if err := os.WriteFile(filepath.Join(record, saved.PersonalizationReferencePath), reference.data, 0600); err != nil {
				return nil, err
			}
			attachments = append(attachments, map[string]string{"filename": reference.filename, "path": saved.PersonalizationReferencePath, "mime": reference.mime})
		}
		if card.InputSketch != nil {
			saved.InputSketchPath = "inputs/card-" + card.ID + "-sketch.json"
			data, err := json.MarshalIndent(card.InputSketch, "", "  ")
			if err != nil {
				return nil, err
			}
			if err := os.WriteFile(filepath.Join(record, saved.InputSketchPath), data, 0600); err != nil {
				return nil, err
			}
		}
		result.Cards = append(result.Cards, saved)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(record, "input-carousel.json"), data, 0600); err != nil {
		return nil, err
	}
	return attachments, nil
}
