package crushapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const sessionPath = "/v1/workspaces/{id}/sessions/{sid}"

func (c *Client) GetSession(ctx context.Context, wsID, sessionID string) (Session, error) {
	if wsID == "" || sessionID == "" {
		return Session{}, errors.New("crushapi: workspace id and session id are required")
	}
	var sess Session
	path := expandPath(sessionPath, "id", wsID, "sid", sessionID)
	if err := c.doJSON(ctx, "GET", path, nil, &sess); err != nil {
		return Session{}, err
	}
	return sess, nil
}

func (c *Client) SaveSession(ctx context.Context, wsID string, sess Session) (Session, error) {
	if wsID == "" || sess.ID == "" {
		return Session{}, errors.New("crushapi: workspace id and session id are required")
	}
	body, err := json.Marshal(sess)
	if err != nil {
		return Session{}, fmt.Errorf("crushapi: encode session: %w", err)
	}
	var saved Session
	path := expandPath(sessionPath, "id", wsID, "sid", sess.ID)
	if err := c.doJSON(ctx, "PUT", path, bytes.NewReader(body), &saved); err != nil {
		return Session{}, err
	}
	return saved, nil
}

func (c *Client) DeleteSession(ctx context.Context, wsID, sessionID string) error {
	if wsID == "" || sessionID == "" {
		return errors.New("crushapi: workspace id and session id are required")
	}
	return c.doJSON(ctx, "DELETE", expandPath(sessionPath, "id", wsID, "sid", sessionID), nil, nil)
}
