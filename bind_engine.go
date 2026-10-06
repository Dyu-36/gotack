package main

import "errors"

type EngineInfo struct {
	Status   string `json:"status"`
	Running  bool   `json:"running"`
	Endpoint string `json:"endpoint"`
	Version  string `json:"version"`
	Owned    bool   `json:"owned"`
	Error    string `json:"error,omitempty"`
}

func (a *App) engineInfo() EngineInfo   { return EngineInfo(a.host.Info()) }
func (a *App) EngineStatus() EngineInfo { return a.engineInfo() }
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
