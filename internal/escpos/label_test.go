package escpos

import (
	"bytes"
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestRenderLabelContainsNamePriceBarcode(t *testing.T) {
	out := decode(t, RenderLabel(Label{
		ShopName: "测试店", Name: "珍珠奶茶", Price: "13.00", Barcode: "6901234567890",
	}))
	for _, want := range []string{"测试店", "珍珠奶茶", "￥13.00"} {
		if !strings.Contains(out, want) {
			t.Fatalf("价签缺少 %q", want)
		}
	}
	raw := string(RenderLabel(Label{Name: "x", Price: "1.00", Barcode: "123456"}))
	if !strings.Contains(raw, "123456") {
		t.Fatal("条码数据没写进指令流")
	}
}

func TestQRCommandEmbedded(t *testing.T) {
	r := NewReceipt(32)
	r.QR("https://example.com/x", 6)
	b := r.Bytes()
	// GS ( k 存数据：1D 28 6B pL pH 31 50 30
	idx := bytes.Index(b, []byte{0x1d, 0x28, 0x6b})
	if idx < 0 {
		t.Fatal("没有 GS ( k 指令")
	}
	if !bytes.Contains(b, []byte("https://example.com/x")) {
		t.Fatal("二维码内容没写进指令流")
	}
}

func TestRenderOrderPrintsLogo(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.Gray{Y: 0}) // 全黑，必定出点
		}
	}
	b := RenderOrder(Shop{Name: "店"}, Order{OrderNo: "x", Logo: img})
	// GS v 0 光栅
	if !bytes.Contains(b, []byte{0x1d, 0x76, 0x30, 0x00}) {
		t.Fatal("logo 没有按光栅指令输出")
	}
}
