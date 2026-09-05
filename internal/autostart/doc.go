// Package autostart manages the per-user "launch at login" registry entry for
// the desktop host. The entry always starts the host with the main window
// hidden so a boot launch rests in the system tray until the user opens it,
// which keeps the Zalo bot and scheduler alive from sign-in.
package autostart
