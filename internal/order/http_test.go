package order

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"pos/internal/catalog"
	"pos/internal/store"
)

// 权限钩子由上层注入；这里验证「没权限就 403、有权限才放行」。
func TestPermissionGates(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()

	svc := NewService(db, catalog.NewService(db))
	allow := false
	mux := http.NewServeMux()
	Register(mux, svc, Options{
		CanDiscount: func(*http.Request) bool { return allow },
		CanRefund:   func(*http.Request) bool { return allow },
		Operator:    func(*http.Request) string { return "测试员" },
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	post := func(path, body string) *http.Response {
		resp, err := http.Post(ts.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("请求失败: %v", err)
		}
		return resp
	}

	// 无权限：打折 / 退款都应 403
	if resp := post("/pos/v1/orders", `{"cashMinor":10000,"discountMinor":500,"items":[{"productId":"1","qty":1}]}`); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("无权限打折应 403，实际 %d", resp.StatusCode)
	}

	resp := post("/pos/v1/orders", `{"cashMinor":10000,"items":[{"productId":"1","qty":1}]}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("下单失败: %d", resp.StatusCode)
	}
	var created Order
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	if resp := post("/pos/v1/orders/"+created.OrderNo+"/refund", `{"reason":"x"}`); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("无权限退款应 403，实际 %d", resp.StatusCode)
	}

	// 有权限：都能过
	allow = true
	resp = post("/pos/v1/orders", `{"cashMinor":10000,"discountMinor":500,"items":[{"productId":"1","qty":1}]}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("有权限打折应 201，实际 %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = post("/pos/v1/orders/"+created.OrderNo+"/refund", `{"reason":"顾客不要了"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("有权限退款应 200，实际 %d", resp.StatusCode)
	}
	var refunded Order
	_ = json.NewDecoder(resp.Body).Decode(&refunded)
	resp.Body.Close()
	if refunded.Status != "refunded" {
		t.Fatalf("退款后状态应为 refunded，实际 %s", refunded.Status)
	}
}

var _ = bytes.MinRead
