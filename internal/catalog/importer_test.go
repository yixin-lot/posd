package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestParseCSV_OK：正常 50 行全部通过，字段解析正确。
func TestParseCSV_OK(t *testing.T) {
	var b strings.Builder
	b.WriteString("名称,条码,类目,售价,成本,热销\n")
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&b, "商品%d,690000%05d,饮品,%.1f,%.1f,是\n", i, i, float64(i)+0.5, float64(i)*0.3)
	}
	rows, errs, err := ParseCSV([]byte(b.String()))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("期望 0 失败行，实际 %d: %+v", len(errs), errs)
	}
	if len(rows) != 50 {
		t.Fatalf("期望 50 行，实际 %d", len(rows))
	}
	if rows[0].Name != "商品1" || rows[0].Barcode != "69000000001" || !rows[0].Hot {
		t.Fatalf("首行解析错误: %+v", rows[0])
	}
	if rows[49].Line != 51 {
		t.Fatalf("末行行号错误: got %d want 51", rows[49].Line)
	}
}

// TestParseCSV_DuplicateBarcode：文件内重复条码，第二行报错且带行号。
func TestParseCSV_DuplicateBarcode(t *testing.T) {
	data := "名称,条码,类目,售价,成本,热销\n" +
		"面包A,690001,面包,12,5,否\n" +
		"面包B,690001,面包,15,6,否\n" +
		"面包C,690002,面包,8,3,否\n"
	rows, errs, err := ParseCSV([]byte(data))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("期望 2 成功行，实际 %d", len(rows))
	}
	if len(errs) != 1 {
		t.Fatalf("期望 1 失败行，实际 %d: %+v", len(errs), errs)
	}
	if errs[0].Line != 3 {
		t.Fatalf("重复条码行号错误: got %d want 3", errs[0].Line)
	}
	if !strings.Contains(errs[0].Message, "重复") {
		t.Fatalf("错误信息应含\"重复\": %s", errs[0].Message)
	}
}

// TestParseCSV_EmptyName：空名称报错且带行号；空条码允许。
func TestParseCSV_EmptyName(t *testing.T) {
	data := "名称,条码,类目,售价,成本,热销\n" +
		",690003,面包,12,5,否\n" +
		"正常商品,,面包,10,4,否\n"
	rows, errs, err := ParseCSV([]byte(data))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(errs) != 1 || errs[0].Line != 2 {
		t.Fatalf("期望第 2 行报错，实际 %+v", errs)
	}
	if !strings.Contains(errs[0].Message, "商品名不能为空") {
		t.Fatalf("错误信息不符: %s", errs[0].Message)
	}
	if len(rows) != 1 || rows[0].Barcode != "" {
		t.Fatalf("空条码行应通过: %+v", rows)
	}
}

// TestParseCSV_BadPrice：售价非正数报错。
func TestParseCSV_BadPrice(t *testing.T) {
	data := "名称,条码,类目,售价,成本,热销\n" +
		"免费送,690004,面包,0,0,否\n"
	_, errs, err := ParseCSV([]byte(data))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "售价") {
		t.Fatalf("期望售价报错，实际 %+v", errs)
	}
}

// TestImportExportHTTP：走 HTTP 全链路——导入 50 行成功、重复条码报行号、导出带 BOM。
func TestImportExportHTTP(t *testing.T) {
	ts, _ := newServer(t)

	// 导入 50 行
	var b strings.Builder
	b.WriteString("名称,条码,类目,售价,成本,热销\n")
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&b, "导入商品%d,699%05d,饮品,%.1f,%.1f,是\n", i, i, float64(i)+0.5, float64(i)*0.3)
	}
	resp, body := doRaw(t, ts, http.MethodPost, "/pos/v1/products/import",
		"text/csv; charset=utf-8", []byte(b.String()))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("导入状态码 %d: %s", resp.StatusCode, string(body))
	}
	var out struct {
		Imported int `json:"imported"`
		Failed   int `json:"failed"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("响应解析失败: %v", err)
	}
	if out.Imported != 50 || out.Failed != 0 {
		t.Fatalf("期望 imported=50 failed=0，实际 %+v", out)
	}

	// 再导同文件 → 50 行全部因条码已存在失败，报行号
	resp, body = doRaw(t, ts, http.MethodPost, "/pos/v1/products/import",
		"text/csv; charset=utf-8", []byte(b.String()))
	_ = json.Unmarshal(body, &out)
	if out.Imported != 0 || out.Failed != 50 {
		t.Fatalf("重复导入应全失败，实际 %+v", out)
	}
	var full struct {
		Errors []RowError `json:"errors"`
	}
	_ = json.Unmarshal(body, &full)
	if len(full.Errors) == 0 || full.Errors[0].Line != 2 {
		t.Fatalf("失败行应带行号，实际 %+v", full.Errors)
	}
	if !strings.Contains(full.Errors[0].Message, "已存在") {
		t.Fatalf("失败原因应为条码已存在: %s", full.Errors[0].Message)
	}

	// 导出：带 UTF-8 BOM 与表头
	resp, body = doRaw(t, ts, http.MethodGet, "/pos/v1/products/export", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("导出状态码 %d", resp.StatusCode)
	}
	if !bytes.HasPrefix(body, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatal("导出缺少 UTF-8 BOM")
	}
	text := string(body)
	if !strings.HasPrefix(text, "\ufeff名称,条码,类目,售价,成本,热销\n") {
		t.Fatalf("导出表头错误: %q", text[:40])
	}
	if !strings.Contains(text, "导入商品1,69900001,饮品,") {
		t.Fatalf("导出缺少导入的商品: %q", text)
	}
}

func doRaw(t *testing.T, ts *httptest.Server, method, path, contentType string, body []byte) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, ts.URL+path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	_ = resp.Body.Close()
	return resp, buf.Bytes()
}
