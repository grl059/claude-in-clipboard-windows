//go:build windows

package main

// Tray icons are drawn in code (no image assets): each icon is a few geometric shapes in a
// 16x16 design space, supersampled into a 32-bit BGRA bitmap and turned into an HICON.

import (
	"math"
	"strconv"
	"unsafe"
)

type shape func(x, y float64) bool

func distSeg(px, py, x0, y0, x1, y1 float64) float64 {
	dx, dy := x1-x0, y1-y0
	l2 := dx*dx + dy*dy
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, ((px-x0)*dx+(py-y0)*dy)/l2))
	}
	return math.Hypot(px-(x0+t*dx), py-(y0+t*dy))
}

func seg(x0, y0, x1, y1, w float64) shape {
	return func(x, y float64) bool { return distSeg(x, y, x0, y0, x1, y1) <= w/2 }
}

func disc(cx, cy, r float64) shape {
	return func(x, y float64) bool { return math.Hypot(x-cx, y-cy) <= r }
}

func ring(cx, cy, r, w float64) shape {
	return func(x, y float64) bool { return math.Abs(math.Hypot(x-cx, y-cy)-r) <= w/2 }
}

// poly is a filled polygon (even-odd rule); pts is x0,y0,x1,y1,...
func poly(pts ...float64) shape {
	return func(x, y float64) bool {
		in := false
		n := len(pts) / 2
		for i, j := 0, n-1; i < n; j, i = i, i+1 {
			xi, yi, xj, yj := pts[2*i], pts[2*i+1], pts[2*j], pts[2*j+1]
			if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
				in = !in
			}
		}
		return in
	}
}

// path is a stroked open polyline (closed when closed is true).
func path(w float64, closed bool, pts ...float64) []shape {
	var out []shape
	n := len(pts) / 2
	for i := 0; i+1 < n; i++ {
		out = append(out, seg(pts[2*i], pts[2*i+1], pts[2*i+2], pts[2*i+3], w))
	}
	if closed && n > 2 {
		out = append(out, seg(pts[2*n-2], pts[2*n-1], pts[0], pts[1], w))
	}
	return out
}

func cat(groups ...[]shape) []shape {
	var out []shape
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

func star(cx, cy, r, waist float64) shape {
	return poly(cx, cy-r, cx+waist, cy-waist, cx+r, cy, cx+waist, cy+waist,
		cx, cy+r, cx-waist, cy+waist, cx-r, cy, cx-waist, cy-waist)
}

// iconShapes names mirror the SF Symbols the macOS app uses.
func iconShapes(name string) []shape {
	switch name {
	case "keyboard": // prompt open, typing
		keys := []shape{}
		for _, y := range []float64{6.9, 9.1} {
			for _, x := range []float64{4.2, 6.8, 9.4, 12} {
				keys = append(keys, disc(x, y, 0.75))
			}
		}
		return cat(path(1.3, true, 1.6, 3.8, 14.4, 3.8, 14.4, 12.2, 1.6, 12.2), keys, []shape{seg(5, 11, 11, 11, 1.1)})
	case "hourglass": // Claude is working
		return cat(
			[]shape{seg(3.5, 2, 12.5, 2, 1.4), seg(3.5, 14, 12.5, 14, 1.4)},
			path(1.2, false, 4.6, 2, 8, 8, 4.6, 14),
			path(1.2, false, 11.4, 2, 8, 8, 11.4, 14),
			[]shape{poly(5.8, 13.2, 10.2, 13.2, 8, 10.4)})
	case "checkmark.circle": // copied
		return cat([]shape{ring(8, 8, 6.3, 1.4)}, path(1.5, false, 5, 8.2, 7.2, 10.4, 11, 5.8))
	case "trash": // clipboard cleared
		return cat(
			[]shape{seg(2.8, 4.2, 13.2, 4.2, 1.4), seg(6.2, 2.4, 9.8, 2.4, 1.2), seg(6.2, 2.4, 6.2, 4.2, 1.2), seg(9.8, 2.4, 9.8, 4.2, 1.2)},
			path(1.3, false, 4.2, 5.2, 4.9, 13.6, 11.1, 13.6, 11.8, 5.2),
			[]shape{seg(6.8, 6.6, 6.8, 12, 1), seg(9.2, 6.6, 9.2, 12, 1)})
	case "exclamationmark.triangle": // error
		return cat(path(1.4, true, 8, 2, 14.6, 13.6, 1.4, 13.6),
			[]shape{seg(8, 6, 8, 9.4, 1.5), disc(8, 11.6, 0.95)})
	default: // "sparkles": ready
		return []shape{star(7, 8.6, 6.2, 1.5), star(12.6, 3.4, 2.7, 0.65)}
	}
}

// renderIcon draws the named icon at n x n pixels in the given colour as top-down BGRA (straight alpha).
func renderIcon(name string, n int, r, g, b byte) []byte {
	shapes := iconShapes(name)
	const ss = 4
	unit := 16.0 / float64(n)
	pix := make([]byte, n*n*4)
	for py := 0; py < n; py++ {
		for px := 0; px < n; px++ {
			hits := 0
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/ss) * unit
					y := (float64(py) + (float64(sy)+0.5)/ss) * unit
					for _, s := range shapes {
						if s(x, y) {
							hits++
							break
						}
					}
				}
			}
			i := (py*n + px) * 4
			pix[i], pix[i+1], pix[i+2] = b, g, r
			pix[i+3] = byte(hits * 255 / (ss * ss))
		}
	}
	return pix
}

// makeIcon builds an HICON from BGRA pixels.
func makeIcon(pix []byte, n int) uintptr {
	hdr := bitmapInfoHeader{biSize: uint32(unsafe.Sizeof(bitmapInfoHeader{})), biWidth: int32(n), biHeight: -int32(n),
		biPlanes: 1, biBitCount: 32}
	var bits uintptr
	color, _, _ := pCreateDIBSection.Call(0, ptr(unsafe.Pointer(&hdr)), 0 /* DIB_RGB_COLORS */, ptr(unsafe.Pointer(&bits)), 0, 0)
	if color == 0 || bits == 0 {
		return 0
	}
	copy(unsafe.Slice((*byte)(fromPtr(bits)), len(pix)), pix)
	maskBits := make([]byte, (n+15)/16*2*n) // 1bpp, rows padded to 16 bits; all zero (the alpha channel does the work)
	mask, _, _ := pCreateBitmap.Call(uintptr(n), uintptr(n), 1, 1, ptr(unsafe.Pointer(&maskBits[0])))
	ii := iconInfo{fIcon: 1, hbmMask: mask, hbmColor: color}
	h, _, _ := pCreateIconIndirect.Call(ptr(unsafe.Pointer(&ii)))
	pDeleteObject.Call(color)
	pDeleteObject.Call(mask)
	return h
}

// taskbarIsLight reports whether the taskbar uses the light theme (so the glyph must be dark).
func taskbarIsLight() bool {
	v, ok := regDword(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, "SystemUsesLightTheme")
	return ok && v == 1
}

var iconCache = map[string]uintptr{}

// trayIcon returns a cached HICON for the name at the current taskbar size and theme.
func trayIcon(name string) uintptr {
	n, _, _ := pGetSystemMetrics.Call(smCxSmIcon)
	if n < 16 {
		n = 16
	}
	light := taskbarIsLight()
	key := name + "/" + strconv.FormatBool(light) + "/" + strconv.Itoa(int(n))
	if h, ok := iconCache[key]; ok {
		return h
	}
	var c byte = 255
	if light {
		c = 20
	}
	h := makeIcon(renderIcon(name, int(n), c, c, c), int(n))
	iconCache[key] = h
	return h
}
