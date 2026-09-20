package main

func (a *App) startZaloIfEnabled() {
	if a.zalo == nil {
		return
	}
	if a.cfg != nil && a.cfg.Zalo.Enabled && a.zalo.Status().Configured {
		a.zalo.Start()
	} else {
		a.zalo.Stop()
	}
}
