// Package bridge is a minimal local Anthropic Messages → OpenAI Chat Completions
// translator for Claude Code against Runpod vLLM (or any OpenAI-compatible upstream).
//
// Supported well enough for Claude Code chat + tool_use (tools / tool_choice /
// tool_result round-trips) with streaming SSE.
//
// Gaps / limitations (document for operators):
//   - Anthropic top_k, thinking / redacted_thinking, and server-side tools are dropped.
//   - Image blocks are forwarded as OpenAI image_url parts when possible; vision
//     depends on the upstream model.
//   - count_tokens is a rough stub (chars/4), not a real tokenizer.
//   - Streaming usage is estimated from chunk lengths when the upstream omits it.
//   - Prefill (trailing assistant message) is passed through as an assistant turn;
//     not all OpenAI backends honor it.
//   - Runpod worker-vllm returns HTTP 500 when tools are present unless the
//     endpoint sets ENABLE_AUTO_TOOL_CHOICE and TOOL_CALL_PARSER (see internal/family).
//     On upstream 5xx with tools, or a successful but empty stream with tools, the
//     bridge retries once with tools/tool_choice stripped. It never emits a
//     successful Anthropic SSE that is only message_start/stop with no content
//     blocks (Claude Code's blank "Sautéed for 2s").
//   - When REASONING_PARSER is enabled upstream, vLLM may stream into
//     delta.reasoning / reasoning_content with empty content; those are mapped to
//     Anthropic text_delta so the client still sees tokens.
package bridge
