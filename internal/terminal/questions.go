package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Dyu-36/gotack/internal/engineapi"
)

func answerQuestions(ctx context.Context, api *engineapi.Client, ws, session string, payload json.RawMessage, lines <-chan inputLine, diagnostic io.Writer) error {
	var batch struct {
		ID        string `json:"id"`
		SessionID string `json:"session_id"`
		Questions []struct {
			ID          string `json:"id"`
			Type        string `json:"type"`
			Question    string `json:"question"`
			Description string `json:"description"`
			Choices     []struct {
				ID    string `json:"id"`
				Label string `json:"label"`
			} `json:"choices"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(payload, &batch); err != nil {
		return err
	}
	if batch.SessionID != session {
		return nil
	}
	responses := make([]map[string]any, 0, len(batch.Questions))
	for _, question := range batch.Questions {
		fmt.Fprintf(diagnostic, "\n%s\n%s\n", question.Question, question.Description)
		for i, choice := range question.Choices {
			fmt.Fprintf(diagnostic, "%d. %s\n", i+1, choice.Label)
		}
		fmt.Fprint(diagnostic, "Answer (choice numbers separated by commas, or text): ")
		answer, err := readLine(ctx, lines)
		if err != nil {
			return err
		}
		if strings.TrimSpace(answer) == "/cancel" {
			return fmt.Errorf("question cancelled")
		}
		response := map[string]any{"request_id": question.ID}
		if question.Type == "yes_no" {
			response["yes"] = strings.EqualFold(strings.TrimSpace(answer), "y") || strings.EqualFold(strings.TrimSpace(answer), "yes")
		} else if len(question.Choices) > 0 {
			var selected []string
			valid := true
			for _, number := range strings.Split(answer, ",") {
				i, err := strconv.Atoi(strings.TrimSpace(number))
				if err != nil || i < 1 || i > len(question.Choices) {
					valid = false
					break
				}
				selected = append(selected, question.Choices[i-1].ID)
			}
			if valid {
				response["selected_ids"] = selected
			} else {
				response["fill_in_text"] = answer
			}
		} else {
			response["fill_in_text"] = answer
		}
		responses = append(responses, response)
	}
	return api.AnswerQuestions(ctx, ws, map[string]any{"batch_request_id": batch.ID, "responses": responses})
}
