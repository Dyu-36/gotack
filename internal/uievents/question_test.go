package uievents

import (
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

func TestForwarderQuestionRequestPreservesSelectableChoices(t *testing.T) {
	var c collector
	f := NewForwarder(slog.Default(), c.emit, Callbacks{})

	request := crushapi.QuestionRequest{
		ID:         "batch-1",
		SessionID:  "session-1",
		ToolCallID: "tool-1",
		Questions: []crushapi.QuestionItem{
			{
				ID:          "format",
				Type:        "single_choice",
				Question:    "Bạn muốn định dạng nào?",
				Description: "Chọn một định dạng.",
				Choices: []crushapi.QuestionChoice{
					{ID: "xlsx", Label: "Excel"},
					{ID: "docx", Label: "Word"},
				},
			},
			{
				ID:          "days",
				Type:        "multi_choice",
				Question:    "Chọn ngày học.",
				Description: "Có thể chọn nhiều ngày.",
				Choices: []crushapi.QuestionChoice{
					{ID: "mon", Label: "Thứ Hai"},
					{ID: "tue", Label: "Thứ Ba"},
				},
			},
		},
	}
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	f.handle(crushapi.StreamEvent{Kind: "question_batch_request", Event: "created", Payload: payload})

	events := c.of(QuestionRequest)
	if len(events) != 1 {
		t.Fatalf("question events = %d, want 1", len(events))
	}
	got, ok := events[0].data.(crushapi.QuestionRequest)
	if !ok {
		t.Fatalf("question payload type = %T", events[0].data)
	}
	if got.ID != request.ID || got.SessionID != request.SessionID || got.ToolCallID != request.ToolCallID {
		t.Fatalf("question envelope = %+v", got)
	}
	if len(got.Questions) != 2 {
		t.Fatalf("questions = %d, want 2", len(got.Questions))
	}
	if got.Questions[0].Type != "single_choice" || len(got.Questions[0].Choices) != 2 {
		t.Fatalf("single-choice question lost choices: %+v", got.Questions[0])
	}
	if got.Questions[1].Type != "multi_choice" || len(got.Questions[1].Choices) != 2 {
		t.Fatalf("multi-choice question lost choices: %+v", got.Questions[1])
	}
}
