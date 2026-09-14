package bridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// openaiUsage is the usage object carried on some OpenAI stream chunks.
type openaiUsage struct {
	CompletionTokens int `json:"completion_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
}

// StreamState tracks Anthropic SSE emission while consuming OpenAI chunks.
type StreamState struct {
	MsgID          string
	Model          string
	TextStarted    bool
	TextIndex      int
	Tools          map[int]*toolStream // openai tool_calls index → state
	NextBlockIndex int
	StopReason     string
	OutputChars    int
	Closed         bool
}

type toolStream struct {
	BlockIndex int
	ID         string
	Name       string
	Started    bool
}

func NewStreamState(msgID, model string) *StreamState {
	if msgID == "" {
		msgID = "msg_" + randomID(8)
	}
	return &StreamState{
		MsgID: msgID,
		Model: model,
		Tools: map[int]*toolStream{},
	}
}

func (s *StreamState) writeEvent(w io.Writer, name string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, b)
	return err
}

func (s *StreamState) EmitMessageStart(w io.Writer) error {
	return s.writeEvent(w, "message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            s.MsgID,
			"type":          "message",
			"role":          "assistant",
			"model":         s.Model,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         map[string]int{"input_tokens": 0, "output_tokens": 0},
		},
	})
}

func (s *StreamState) ensureTextStart(w io.Writer) error {
	if s.TextStarted {
		return nil
	}
	s.TextStarted = true
	s.TextIndex = s.NextBlockIndex
	s.NextBlockIndex++
	return s.writeEvent(w, "content_block_start", map[string]any{
		"type":  "content_block_start",
		"index": s.TextIndex,
		"content_block": map[string]any{
			"type": "text",
			"text": "",
		},
	})
}

func (s *StreamState) HandleOpenAIChunk(w io.Writer, data string) error {
	if data == "[DONE]" {
		return s.Finish(w, nil)
	}
	var chunk struct {
		ID      string `json:"id"`
		Choices []struct {
			Delta struct {
				Content   *string `json:"content"`
				ToolCalls []struct {
					Index    int    `json:"index"`
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
			FinishReason *string `json:"finish_reason"`
		} `json:"choices"`
		Usage *openaiUsage `json:"usage"`
	}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil // skip malformed
	}
	if chunk.ID != "" && strings.HasPrefix(chunk.ID, "chatcmpl-") {
		s.MsgID = "msg_" + strings.TrimPrefix(chunk.ID, "chatcmpl-")
	}
	if len(chunk.Choices) == 0 {
		if chunk.Usage != nil {
			return s.Finish(w, chunk.Usage)
		}
		return nil
	}
	ch := chunk.Choices[0]
	if ch.Delta.Content != nil && *ch.Delta.Content != "" {
		if err := s.ensureTextStart(w); err != nil {
			return err
		}
		s.OutputChars += len(*ch.Delta.Content)
		if err := s.writeEvent(w, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": s.TextIndex,
			"delta": map[string]any{"type": "text_delta", "text": *ch.Delta.Content},
		}); err != nil {
			return err
		}
	}
	for _, tc := range ch.Delta.ToolCalls {
		ts := s.Tools[tc.Index]
		if ts == nil {
			ts = &toolStream{BlockIndex: -1}
			s.Tools[tc.Index] = ts
		}
		if tc.ID != "" {
			ts.ID = tc.ID
		}
		if tc.Function.Name != "" {
			ts.Name = tc.Function.Name
		}
		if !ts.Started && ts.Name != "" {
			if ts.ID == "" {
				ts.ID = "toolu_" + randomID(10)
			}
			ts.BlockIndex = s.NextBlockIndex
			s.NextBlockIndex++
			ts.Started = true
			if err := s.writeEvent(w, "content_block_start", map[string]any{
				"type":  "content_block_start",
				"index": ts.BlockIndex,
				"content_block": map[string]any{
					"type":  "tool_use",
					"id":    ts.ID,
					"name":  ts.Name,
					"input": map[string]any{},
				},
			}); err != nil {
				return err
			}
		}
		if tc.Function.Arguments != "" && ts.Started {
			s.OutputChars += len(tc.Function.Arguments)
			if err := s.writeEvent(w, "content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": ts.BlockIndex,
				"delta": map[string]any{
					"type":         "input_json_delta",
					"partial_json": tc.Function.Arguments,
				},
			}); err != nil {
				return err
			}
		}
	}
	if ch.FinishReason != nil && *ch.FinishReason != "" {
		s.StopReason = mapFinishReason(*ch.FinishReason)
		return s.Finish(w, chunk.Usage)
	}
	return nil
}

func (s *StreamState) Finish(w io.Writer, usage *openaiUsage) error {
	if s.Closed {
		return nil
	}
	s.Closed = true
	if s.TextStarted {
		if err := s.writeEvent(w, "content_block_stop", map[string]any{
			"type": "content_block_stop", "index": s.TextIndex,
		}); err != nil {
			return err
		}
	}
	idxs := make([]int, 0, len(s.Tools))
	for i, ts := range s.Tools {
		if ts != nil && ts.Started {
			idxs = append(idxs, i)
		}
	}
	sort.Ints(idxs)
	for _, i := range idxs {
		ts := s.Tools[i]
		if err := s.writeEvent(w, "content_block_stop", map[string]any{
			"type": "content_block_stop", "index": ts.BlockIndex,
		}); err != nil {
			return err
		}
	}
	if s.StopReason == "" {
		if len(idxs) > 0 {
			s.StopReason = "tool_use"
		} else {
			s.StopReason = "end_turn"
		}
	}
	outTok := s.OutputChars / 4
	if usage != nil && usage.CompletionTokens > 0 {
		outTok = usage.CompletionTokens
	}
	if outTok < 1 && s.OutputChars > 0 {
		outTok = 1
	}
	if err := s.writeEvent(w, "message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": s.StopReason, "stop_sequence": nil},
		"usage": map[string]int{"output_tokens": outTok},
	}); err != nil {
		return err
	}
	return s.writeEvent(w, "message_stop", map[string]any{"type": "message_stop"})
}

// PipeOpenAISSE reads OpenAI SSE from r and writes Anthropic SSE to w.
func PipeOpenAISSE(r io.Reader, w io.Writer, model string) error {
	st := NewStreamState("", model)
	if err := st.EmitMessageStart(w); err != nil {
		return err
	}
	if f, ok := w.(interface{ Flush() }); ok {
		f.Flush()
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	var dataLines []string
	flushData := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = nil
		if err := st.HandleOpenAIChunk(w, data); err != nil {
			return err
		}
		if f, ok := w.(interface{ Flush() }); ok {
			f.Flush()
		}
		return nil
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if err := flushData(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			continue
		}
	}
	if err := flushData(); err != nil {
		return err
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return st.Finish(w, nil)
}
