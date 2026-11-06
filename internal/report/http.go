// Package report 的 HTTP 层。
package report

import (
	"encoding/json"
	"fmt"
	"strings"
	"net/http"
	"time"
)

// Handler 暴露报表接口（需店长，由调用方保证）。
type Handler struct{ svc *Service }

// NewHandler 创建处理器。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register 挂载 /pos/v1/reports/*；guard 为 nil 表示不额外鉴权（由调用方已保证）。
func (h *Handler) Register(mux *http.ServeMux, guard func(http.ResponseWriter, *http.Request) bool) {
	mux.HandleFunc("GET /pos/v1/reports/daily", func(w http.ResponseWriter, r *http.Request) {
		if guard != nil && !guard(w, r) {
			return
		}
		date := r.URL.Query().Get("date")
		if date == "" {
			date = todayBizDate(time.Now())
		}
		d, err := h.svc.Daily(date)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "统计失败"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"bizDate":     d.BizDate,
			"orderCount":  d.OrderCount,
			"grossSales":  yuan(d.GrossSales),
			"refundTotal": yuan(d.RefundTotal),
			"netSales":    yuan(d.NetSales),
			"voidCount":   d.VoidCount,
			"voidTotal":   yuan(d.VoidTotal),
			"avgTicket":   yuan(d.AvgTicket),
			"cashNet":     yuan(d.CashNet),
			"memberAmount": yuan(d.MemberAmount),
			"memberDiscountTotal": yuan(d.MemberDiscountTotal),
			"memberOrders": d.MemberOrders,
			"pointsEarned": d.PointsEarned,
			"pointsUsed":   d.PointsUsed,
			"byMethod":    methodJSON(d.ByMethod),
			"topProducts": topJSON(d.TopProducts),
			"grossProfit": yuan(d.GrossProfit),
			"hourly":      hourlyJSON(d.Hourly),
		})
	})
}

func hourlyJSON(hourly [24]int64) []float64 {
	out := make([]float64, 24)
	for i, v := range hourly {
		out[i] = yuan(v)
	}
	return out
}

func methodJSON(list []MethodStat) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, m := range list {
		out = append(out, map[string]any{"method": m.Method, "amount": yuan(m.Amount)})
	}
	return out
}

// registerMonthly 挂月度汇总与导出。
func (h *Handler) registerMonthly(mux *http.ServeMux, guard func(http.ResponseWriter, *http.Request) bool) {
	mux.HandleFunc("GET /pos/v1/reports/monthly", func(w http.ResponseWriter, r *http.Request) {
		if guard != nil && !guard(w, r) {
			return
		}
		month := r.URL.Query().Get("month")
		if month == "" {
			month = time.Now().Format("2006-01")
		}
		d, err := h.svc.Monthly(month)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"month": d.BizDate, "orderCount": d.OrderCount,
			"grossSales": yuan(d.GrossSales), "refundTotal": yuan(d.RefundTotal),
			"voidTotal": yuan(d.VoidTotal), "netSales": yuan(d.NetSales),
			"avgTicket": yuan(d.AvgTicket), "memberDiscountTotal": yuan(d.MemberDiscountTotal),
			"byMethod": methodJSON(d.ByMethod),
			"topProducts": topJSON(d.TopProducts),
			"grossProfit": yuan(d.GrossProfit),
			"hourly":      hourlyJSON(d.Hourly),
		})
	})

	mux.HandleFunc("GET /pos/v1/reports/monthly.csv", func(w http.ResponseWriter, r *http.Request) {
		if guard != nil && !guard(w, r) {
			return
		}
		month := r.URL.Query().Get("month")
		if month == "" {
			month = time.Now().Format("2006-01")
		}
		d, err := h.svc.Monthly(month)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
			return
		}
		var b strings.Builder
		b.WriteString("\ufeff")
		b.WriteString("项目,数值\n")
		fmt.Fprintf(&b, "月份,%s\n", d.BizDate)
		fmt.Fprintf(&b, "订单数,%d\n", d.OrderCount)
		fmt.Fprintf(&b, "销售合计,%.2f\n", yuan(d.GrossSales))
		fmt.Fprintf(&b, "退款合计,%.2f\n", yuan(d.RefundTotal))
		fmt.Fprintf(&b, "作废金额,%.2f\n", yuan(d.VoidTotal))
		fmt.Fprintf(&b, "净销售,%.2f\n", yuan(d.NetSales))
		fmt.Fprintf(&b, "毛利(估),%.2f\n", yuan(d.GrossProfit))
		fmt.Fprintf(&b, "客单价,%.2f\n\n", yuan(d.AvgTicket))
		b.WriteString("支付方式,金额\n")
		for _, p := range d.ByMethod {
			fmt.Fprintf(&b, "%s,%.2f\n", p.Method, yuan(p.Amount))
		}
		b.WriteString("\n商品,数量,金额\n")
		for _, p := range d.TopProducts {
			fmt.Fprintf(&b, "%s,%d,%.2f\n", p.Name, p.Qty, yuan(p.Amount))
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=monthly-"+month+".csv")
		_, _ = w.Write([]byte(b.String()))
	})
}

// RegisterCSV 挂导出接口（会计要的是文件，不是页面）。
func (h *Handler) RegisterCSV(mux *http.ServeMux, guard func(http.ResponseWriter, *http.Request) bool) {
	h.registerMonthly(mux, guard)
	mux.HandleFunc("GET /pos/v1/reports/daily.csv", func(w http.ResponseWriter, r *http.Request) {
		if guard != nil && !guard(w, r) {
			return
		}
		date := r.URL.Query().Get("date")
		if date == "" {
			date = todayBizDate(time.Now())
		}
		d, err := h.svc.Daily(date)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "统计失败"})
			return
		}
		var b strings.Builder
		b.WriteString("\ufeff")
		b.WriteString("项目,数值\n")
		fmt.Fprintf(&b, "营业日,%s\n", d.BizDate)
		fmt.Fprintf(&b, "订单数,%d\n", d.OrderCount)
		fmt.Fprintf(&b, "销售合计,%.2f\n", yuan(d.GrossSales))
		fmt.Fprintf(&b, "退款合计,%.2f\n", yuan(d.RefundTotal))
		fmt.Fprintf(&b, "作废金额,%.2f\n", yuan(d.VoidTotal))
		fmt.Fprintf(&b, "净销售,%.2f\n", yuan(d.NetSales))
		fmt.Fprintf(&b, "客单价,%.2f\n", yuan(d.AvgTicket))
		fmt.Fprintf(&b, "会员消费,%.2f\n", yuan(d.MemberAmount))
		fmt.Fprintf(&b, "积分发放,%d\n", d.PointsEarned)
		fmt.Fprintf(&b, "积分核销,%d\n\n", d.PointsUsed)
		b.WriteString("支付方式,金额\n")
		for _, p := range d.ByMethod {
			fmt.Fprintf(&b, "%s,%.2f\n", p.Method, yuan(p.Amount))
		}
		b.WriteString("\n商品,数量,金额\n")
		for _, p := range d.TopProducts {
			fmt.Fprintf(&b, "%s,%d,%.2f\n", p.Name, p.Qty, yuan(p.Amount))
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=daily-"+date+".csv")
		_, _ = w.Write([]byte(b.String()))
	})
}

func topJSON(list []ProductStat) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, p := range list {
		out = append(out, map[string]any{"name": p.Name, "qty": p.Qty, "amount": yuan(p.Amount)})
	}
	return out
}

// todayBizDate 以 04:00 为日切。
func todayBizDate(now time.Time) string {
	if now.Hour() < 4 {
		now = now.AddDate(0, 0, -1)
	}
	return now.Format("20060102")
}

func yuan(cents int64) float64 { return float64(cents) / 100 }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
