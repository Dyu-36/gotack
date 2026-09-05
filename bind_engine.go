package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/Dyu-36/gotack/internal/changes"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/enginelink"
	"github.com/Dyu-36/gotack/internal/permission"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/uievents"
	"github.com/Dyu-36/gotack/internal/workspace"
)

var _ enginelink.EventConsumer = (*uievents.Forwarder)(nil)

type EngineInfo struct {
	Status   string `json:"status"`
	Running  bool   `json:"running"`
	Endpoint string `json:"endpoint"`
	Version  string `json:"version"`
	Owned    bool   `json:"owned"`
	Error    string `json:"error,omitempty"`
}

func (a *App) engineInfo() EngineInfo {
	info := EngineInfo{
		Status: string(enginelink.StatusStopped),
	}
	if a.link != nil {
		status := a.link.Status()
		info.Status = string(status)
		info.Running = status == enginelink.StatusRunning
		info.Endpoint = a.link.Endpoint().Address
		info.Version = a.link.Version()
		info.Error = a.link.LastError()
	}
	if a.sup != nil {
		info.Owned = a.sup.Owned()
	}
	return info
}

func (a *App) EngineStatus() EngineInfo {
	return a.engineInfo()
}

func (a *App) StartEngine() EngineInfo {
	a.tryConnect()
	return a.EngineStatus()
}

func (a *App) StopEngine() EngineInfo {
	a.stopTransport()
	if a.sup != nil {
		_ = a.sup.Stop()
	}
	return a.EngineStatus()
}

func (a *App) ReconnectEngine() error {
	a.stopTransport()
	if !a.tryConnect() {
		return errors.New("engine connect already in progress")
	}
	return nil
}

func (a *App) tryConnect() bool {
	scope, started := a.link.BeginConnect(context.Background())
	if !started {
		return false
	}
	a.emit(uievents.EngineStatus, a.engineInfo())
	go a.connect(scope)
	return true
}

func (a *App) connect(scope context.Context) {
	err := a.link.Connect(scope, func(ctx context.Context, api *crushapi.Client, ep crushapi.Endpoint, version string) error {
		callbacks := uievents.Callbacks{
			RunDone:              a.runDone,
			AssistantIteration:   a.assistantIteration,
			LearningToolExecuted: a.learningToolExecuted,
			RunTelemetry: func(telemetry *crushapi.RunTelemetry) {
				if a.runMetrics != nil {
					a.runMetrics.Append(telemetry)
				}
			},
		}
		if relay := a.permsFromConn(); relay != nil {
			callbacks.PermissionPending = relay.Pending
		}
		fwd := uievents.NewForwarder(a.log, a.emit, callbacks)
		ws := workspace.NewService(api)
		sess := session.NewService(api, ws)
		diffs := changes.NewService(api, ws)

		if !a.commitAttach(ctx, api, fwd, ws, sess, diffs, ep, version) {
			return enginelink.ErrAttachSuperseded
		}

		svc := &bridgeServices{api: api, ws: ws, sess: sess, diffs: diffs}

		a.migrateChatGPTProviderCredential(svc)
		workspaceWarning := ""
		if _, err := a.activateAssistantWorkspace(svc); err != nil {

			a.log.Warn("could not attach the default workspace", "err", err)
			workspaceWarning = fmt.Sprintf("initialize assistant workspace: %v", err)
		}

		a.log.Info("engine connected", "endpoint", ep.Address, "version", version, "owned", a.sup.Owned())
		a.link.MarkRunning()
		if desc, ok := svc.ws.Current(); ok {
			a.rebindWorkspaceRuntime(desc.WorkspaceID)
		}
		status := a.engineInfo()
		if status.Error == "" {
			status.Error = workspaceWarning
		}
		a.emit(uievents.EngineStatus, status)

		a.setSchedulerReady(true)
		a.reapplySavedWorkspaceSettings()
		if a.zalo != nil && a.zalo.Status().Configured {
			a.zalo.Start()
		}
		return nil
	})
	switch {
	case err == nil:
	case errors.Is(err, enginelink.ErrAttachSuperseded):

	default:
		a.failConnect(err.Error())
	}
}

func (a *App) commitAttach(
	ctx context.Context,
	api *crushapi.Client,
	fwd *uievents.Forwarder,
	ws *workspace.Service,
	sess *session.Service,
	diffs *changes.Service,
	ep crushapi.Endpoint,
	version string,
) bool {
	if a.getConn() == nil {
		return false
	}
	var stillCurrent bool
	a.swapConn(func(c *conn) *conn {
		if ctx.Err() != nil {
			return c
		}
		c.api = api
		c.fwd = fwd
		c.ws = ws
		c.sess = sess
		c.diffs = diffs
		stillCurrent = true
		return c
	})
	if !stillCurrent {
		return false
	}
	return a.link.CommitAttach(ctx, ep, version)
}

func (a *App) permsFromConn() *permission.Relay {
	if c := a.getConn(); c != nil {
		return c.perms
	}
	return nil
}

func (a *App) failConnect(reason string) {
	if a.log != nil {
		a.log.Error("engine connect failed", "reason", reason)
	}
	a.link.Fail(reason)
	a.emit(uievents.EngineStatus, a.engineInfo())
}

func (a *App) transportLost(scope context.Context, reason string) {
	if !a.link.TransportLost(scope, reason) {
		return
	}
	a.setSchedulerReady(false)
	if a.log != nil {
		a.log.Warn("engine transport lost", "reason", reason)
	}
	a.emit(uievents.EngineStatus, a.engineInfo())
}

func (a *App) attachStream(scope context.Context, workspaceID string) error {
	c := a.getConn()
	if c == nil || c.api == nil || c.fwd == nil {
		return enginelink.ErrTransportNotWired
	}
	return enginelink.AttachStream(scope, c.api, c.fwd, workspaceID, a.transportLost)
}

func (a *App) startStream(scope context.Context, workspaceID string) {
	if err := a.attachStream(scope, workspaceID); err != nil {
		a.transportLost(scope, err.Error())
	}
}

func (a *App) replaceWorkspaceStream(workspaceID string) error {
	if workspaceID == "" {
		return enginelink.ErrWorkspaceIDRequired
	}
	if a.getConn() == nil {
		return enginelink.ErrNoConnection
	}

	scope := a.link.ReplaceStreamScope(a.ctx)
	if err := a.attachStream(scope, workspaceID); err != nil {
		a.link.CancelScope()
		return err
	}
	return nil
}

func (a *App) stopTransport() {
	if a.getConn() == nil {
		return
	}
	var fwd *uievents.Forwarder
	a.swapConn(func(c *conn) *conn {
		fwd = c.fwd
		c.api = nil
		c.fwd = nil
		c.ws = nil
		c.sess = nil
		c.diffs = nil
		return c
	})
	a.link.Disconnect()
	a.setSchedulerReady(false)
	if fwd != nil {
		fwd.Stop()
	}
	if a.zalo != nil {
		a.zalo.Stop()
	}
	a.emit(uievents.EngineStatus, a.engineInfo())
}

type bridgeServices struct {
	api   *crushapi.Client
	ws    *workspace.Service
	sess  *session.Service
	diffs *changes.Service
}

func (a *App) services() (*bridgeServices, error) {
	c := a.getConn()
	if c == nil || c.api == nil || c.ws == nil || c.sess == nil || a.link.Status() != enginelink.StatusRunning {
		return nil, errors.New("engine is not running")
	}
	return &bridgeServices{api: c.api, ws: c.ws, sess: c.sess, diffs: c.diffs}, nil
}
