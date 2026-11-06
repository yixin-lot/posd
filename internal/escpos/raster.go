package escpos

import "image"

// rasterBytes 把图片转成 ESC/POS GS v 0 光栅数据。
// 规则：按亮度阈值二值化，超宽按 maxWidth 等比缩放（最近邻，够用且无依赖）。
func rasterBytes(img image.Image, maxWidth int) []byte {
	if img == nil {
		return nil
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil
	}
	if maxWidth > 0 && w > maxWidth {
		ratio := float64(maxWidth) / float64(w)
		w = maxWidth
		h = int(float64(h)*ratio + 0.5)
		if h <= 0 {
			h = 1
		}
	}
	bytesPerRow := (w + 7) / 8
	out := make([]byte, 0, 8+bytesPerRow*h)
	// GS v 0 m xL xH yL yH
	out = append(out, gs, 0x76, 0x30, 0x00,
		byte(bytesPerRow%256), byte(bytesPerRow/256),
		byte(h%256), byte(h/256))

	row := make([]byte, bytesPerRow)
	for y := 0; y < h; y++ {
		for i := range row {
			row[i] = 0
		}
		for x := 0; x < w; x++ {
			// 最近邻取样
			sx := b.Min.X + x*b.Dx()/w
			sy := b.Min.Y + y*b.Dy()/h
			r, g, bl, _ := img.At(sx, sy).RGBA()
			// 亮度（0-65535），阈值取中位偏亮，保证细线不糊
			lum := (299*int(r) + 587*int(g) + 114*int(bl)) / 1000
			if lum < 32768 {
				row[x/8] |= 1 << (7 - uint(x%8))
			}
		}
		out = append(out, row...)
	}
	return out
}
