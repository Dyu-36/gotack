package engine

import (
	"context"
	"github.com/Dyu-36/gotack/internal/engineapi"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestConnectRejectsIncompatibleProtocolBeforeAttaching(t *testing.T) {
	sup := &fakeSupervisor{found: true}
	link := NewLink(sup)
	link.dial = func(_ engineapi.Endpoint) (*http.Client, error) {
		return &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"version":"old","protocol":0}`)), Header: make(http.Header), Request: req}, nil
		})}, nil
	}
	scope, _ := link.BeginConnect(context.Background())
	defer link.Disconnect()
	err := link.Connect(scope, func(context.Context, *engineapi.Client, engineapi.Endpoint, string) error {
		t.Fatal("incompatible engine attached")
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "protocol mismatch") {
		t.Fatalf("error = %v", err)
	}
}
