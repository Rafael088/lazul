//go:build !linux

package main

// En Windows Wails ya concede los permisos de WebView2 (SetGlobalPermission).
func parcheWebview() {}
func parcheListo()   {}
