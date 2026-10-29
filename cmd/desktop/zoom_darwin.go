//go:build desktop

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

static WKWebView *mesaFindWebView(NSView *view) {
	if ([view isKindOfClass:[WKWebView class]]) {
		return (WKWebView *)view;
	}
	for (NSView *sub in [view subviews]) {
		WKWebView *found = mesaFindWebView(sub);
		if (found != nil) {
			return found;
		}
	}
	return nil;
}

// pageZoom is what Safari's View → Zoom In sets: the page lays out again at
// the new size, so nothing in it needs to know.
static void mesaSetPageZoom(double level) {
	dispatch_async(dispatch_get_main_queue(), ^{
		for (NSWindow *window in [NSApp windows]) {
			WKWebView *webview = mesaFindWebView([window contentView]);
			if (webview == nil) {
				continue;
			}
			if (@available(macOS 11.0, *)) {
				webview.pageZoom = level;
			}
		}
	});
}
*/
import "C"

// nativeZoom says the View menu carries Zoom In / Zoom Out / Actual Size.
const nativeZoom = true

func setZoom(level float64) { C.mesaSetPageZoom(C.double(level)) }
