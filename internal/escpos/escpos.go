// Package escpos 生成热敏小票字节（ESC/POS + GBK）。
//
// 与 Java 版 EscPosRenderer 逐字节对齐：同一个订单必须产出同一串字节，
// 迁移期用 testdata 里的 golden 文件做比对（见 escpos_test.go）。
package escpos

import (
	"image"
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
)

const (
	esc = 0x1b
	gs  = 0x1d
	lf  = 0x0a
)

var gbk = simplifiedchinese.GBK.NewEncoder()

// Shop 是小票抬头信息。
type Shop struct {
	Name   string
	Phone  string
	Footer string
}

// Item 是一行商品。
type Item struct {
	Name   string
	Price  string // 元，两位小数
	Qty    int
	Amount string // 元，两位小数
}

// Pay 是一笔支付（名称 + 金额字符串）。
type Pay struct {
	Name   string
	Amount string
}

// Order 是渲染一张小票需要的字段。
type Order struct {
	OrderNo   string
	CreatedAt string
	Table     string
	Items     []Item
	Total     string
	Discount  string
	Cash      string
	Change    string
	RoundOff  string
	MemberDiscount string
	Points    string
	Notice    string
	// Footer 是小票页脚（设置 receipt_footer，可多行）
	Footer    string
	// QRText 是小票二维码内容（设置 receipt_qr_text，空则不打印）
	QRText    string
	// Logo 是小票 logo（设置里上传，nil 则不打印）
	Logo      image.Image
	Payments  []Pay
}

// Shift 是交接单需要的字段。
type Shift struct {
	ShiftNo     string
	StaffName   string
	OpenedAt    string
	ClosedAt    string
	OpeningCash string
	Payments    []Payment
	OrderCount  int
	Total       string
	RefundTotal string
	VoidTotal   string
	DineInCount int
	TakeawayCnt int
	MemberDiscountTotal string
	PointsEarned int64
	PointsUsed   int64
	CrossRefundCount int
	CrossRefundTotal string
	Expected    string
	Actual      string
	Difference  string
	Note        string
}

// Payment 是收款方式小计。
type Payment struct {
	Method string
	Amount string
}

// Receipt 是 ESC/POS 构造器：只做换行、对齐、加粗、分栏、切纸。
type Receipt struct {
	width int
	buf   bytes.Buffer
}

// NewReceipt 创建指定列宽的构造器（58mm 一般为 32 列）。
func NewReceipt(width int) *Receipt {
	return &Receipt{width: width}
}

func (r *Receipt) raw(b ...byte) *Receipt {
	r.buf.Write(b)
	return r
}

// Init 初始化打印机。
func (r *Receipt) Init() *Receipt { return r.raw(esc, 0x40) }

// Align 对齐：0 左、1 中、2 右。
func (r *Receipt) Align(mode byte) *Receipt { return r.raw(esc, 0x61, mode) }

// Bold 打开/关闭加粗。
func (r *Receipt) Bold(on bool) *Receipt {
	if on {
		return r.raw(esc, 0x45, 1)
	}
	return r.raw(esc, 0x45, 0)
}

// Text 写入一段文本（GBK 编码）。
func (r *Receipt) Text(s string) *Receipt {
	if b, err := gbk.Bytes([]byte(s)); err == nil {
		r.buf.Write(b)
	} else {
		r.buf.WriteString(s)
	}
	return r
}

// Line 写一行。
func (r *Receipt) Line(s string) *Receipt { return r.Text(s).raw(lf) }

// Rule 写一条分隔线。
func (r *Receipt) Rule() *Receipt { return r.Line(strings.Repeat("-", r.width)) }

// Columns 写左右分栏：左列按显示宽度截断，右列右对齐。
func (r *Receipt) Columns(left, right string) *Receipt {
	rw := displayWidth(right)
	l := truncate(left, max(0, r.width-rw-1))
	pad := max(1, r.width-displayWidth(l)-rw)
	return r.Line(l + strings.Repeat(" ", pad) + right)
}

// Feed 走纸 n 行。
func (r *Receipt) Feed(n int) *Receipt {
	for i := 0; i < n; i++ {
		r.raw(lf)
	}
	return r
}

// Cut 切纸。
func (r *Receipt) Cut() *Receipt { return r.raw(gs, 0x56, 0x42, 0x00) }

// Bytes 返回最终字节。
func (r *Receipt) Bytes() []byte { return r.buf.Bytes() }

// RenderOrder 渲染一张销售小票（32 列）。
func RenderOrder(shop Shop, o Order) []byte {
	r := NewReceipt(32)
	r.Init()
	r.Align(1).Bold(true).Line(shop.Name).Bold(false)
	if strings.TrimSpace(shop.Phone) != "" {
		r.Line(shop.Phone)
	}
	r.Align(0).Rule()
	r.Columns("单号", o.OrderNo)
	if strings.TrimSpace(o.Table) != "" {
		r.Columns("桌台", o.Table)
	}
	r.Columns("时间", o.CreatedAt)
	r.Rule()

	for _, it := range o.Items {
		r.Line(fmt.Sprintf("%s x%d", it.Name, it.Qty))
		r.Columns("  "+it.Price+" / 件", it.Amount)
	}

	r.Rule()
	r.Columns("合计(元)", o.Total)
	if o.Discount != "" && o.Discount != "0.00" {
		r.Columns("优惠", "-"+o.Discount)
	}
	if o.MemberDiscount != "" && o.MemberDiscount != "0.00" {
		r.Columns("会员折扣", "-"+o.MemberDiscount)
	}
	if o.RoundOff != "" && o.RoundOff != "0.00" {
		r.Columns("抹零", "-"+o.RoundOff)
	}
	// 支付方式逐行打印：现金显示实收与找零，扫码只显示金额。
	for _, p := range o.Payments {
		r.Columns(p.Name, p.Amount)
	}
	if len(o.Payments) == 0 {
		r.Columns("现金", o.Cash)
	}
	if o.Points != "" && o.Points != "0" {
		r.Columns("本单积分", "+"+o.Points)
	}
	// 找零：无支付明细（纯现金单）时总要印；有明细时只有真收了现金才印，
	// 否则扫码/会员单会多出一行「找零 0.00」的噪音。
	if len(o.Payments) == 0 || (o.Cash != "" && o.Cash != "0.00") {
		r.Columns("找零", o.Change)
	}
	r.Rule()
	if o.Logo != nil {
		r.Align(1).Image(o.Logo, 256)
	}
	r.Align(1).Line(shop.Footer)
	// 自定义页脚：设置里配（可多行），例如"谢谢惠顾 / 电话 138xxxx"
	for _, line := range strings.Split(o.Footer, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			r.Line(s)
		}
	}
	if strings.TrimSpace(o.Notice) != "" {
		r.Rule()
		r.Align(1).Line(o.Notice)
	}
	if strings.TrimSpace(o.QRText) != "" {
		r.Align(1).QR(o.QRText, 6)
	}
	r.Feed(3)
	r.Cut()
	return r.Bytes()
}

// Barcode 打印 CODE39 条码（GS k 4）。只接受 CODE39 字符集，其它字符会丢。
// 用于价签/商品标签；宽度与高度由打印机默认值决定。
func (r *Receipt) Barcode(code string) *Receipt {
	if strings.TrimSpace(code) == "" {
		return r
	}
	data := strings.ToUpper(code)
	r.raw(gs, 0x68, 0x50) // 条码高度 80 dots
	r.raw(gs, 0x77, 0x02) // 条码宽度 2
	r.raw(gs, 0x48, 0x02) // 打印 HRI 文本在条码下方
	r.raw(gs, 0x6B, 0x04, byte(len(data)))
	r.raw([]byte(data)...)
	return r.raw(lf)
}

// QR 打印二维码（GS ( k，型号 2）。不引入二维码库：
// 由打印机固件编码，单文件体积不增加，老机器不支持的会打出乱码（需实测）。
func (r *Receipt) QR(text string, moduleSize byte) *Receipt {
	if strings.TrimSpace(text) == "" {
		return r
	}
	if moduleSize == 0 {
		moduleSize = 6
	}
	data := []byte(text)
	n := len(data) + 3
	// 选择型号 2
	r.raw(gs, 0x28, 0x6B, 0x04, 0x00, 0x31, 0x41, 0x32, 0x00)
	// 模块大小
	r.raw(gs, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x43, moduleSize)
	// 纠错等级 M
	r.raw(gs, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x45, 0x31)
	// 存入数据
	r.raw(gs, 0x28, 0x6B, byte(n%256), byte(n/256), 0x31, 0x50, 0x30)
	r.raw(data...)
	// 打印
	r.raw(gs, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x51, 0x30)
	return r.raw(lf)
}

// Image 打印光栅图（GS v 0）。会做黑白阈值处理，适合 logo。
func (r *Receipt) Image(img image.Image, maxWidth int) *Receipt {
	payload := rasterBytes(img, maxWidth)
	if len(payload) == 0 {
		return r
	}
	r.raw(payload...)
	return r.raw(lf)
}

// Label 是一张价签/商品标签需要的字段。
type Label struct {
	ShopName string
	Name     string
	Price    string
	Barcode  string
	Spec     string // 规格/单位，如 "500ml"
}

// RenderLabel 渲染价签（32 列）：店名小字、品名加粗、价格大字、条码。
func RenderLabel(l Label) []byte {
	r := NewReceipt(32)
	r.Init()
	if strings.TrimSpace(l.ShopName) != "" {
		r.Align(1).Line(l.ShopName)
	}
	r.Align(1).Bold(true).Line(l.Name).Bold(false)
	if strings.TrimSpace(l.Spec) != "" {
		r.Align(1).Line(l.Spec)
	}
	r.Align(1).Bold(true)
	r.Line("￥" + l.Price)
	r.Bold(false)
	if strings.TrimSpace(l.Barcode) != "" {
		r.Align(1).Barcode(l.Barcode)
	}
	r.Feed(2)
	r.Cut()
	return r.Bytes()
}

// RenderShift 渲染交接班单。
func RenderShift(s Shift) []byte {
	r := NewReceipt(32)
	r.Init()
	r.Align(1).Bold(true).Line("交接班单").Bold(false)
	r.Align(0).Rule()
	r.Columns("班次", s.ShiftNo)
	staff := s.StaffName
	if strings.TrimSpace(staff) == "" {
		staff = "-"
	}
	r.Columns("收银员", staff)
	r.Columns("开班", s.OpenedAt)
	if s.ClosedAt != "" {
		r.Columns("结班", s.ClosedAt)
	}
	r.Rule()
	r.Columns("期初现金", s.OpeningCash)
	for _, p := range s.Payments {
		r.Columns("收款-"+methodName(p.Method), p.Amount)
	}
	r.Rule()
	r.Columns("订单笔数", fmt.Sprintf("%d", s.OrderCount))
	if s.CrossRefundCount > 0 {
		r.Columns("跨班次退款", fmt.Sprintf("%d 笔 %s", s.CrossRefundCount, "-"+s.CrossRefundTotal))
	}
	if s.PointsEarned > 0 || s.PointsUsed > 0 {
		r.Columns("积分发放/核销", fmt.Sprintf("%d / %d", s.PointsEarned, s.PointsUsed))
	}
	if s.MemberDiscountTotal != "" && s.MemberDiscountTotal != "0.00" {
		r.Columns("会员折扣", "-"+s.MemberDiscountTotal)
	}
	if s.DineInCount > 0 || s.TakeawayCnt > 0 {
		r.Columns("堂食/外带", fmt.Sprintf("%d / %d", s.DineInCount, s.TakeawayCnt))
	}
	r.Columns("应收合计", s.Total)
	if s.RefundTotal != "" && s.RefundTotal != "0.00" {
		r.Columns("退款合计", "-"+s.RefundTotal)
	}
	if s.VoidTotal != "" && s.VoidTotal != "0.00" {
		r.Columns("作废合计", "-"+s.VoidTotal)
	}
	if s.Expected != "" {
		r.Columns("现金应收", s.Expected)
		r.Columns("现金实收", s.Actual)
		r.Columns("差异", s.Difference)
		if strings.TrimSpace(s.Note) != "" {
			r.Line("备注：" + s.Note)
		}
	}
	r.Rule()
	r.Align(1).Line("请双方签字确认")
	r.Feed(3)
	r.Cut()
	return r.Bytes()
}

func methodName(m string) string {
	switch m {
	case "cash":
		return "现金"
	case "offline_qr":
		return "线下码"
	case "credit":
		return "挂账"
	default:
		return m
	}
}

func displayWidth(s string) int {
	w := 0
	for _, c := range s {
		if c > 0xff {
			w += 2
		} else {
			w++
		}
	}
	return w
}

func truncate(s string, maxWidth int) string {
	var b strings.Builder
	w := 0
	for _, c := range s {
		cw := 1
		if c > 0xff {
			cw = 2
		}
		if w+cw > maxWidth {
			break
		}
		b.WriteRune(c)
		w += cw
	}
	return b.String()
}

// Void 是作废小票需要的字段（金额单位：元字符串）。
type Void struct {
	OrderNo   string
	CreatedAt string
	Amount    string
	Reason    string
	Operator  string
}

// RenderVoid 渲染一张作废小票：纸面留痕，防止"单子消失了没人知道"。
func RenderVoid(shop Shop, v Void) []byte {
	r := NewReceipt(32)
	r.Init()
	r.Align(1).Bold(true).Line("作废单").Bold(false)
	if strings.TrimSpace(shop.Name) != "" {
		r.Line(shop.Name)
	}
	r.Align(0).Rule()
	r.Columns("单号", v.OrderNo)
	r.Columns("时间", v.CreatedAt)
	op := v.Operator
	if strings.TrimSpace(op) == "" {
		op = "-"
	}
	r.Columns("操作员", op)
	reason := v.Reason
	if strings.TrimSpace(reason) == "" {
		reason = "-"
	}
	r.Columns("原因", reason)
	r.Rule()
	r.Columns("作废金额", v.Amount)
	r.Rule()
	r.Align(1).Line("此单不计销售")
	r.Feed(3)
	r.Cut()
	return r.Bytes()
}

// KitchenItem 是后厨单的一行。
type KitchenItem struct {
	Name string
	Qty  int
}

// RenderKitchen 渲染后厨单：不印价格，突出品名与数量（厨师只看这个）。
func RenderKitchen(orderNo, tableName, station, createdAt string, items []KitchenItem) []byte {
	r := NewReceipt(32)
	r.Init()
	r.Align(1).Bold(true).Line("== 后厨单 ==").Bold(false)
	if strings.TrimSpace(station) != "" {
		r.Line(station)
	}
	if strings.TrimSpace(tableName) != "" {
		r.Line(tableName)
	}
	r.Align(0).Rule()
	r.Columns("单号", orderNo)
	r.Columns("时间", createdAt)
	r.Rule()
	for _, it := range items {
		r.Bold(true).Line(fmt.Sprintf("%s  x%d", it.Name, it.Qty)).Bold(false)
	}
	r.Rule()
	r.Align(1).Line("请按单出品")
	r.Feed(3)
	r.Cut()
	return r.Bytes()
}
