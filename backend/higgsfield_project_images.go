package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync"
)

// Preserve the default used by clients that predate project image settings.
const higgsfieldProjectImageModel = "gpt_image_2_5"
const higgsfieldImageSourceLabel = "Higgsfield"

type higgsfieldImageBudgetError struct{ message string }

func (e *higgsfieldImageBudgetError) Error() string { return e.message }

func validateHiggsfieldImageAllowance(credits *float64) error {
	if credits == nil || math.IsNaN(*credits) || math.IsInf(*credits, 0) || *credits < 0 {
		return &higgsfieldImageBudgetError{"Review and confirm the Higgsfield image credit limit before generating."}
	}
	return nil
}

type higgsfieldImageBudgetKey struct{}
type higgsfieldImageBudget struct {
	mu        sync.Mutex
	remaining *big.Rat
}

func withHiggsfieldImageAllowance(ctx context.Context, credits *float64) (context.Context, error) {
	if err := validateHiggsfieldImageAllowance(credits); err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, higgsfieldImageBudgetKey{}, &higgsfieldImageBudget{remaining: higgsfieldDecimalCredits(*credits)}), nil
}

func higgsfieldDecimalCredits(credits float64) *big.Rat {
	value, _ := new(big.Rat).SetString(strconv.FormatFloat(credits, 'f', -1, 64))
	return value
}

func (budget *higgsfieldImageBudget) reserve(credits float64) bool {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	price := higgsfieldDecimalCredits(credits)
	if price.Cmp(budget.remaining) > 0 {
		return false
	}
	// Decimal accounting keeps the last approved image within a four-image budget.
	budget.remaining.Sub(budget.remaining, price)
	return true
}

func higgsfieldProjectImageOptions(aspect string) studioImageOptions {
	if aspect == "" {
		aspect = "1:1"
	}
	return studioImageOptions{SourceID: higgsfieldSourceID, ModelID: higgsfieldProjectImageModel, AspectRatio: aspect, Resolution: "1k", Quality: "low"}
}

func normalizeHiggsfieldProjectImageOptions(options studioImageOptions, icon bool) (studioImageOptions, error) {
	return normalizeHiggsfieldProjectImageOptionsInCatalogue(options, icon, studioImageModels())
}

func normalizeHiggsfieldProjectImageOptionsInCatalogue(options studioImageOptions, icon bool, sources []studioImageSourceCapability) (studioImageOptions, error) {
	options.SourceID = strings.TrimSpace(options.SourceID)
	if options.SourceID != higgsfieldSourceID {
		return options, errors.New("Choose Higgsfield for these project image settings.")
	}
	if strings.TrimSpace(options.ModelID) == "" {
		options.ModelID = higgsfieldProjectImageModel
	}
	if icon {
		if strings.TrimSpace(options.AspectRatio) != "" && strings.TrimSpace(options.AspectRatio) != "1:1" {
			return options, errors.New("App icons require a square 1:1 image.")
		}
		options.AspectRatio = "1:1"
	} else if strings.TrimSpace(options.AspectRatio) == "" {
		for _, source := range sources {
			if source.ID != higgsfieldSourceID {
				continue
			}
			for _, model := range source.Models {
				if model.ID == strings.TrimSpace(options.ModelID) && studioImageContains(model.AspectRatios, "16:9") {
					options.AspectRatio = "16:9"
				}
			}
		}
	}
	return normalizeStudioImageOptionsInCatalogue(options, sources)
}

// A missing price is not a free image. A reported numeric zero is valid.
func parseHiggsfieldImageCredits(raw json.RawMessage) (float64, error) {
	var response struct {
		Cost struct {
			Credits *float64 `json:"credits"`
		} `json:"cost"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Cost.Credits == nil || math.IsNaN(*response.Cost.Credits) || math.IsInf(*response.Cost.Credits, 0) || *response.Cost.Credits < 0 {
		return 0, errors.New("Higgsfield did not return a valid price for this image.")
	}
	return *response.Cost.Credits, nil
}

func generateHiggsfieldProjectImage(ctx context.Context, options studioImageOptions, prompt, reference string) (string, string, error) {
	budget, _ := ctx.Value(higgsfieldImageBudgetKey{}).(*higgsfieldImageBudget)
	if budget == nil {
		return "", higgsfieldImageSourceLabel, validateHiggsfieldImageAllowance(nil)
	}
	options, err := normalizeStudioImageOptions(options)
	if err != nil {
		return "", higgsfieldImageSourceLabel, err
	}
	credits, err := higgsfieldImageEstimate(ctx, options)
	if err != nil {
		return "", higgsfieldImageSourceLabel, &higgsfieldImageBudgetError{"Could not recheck the Higgsfield image price. No image was requested."}
	}
	// Reserve before submission. An uncertain response must not refund or replay it.
	if !budget.reserve(credits) {
		return "", higgsfieldImageSourceLabel, &higgsfieldImageBudgetError{"The Higgsfield image price exceeds the confirmed credit limit. Review the price again; no image was requested."}
	}
	return generateStudioSelectedImageWithOptions(ctx, options, "", prompt, reference)
}
