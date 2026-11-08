package catalog

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"pos/internal/store"
)

func newServer(t *testing.T) (*httptest.Server, *store.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	mux := http.NewServeMux()
	Register(mux, NewService(db))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, db
}

func doJSON(t *testing.T, ts *httptest.Server, method, path string, body any) (*http.Response, []byte) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("编码失败: %v", err)
		}
	}
	req, _ := http.NewRequest(method, ts.URL+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	data := new(bytes.Buffer)
	_, _ = data.ReadFrom(resp.Body)
	_ = resp.Body.Close()
	return resp, data.Bytes()
}

func TestListSeeds(t *testing.T) {
	ts, _ := newServer(t)
	resp, body := doJSON(t, ts, http.MethodGet, "/pos/v1/products", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 %d: %s", resp.StatusCode, body)
	}
	var list []productJSON
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(list) != 12 {
		t.Fatalf("种子商品应为 12，实际 %d", len(list))
	}
	if list[0].Name != "珍珠奶茶" {
		t.Fatalf("排序不符，首项 %q", list[0].Name)
	}
}

func TestCreateRejectsDuplicateBarcode(t *testing.T) {
	ts, _ := newServer(t)
	in := map[string]any{"name": "测试茶", "barcode": "6900000001", "category": "测试", "price": 9.0}
	resp, body := doJSON(t, ts, http.MethodPost, "/pos/v1/products", in)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("首次创建失败: %d %s", resp.StatusCode, body)
	}
	resp, _ = doJSON(t, ts, http.MethodPost, "/pos/v1/products", in)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("重复条码应 409，实际 %d", resp.StatusCode)
	}
}

func TestDeactivateHidesFromDefaultList(t *testing.T) {
	ts, _ := newServer(t)
	resp, _ := doJSON(t, ts, http.MethodPost, "/pos/v1/products/1/active", map[string]bool{"active": false})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("下架失败: %d", resp.StatusCode)
	}
	_, body := doJSON(t, ts, http.MethodGet, "/pos/v1/products", nil)
	var list []productJSON
	_ = json.Unmarshal(body, &list)
	for _, p := range list {
		if p.ID == "1" {
			t.Fatal("已下架商品不应出现在默认列表")
		}
	}
	_, body = doJSON(t, ts, http.MethodGet, "/pos/v1/products?includeInactive=true", nil)
	list = nil
	_ = json.Unmarshal(body, &list)
	found := false
	for _, p := range list {
		if p.ID == "1" {
			found = true
		}
	}
	if !found {
		t.Fatal("includeInactive=true 时应能看到已下架商品")
	}
}

func TestDeleteBlockedWhenUsedInOrders(t *testing.T) {
	ts, db := newServer(t)
	// 手工造一条历史订单明细，模拟"卖过的商品"
	if _, err := db.SQL().Exec(
		"INSERT INTO t_pos_order_item(order_id, product_id, name, price, qty, amount) VALUES(1,'1','珍珠奶茶',1300,1,1300)"); err != nil {
		t.Fatalf("插入历史明细失败: %v", err)
	}
	resp, body := doJSON(t, ts, http.MethodDelete, "/pos/v1/products/1", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("有历史订单应 409，实际 %d %s", resp.StatusCode, body)
	}
}
func TestImportTemplateReplacesSeeds(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()
	svc := NewService(db)

	items := []ImportItem{
		{Name: "可颂", Category: "面包", Barcode: "880000000001", Price: 900, Cost: 300, Hot: true},
		{Name: "贝果", Category: "面包", Barcode: "880000000002", Price: 1100, Cost: 400},
	}
	imported, replaced, err := svc.ImportTemplate(items)
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if !replaced || imported != 2 {
		t.Fatalf("应替换示例并导入 2 条，实际 replaced=%v imported=%d", replaced, imported)
	}

	all, err := svc.List("", true)
	if err != nil {
		t.Fatalf("读取商品失败: %v", err)
	}
	for _, p := range all {
		if p.Source == "seed" {
			t.Fatalf("示例商品应被清空，仍存在: %s", p.Name)
		}
	}
	if len(all) != 2 {
		t.Fatalf("导入后应只剩 2 个商品，实际 %d", len(all))
	}
}

func TestImportTemplateKeepsManualProducts(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	defer db.Close()
	svc := NewService(db)

	if _, err := svc.Create(SaveInput{Name: "自建拿铁", Category: "咖啡", Price: 18, Cost: 6}); err != nil {
		t.Fatalf("建商品失败: %v", err)
	}
	imported, replaced, err := svc.ImportTemplate([]ImportItem{
		{Name: "可颂", Category: "面包", Barcode: "880000000003", Price: 900, Cost: 300},
	})
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if replaced {
		t.Fatal("已有自建商品时不应清空示例")
	}
	if imported != 1 {
		t.Fatalf("应增量导入 1 条，实际 %d", imported)
	}
	all, _ := svc.List("", true)
	found := false
	for _, p := range all {
		if p.Name == "自建拿铁" {
			found = true
		}
	}
	if !found {
		t.Fatal("自建商品不应被删除")
	}
}
