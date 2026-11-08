package main

import (
	"strings"

	"pos/internal/setting"
	"pos/internal/store"
	"pos/internal/templates"
)

// localizeSeedDemo 把内置示例商品/门店名切到英文，避免英文界面里商品仍是中文。
// 只改 source='seed' 的行，不动商家自己建的商品。
func localizeSeedDemo(db *store.DB, svc *setting.Service) {
	lang, _ := svc.Get(setting.KeyLocale, setting.DefaultLocale)
	if !strings.HasPrefix(strings.ToLower(lang), "en") {
		return
	}
	all, err := templates.All()
	if err != nil {
		return
	}
	names := map[string]string{}
	cats := map[string]string{}
	for _, t := range all {
		for _, p := range t.Products {
			if p.NameEN != "" && p.Name != p.NameEN {
				names[p.Name] = p.NameEN
			}
			if p.Category != "" && p.CategoryEN != "" && p.Category != p.CategoryEN {
				cats[p.Category] = p.CategoryEN
			}
		}
	}
	sqlDB := db.SQL()
	for zh, en := range names {
		_, _ = sqlDB.Exec("UPDATE t_pos_product SET name=? WHERE source='seed' AND name=?", en, zh)
	}
	for zh, en := range cats {
		_, _ = sqlDB.Exec("UPDATE t_pos_product SET category=? WHERE source='seed' AND category=?", en, zh)
	}
	if v, _ := svc.Get(setting.KeyShopName, setting.DefaultShopName); v == setting.DefaultShopName {
		_ = svc.Set(setting.KeyShopName, "The Tea House")
	}
	if v, _ := svc.Get(setting.KeyShopFooter, setting.DefaultShopFooter); v == setting.DefaultShopFooter {
		_ = svc.Set(setting.KeyShopFooter, "Thank you, please come again")
	}
}
