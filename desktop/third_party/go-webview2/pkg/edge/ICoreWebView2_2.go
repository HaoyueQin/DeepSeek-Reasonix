//go:build windows

package edge

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type iCoreWebView2_2Vtbl struct {
	QueryInterface ComProc
	AddRef         ComProc
	Release        ComProc
	// ICoreWebView2 methods
	GetSettings                            ComProc
	GetSource                              ComProc
	Navigate                               ComProc
	NavigateToString                       ComProc
	AddNavigationStarting                  ComProc
	RemoveNavigationStarting               ComProc
	AddContentLoading                      ComProc
	RemoveContentLoading                   ComProc
	AddSourceChanged                       ComProc
	RemoveSourceChanged                    ComProc
	AddHistoryChanged                      ComProc
	RemoveHistoryChanged                   ComProc
	AddNavigationCompleted                 ComProc
	RemoveNavigationCompleted              ComProc
	AddFrameNavigationStarting             ComProc
	RemoveFrameNavigationStarting          ComProc
	AddFrameNavigationCompleted            ComProc
	RemoveFrameNavigationCompleted         ComProc
	AddScriptDialogOpening                 ComProc
	RemoveScriptDialogOpening              ComProc
	AddPermissionRequested                 ComProc
	RemovePermissionRequested              ComProc
	AddProcessFailed                       ComProc
	RemoveProcessFailed                    ComProc
	AddScriptToExecuteOnDocumentCreated    ComProc
	RemoveScriptToExecuteOnDocumentCreated ComProc
	ExecuteScript                          ComProc
	CapturePreview                         ComProc
	Reload                                 ComProc
	PostWebMessageAsJSON                   ComProc
	PostWebMessageAsString                 ComProc
	AddWebMessageReceived                  ComProc
	RemoveWebMessageReceived               ComProc
	CallDevToolsProtocolMethod             ComProc
	GetBrowserProcessID                    ComProc
	GetCanGoBack                           ComProc
	GetCanGoForward                        ComProc
	GoBack                                 ComProc
	GoForward                              ComProc
	GetDevToolsProtocolEventReceiver       ComProc
	Stop                                   ComProc
	AddNewWindowRequested                  ComProc
	RemoveNewWindowRequested               ComProc
	AddDocumentTitleChanged                ComProc
	RemoveDocumentTitleChanged             ComProc
	GetDocumentTitle                       ComProc
	AddHostObjectToScript                  ComProc
	RemoveHostObjectFromScript             ComProc
	OpenDevToolsWindow                     ComProc
	AddContainsFullScreenElementChanged    ComProc
	RemoveContainsFullScreenElementChanged ComProc
	GetContainsFullScreenElement           ComProc
	AddWebResourceRequested                ComProc
	RemoveWebResourceRequested             ComProc
	AddWebResourceRequestedFilter          ComProc
	RemoveWebResourceRequestedFilter       ComProc
	AddWindowCloseRequested                ComProc
	RemoveWindowCloseRequested             ComProc
	// ICoreWebView2_2 methods
	AddWebResourceResponseReceived    ComProc
	RemoveWebResourceResponseReceived ComProc
	NavigateWithWebResourceRequest    ComProc
	AddDomContentLoaded               ComProc
	RemoveDomContentLoaded            ComProc
	GetCookieManager                  ComProc
	GetEnvironment                    ComProc
}

type ICoreWebView2_2 struct {
	vtbl *iCoreWebView2_2Vtbl
}

// AddRef increments the reference count of the ICoreWebView2_2 interface
func (i *ICoreWebView2_2) AddRef() uint32 {
	ret, _, _ := i.vtbl.AddRef.Call(uintptr(unsafe.Pointer(i)))

	return uint32(ret)
}

// Release decrements the reference count of the ICoreWebView2_2 interface
func (i *ICoreWebView2_2) Release() uint32 {
	ret, _, _ := i.vtbl.Release.Call(uintptr(unsafe.Pointer(i)))

	return uint32(ret)
}

func (i *ICoreWebView2_2) GetCookieManager() (*ICoreWebView2CookieManager, error) {
	var cookieManager *ICoreWebView2CookieManager
	hr, _, _ := i.vtbl.GetCookieManager.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&cookieManager)),
	)
	if windows.Handle(hr) != windows.S_OK {
		return nil, syscall.Errno(hr)
	}
	return cookieManager, nil
}

// CallDevToolsProtocolMethod invokes a Chrome DevTools Protocol method on the
// webview and delivers the JSON result (or error) through handler. The handler
// may be nil when the caller does not need the result. The CDP channel is the
// supported way to reach browser-native capabilities (Accessibility tree,
// Input dispatch, Page capture) that the WebView2 platform API does not expose.
func (i *ICoreWebView2_2) CallDevToolsProtocolMethod(methodName, parametersAsJson string, handler *iCoreWebView2CallDevToolsProtocolMethodCompletedHandler) error {
	u16method, err := windows.UTF16PtrFromString(methodName)
	if err != nil {
		return err
	}
	u16params, err := windows.UTF16PtrFromString(parametersAsJson)
	if err != nil {
		return err
	}
	var h uintptr
	if handler != nil {
		h = uintptr(unsafe.Pointer(handler))
	}
	hr, _, _ := i.vtbl.CallDevToolsProtocolMethod.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(u16method)),
		uintptr(unsafe.Pointer(u16params)),
		h,
	)
	if windows.Handle(hr) != windows.S_OK {
		return windows.Errno(hr)
	}
	return nil
}

// Reload reloads the current page.
func (i *ICoreWebView2_2) Reload() error {
	hr, _, _ := i.vtbl.Reload.Call(uintptr(unsafe.Pointer(i)))
	if windows.Handle(hr) != windows.S_OK {
		return windows.Errno(hr)
	}
	return nil
}

// GetDocumentTitle returns the current page title.
func (i *ICoreWebView2_2) GetDocumentTitle() (string, error) {
	var title *uint16
	hr, _, _ := i.vtbl.GetDocumentTitle.Call(uintptr(unsafe.Pointer(i)), uintptr(unsafe.Pointer(&title)))
	if windows.Handle(hr) != windows.S_OK {
		return "", windows.Errno(hr)
	}
	if title == nil {
		return "", nil
	}
	return windows.UTF16PtrToString(title), nil
}
