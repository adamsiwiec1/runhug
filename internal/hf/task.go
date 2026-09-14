package hf

import "strings"

// InferPipelineTag picks a Hub pipeline_tag from an NL query.
// Default is "any" so relevance search is not forced onto text-generation LLMs.
func InferPipelineTag(query string) string {
	s := strings.ToLower(strings.TrimSpace(query))
	if s == "" {
		return "any"
	}

	switch {
	case containsAny(s,
		"text-to-video", "text to video", "video generation", "image-to-video", "img2vid",
		"generate video", "video model", "video models"):
		return "text-to-video"

	case containsAny(s,
		"text-to-image", "text to image", "image generation", "image model", "image models",
		"diffusion", "stable diffusion", "sdxl", "midjourney", "img2img", "image-to-image",
		"cartoon", "anime", "animation", "animated", "toon", "illustration",
		"uncensored image", "nsfw image") ||
		(containsAny(s, "image", "vision") && containsAny(s, "model", "models", "gen", "generate", "uncensored", "heretic", "flux", "sd")) ||
		containsAny(s, " flux", "flux ", "flux.", "sd1", "sd2", "sdxl"):
		return "text-to-image"

	case containsAny(s, "text-to-speech", "tts", "speech synthesis", "voice generation", "voice clone"):
		return "text-to-speech"

	case containsAny(s, "speech-to-text", "asr", "transcription", "speech recognition") ||
		(strings.Contains(s, "whisper") && !strings.Contains(s, "instruct")):
		return "automatic-speech-recognition"

	case containsAny(s, "text-to-audio", "music generation", "audio generation", "sound generation"):
		return "text-to-audio"

	case containsAny(s, "embedding", "embeddings", "sentence similarity", "feature extraction", "reranker"):
		return "feature-extraction"

	case containsAny(s, "fill-mask", "masked language"):
		return "fill-mask"

	case containsAny(s, "translation") || (strings.Contains(s, "translate") && !strings.Contains(s, "instruct")):
		return "translation"

	case containsAny(s, "summarization", "summarize", "tldr"):
		return "summarization"

	default:
		return "any"
	}
}

// ResolveTask turns a CLI --task value into a Hub pipeline_tag.
// "auto" (the default) and empty use InferPipelineTag.
func ResolveTask(flagTask, query string) string {
	t := strings.ToLower(strings.TrimSpace(flagTask))
	switch t {
	case "", "auto":
		return InferPipelineTag(query)
	default:
		return t
	}
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
