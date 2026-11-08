package shift

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type paymentJSON struct {
	Method      string `json:"method"`
	AmountMinor int64  `json:"amountMinor"`
}

type shiftJSON struct {
	ID          int64         `json:"id"`
	ShiftNo     string        `json:"shiftNo"`
	Status      string        `json:"status"`
	StaffName   string        `json:"staffName"`
	OpeningCashMinor int64 `json:"openingCashMinorMinor"`
	ExpectedMinor *int64 `json:"expectedCashMinor,omitempty"`
	ActualMinor *int64 `json:"actualCashMinorMinor,omitempty"`
	DifferenceMinor *int64 `json:"differenceMinor,omitempty"`
	OrderCount  int           `json:"orderCount"`
	OpenedAt    string        `json:"openedAt"`
	ClosedAt    string        `json:"closedAt,omitempty"`
	Note        string        `json:"note,omitempty"`
	Summary     summaryJSON   `json:"summary"`
}

type summaryJSON struct {
	OrderCount  int           `json:"orderCount"`
	TotalMinor int64 `json:"totalMinor"`
	CashNetMinor int64 `json:"cashNetMinor"`
	RefundTotalMinor int64 `json:"refundTotalMinor"`
	VoidTotalMinor int64 `json:"voidTotalMinor"`
	DineInCount int           `json:"dineInCount"`
	TakeawayCnt int           `json:"takeawayCount"`
	PointsEarned int64        `json:"pointsEarned"`
	PointsUsed   int64        `json:"pointsUsed"`
	CrossRefundCount int      `json:"crossRefundCount"`
	CrossRefundTotalMinor int64 `json:"crossRefundTotalMinor"`
	MemberDiscountTotalMinor int64 `json:"memberDiscountTotalMinor"`
	Payments    []paymentJSON `json:"payments"`
}

// Register 挂载交接班接口。
func Register(mux *http.ServeMux, s *Service) {
	mux.HandleFunc("GET /pos/v1/shifts/latest", func(w http.ResponseWriter, r *http.Request) {
		sh, found, err := s.Latest()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, http.StatusOK, toJSON(sh))
	})

	mux.HandleFunc("POST /pos/v1/shifts/open", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			OpeningCashMinor int64  `json:"openingCashMinor"`
			StaffName        string `json:"staffName"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "请求体不是合法 JSON")
			return
		}
		sh, err := s.Open(body.OpeningCashMinor, body.StaffName)
		if err != nil {
			writeServiceErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toJSON(sh))
	})

	mux.HandleFunc("POST /pos/v1/shifts/{id}/close", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "班次编号不合法")
			return
		}
		var body struct {
			ActualCashMinor int64  `json:"actualCashMinor"`
			Note       string  `json:"note"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "请求体不是合法 JSON")
			return
		}
		sh, err := s.Close(id, body.ActualCashMinor, body.Note)
		if err != nil {
			writeServiceErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toJSON(sh))
	})
}

func toJSON(sh Shift) shiftJSON {
	out := shiftJSON{
		ID: sh.ID, ShiftNo: sh.ShiftNo, Status: sh.Status, StaffName: sh.StaffName,
		OpeningCashMinor: sh.OpeningCash, OrderCount: sh.OrderCount,
		OpenedAt: sh.OpenedAt, ClosedAt: sh.ClosedAt, Note: sh.Note,
		Summary: summaryJSON{
			OrderCount: sh.OrderCount, TotalMinor: sh.Total,
			CashNetMinor: sh.CashNet, RefundTotalMinor: sh.RefundTotal, VoidTotalMinor: sh.VoidTotal,
			DineInCount: sh.DineInCount, TakeawayCnt: sh.TakeawayCnt,
			PointsEarned: sh.PointsEarned, PointsUsed: sh.PointsUsed,
			CrossRefundCount: sh.CrossRefundCount, CrossRefundTotalMinor: sh.CrossRefundTotal,
			MemberDiscountTotalMinor: sh.MemberDiscountTotal,
		},
	}
	if sh.Status == "closed" {
		// 用指针：差异为 0 时也要出现在 JSON 里（omitempty 会把 0 吞掉，前端显示成空）。
		e, a, d := sh.Expected, sh.Actual, sh.Difference
		out.ExpectedMinor, out.ActualMinor, out.DifferenceMinor = &e, &a, &d
	}
	for _, p := range sh.Payments {
		out.Summary.Payments = append(out.Summary.Payments, paymentJSON{Method: p.Method, AmountMinor: p.Amount})
	}
	return out
}


func writeServiceErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrConflict):
		writeErr(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrBadRequest):
		writeErr(w, http.StatusBadRequest, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"code": http.StatusText(code), "message": message})
}
