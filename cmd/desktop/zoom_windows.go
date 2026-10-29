//go:build desktop

package main

// WebView2 zooms by itself (Ctrl+= / Ctrl+- / Ctrl+0 and Ctrl+wheel), so the
// View menu leaves those keys to it.
const nativeZoom = false

func setZoom(float64) {}
