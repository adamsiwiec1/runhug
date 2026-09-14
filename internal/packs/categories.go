package packs

// Category describes one downloadable index pack.
type Category struct {
	ID       string // stable id used in filenames and metadata keys
	Title    string
	Pipeline string // Hub pipeline_tag (empty = any / filter-only)
	Filter   string // Hub filter= tag (e.g. gguf)
	// ExtraPipelines are fetched and merged when Pipeline alone is insufficient
	// (e.g. video = text-to-video + image-to-video).
	ExtraPipelines []string
}

// DefaultCategories is the v1 practical set. Packs pull as many Hub models
// as pass quality filters unless RUNHUG_INDEX_LIMIT / --limit caps rows.
func DefaultCategories() []Category {
	return []Category{
		{
			ID:       "text-generation",
			Title:    "Text Generation (LLMs)",
			Pipeline: "text-generation",
		},
		{
			ID:       "text-to-image",
			Title:    "Text to Image",
			Pipeline: "text-to-image",
		},
		{
			ID:             "video",
			Title:          "Video (text/image → video)",
			Pipeline:       "text-to-video",
			ExtraPipelines: []string{"image-to-video"},
		},
		{
			ID:             "audio",
			Title:          "Audio (TTS / ASR)",
			Pipeline:       "text-to-speech",
			ExtraPipelines: []string{"automatic-speech-recognition"},
		},
		{
			ID:     "gguf",
			Title:  "GGUF (local weights)",
			Filter: "gguf",
		},
	}
}

// LookupCategory returns a category by id (case-sensitive), or false.
func LookupCategory(id string) (Category, bool) {
	for _, c := range DefaultCategories() {
		if c.ID == id {
			return c, true
		}
	}
	return Category{}, false
}

// CategoryIDs returns stable ids in default order.
func CategoryIDs() []string {
	cats := DefaultCategories()
	out := make([]string, len(cats))
	for i, c := range cats {
		out[i] = c.ID
	}
	return out
}
