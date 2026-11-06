// Package templates 提供业态模板（茶饮/餐厅/烘焙/烧烤）的内嵌数据。
//
// 模板只描述「开箱默认有什么」，不参与业务逻辑；导入由 catalog 完成。
package templates

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed *.json
var files embed.FS

// Item 是模板里的一个商品（金额单位：分）。
type Item struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Price    int64  `json:"price"`
	Cost     int64  `json:"cost"`
	Hot      bool   `json:"hot"`
	Barcode  string `json:"barcode"`
}

// Template 是一个业态。
type Template struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Summary    string   `json:"summary"`
	Categories []string `json:"categories"`
	Products   []Item   `json:"products"`
}

// IDs 返回全部业态标识（排序，保证输出稳定）。
func IDs() ([]string, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		out = append(out, e.Name()[:len(e.Name())-len(".json")])
	}
	sort.Strings(out)
	return out, nil
}

// Load 读取一个业态模板。
func Load(id string) (Template, error) {
	raw, err := files.ReadFile(id + ".json")
	if err != nil {
		return Template{}, fmt.Errorf("未知业态：%s", id)
	}
	var t Template
	if err := json.Unmarshal(raw, &t); err != nil {
		return Template{}, fmt.Errorf("模板 %s 解析失败：%w", id, err)
	}
	return t, nil
}

// All 返回全部模板（按 ID 排序）。
func All() ([]Template, error) {
	ids, err := IDs()
	if err != nil {
		return nil, err
	}
	out := make([]Template, 0, len(ids))
	for _, id := range ids {
		t, err := Load(id)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}
