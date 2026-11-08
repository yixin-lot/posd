package catalog

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// 契约里商品 id 是字符串：单机版是自增主键的十进制串，SaaS 是 SKU 的 UUID。
// 统一成 string 才能让同一份收银台接两种后端（融合方案 §5）。
type productJSON struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Barcode  string  `json:"barcode,omitempty"`
	Category string  `json:"category"`
	Price    float64 `json:"price"`
	Cost     float64 `json:"cost"`
	Hot      bool    `json:"hot"`
	Active   bool    `json:"active"`
	Sort     int     `json:"sort"`
	Kind     string  `json:"kind"`
	Image    string  `json:"image,omitempty"`
	Bundle   []bundleJSON `json:"bundle,omitempty"`
}

type bundleJSON struct {
	ComponentID string `json:"componentId"`
	Name        string `json:"name"`
	Qty         int    `json:"qty"`
	GroupID     int64  `json:"groupId"`
}

type saveRequest struct {
	Kind     string  `json:"kind"`
	Bundle   []struct {
		ComponentID string `json:"componentId"`
		Qty         int    `json:"qty"`
		GroupID     int64  `json:"groupId"`
	} `json:"bundle"`
	Name     string  `json:"name"`
	Barcode  string  `json:"barcode"`
	Category string  `json:"category"`
	Price    float64 `json:"price"`
	Cost     float64 `json:"cost"`
	Hot      bool    `json:"hot"`
	Sort     int     `json:"sort"`
}

// Register 挂载商品接口（与 pos-cashier-api.yaml 对齐；管理接口为本地扩展）。
func Register(mux *http.ServeMux, s *Service) {
	mux.HandleFunc("GET /pos/v1/products", func(w http.ResponseWriter, r *http.Request) {
		includeInactive := r.URL.Query().Get("includeInactive") == "true"
		list, err := s.List(r.URL.Query().Get("keyword"), includeInactive)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		out := make([]productJSON, 0, len(list))
		for _, p := range list {
			out = append(out, toJSON(p))
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("POST /pos/v1/products", func(w http.ResponseWriter, r *http.Request) {
		in, err := decodeSave(w, r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		p, err := s.Create(in)
		if err != nil {
			writeServiceErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, toJSON(p))
	})

	mux.HandleFunc("PUT /pos/v1/products/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "商品编号不合法")
			return
		}
		in, err := decodeSave(w, r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		p, err := s.Update(id, in)
		if err != nil {
			writeServiceErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toJSON(p))
	})

	mux.HandleFunc("GET /pos/v1/products/{id}/bom", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "商品编号不合法")
			return
		}
		items, err := s.BOM(id)
		if err != nil {
			writeServiceErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
	})

	mux.HandleFunc("PUT /pos/v1/products/{id}/bom", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "商品编号不合法")
			return
		}
		var body []struct {
			ComponentID int64 `json:"componentId"`
			QtyMilli    int64 `json:"qtyMilli"`
			LossBp      int64 `json:"lossBp"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "请求体不是合法 JSON")
			return
		}
		items := make([]BOMItem, 0, len(body))
		for _, b := range body {
			items = append(items, BOMItem{ComponentID: b.ComponentID, QtyMilli: b.QtyMilli, LossBp: b.LossBp})
		}
		if err := s.SetBOM(id, items); err != nil {
			writeServiceErr(w, err)
			return
		}
		cost, _ := s.RollupCost(id, 0)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "costMinor": cost})
	})

	mux.HandleFunc("POST /pos/v1/products/{id}/active", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "商品编号不合法")
			return
		}
		var body struct {
			Active bool `json:"active"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "请求体不是合法 JSON")
			return
		}
		p, err := s.SetActive(id, body.Active)
		if err != nil {
			writeServiceErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toJSON(p))
	})

	mux.HandleFunc("DELETE /pos/v1/products/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "商品编号不合法")
			return
		}
		if err := s.Delete(id); err != nil {
			writeServiceErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /pos/v1/products/export", func(w http.ResponseWriter, r *http.Request) {
		list, err := s.List("", true)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		var b strings.Builder
		b.WriteString("\ufeff") // UTF-8 BOM：Excel 直接打开不乱码
		b.WriteString("名称,条码,类目,售价,成本,热销\n")
		for _, p := range list {
			b.WriteString(csvField(p.Name))
			b.WriteString(",")
			b.WriteString(csvField(p.Barcode))
			b.WriteString(",")
			b.WriteString(csvField(p.Category))
			b.WriteString(",")
			b.WriteString(strconv.FormatFloat(float64(p.Price)/100, 'f', -1, 64))
			b.WriteString(",")
			b.WriteString(strconv.FormatFloat(float64(p.Cost)/100, 'f', -1, 64))
			b.WriteString(",")
			if p.Hot {
				b.WriteString("是")
			} else {
				b.WriteString("否")
			}
			b.WriteString("\n")
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="products.csv"`)
		_, _ = w.Write([]byte(b.String()))
	})

	mux.HandleFunc("POST /pos/v1/products/import", func(w http.ResponseWriter, r *http.Request) {
		body, err := readBody(w, r, 5<<20)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		rows, parseErrs, err := ParseCSV(body)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		imported, dbErrs, err := s.ImportCSV(rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		errs := append(parseErrs, dbErrs...)
		writeJSON(w, http.StatusOK, map[string]any{
			"imported": imported,
			"failed":   len(errs),
			"errors":   errs,
		})
	})
}

func readBody(w http.ResponseWriter, r *http.Request, limit int64) ([]byte, error) {
	defer r.Body.Close()
	buf, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, errors.New("文件过大，单次导入不超过 5MB")
		}
		return nil, err
	}
	return buf, nil
}

// csvField 简单转义 CSV 字段：含逗号/引号/换行时加双引号包裹。
func csvField(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

func decodeSave(w http.ResponseWriter, r *http.Request) (SaveInput, error) {
	var req saveRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		return SaveInput{}, errors.New("请求体不是合法 JSON")
	}
	return toSaveInput(req), nil
}

func toJSON(p Product) productJSON {
	out := productJSON{
		ID: strconv.FormatInt(p.ID, 10), Name: p.Name, Barcode: p.Barcode, Category: p.Category,
		Price: float64(p.Price) / 100, Cost: float64(p.Cost) / 100,
		Hot: p.Hot, Active: p.Active, Sort: p.Sort, Kind: p.Kind, Image: p.Image,
	}
	if out.Kind == "" {
		out.Kind = "single"
	}
	for _, b := range p.Bundle {
		out.Bundle = append(out.Bundle, bundleJSON{
			ComponentID: strconv.FormatInt(b.ComponentID, 10), Name: b.Name, Qty: b.Qty, GroupID: b.GroupID,
		})
	}
	return out
}

// toSaveInput 把请求体转成服务层入参（含套餐组件）。
func toSaveInput(req saveRequest) SaveInput {
	in := SaveInput{
		Kind: req.Kind, Name: req.Name, Barcode: req.Barcode, Category: req.Category,
		Price: req.Price, Cost: req.Cost, Hot: req.Hot, Sort: req.Sort,
	}
	for _, b := range req.Bundle {
		id, _ := strconv.ParseInt(b.ComponentID, 10, 64)
		in.Bundle = append(in.Bundle, BundleItem{ComponentID: id, Qty: b.Qty, GroupID: b.GroupID})
	}
	return in
}

func writeServiceErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrConflict):
		writeErr(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrBadRequest):
		writeErr(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), "参数不合法: "))
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
