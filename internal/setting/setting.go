// Package setting 是门店级键值配置（店名/电话/脚注）。
//
// 存在的理由：这些值原来写死在 posd 的代码里，换个店就得改代码重新编译。
package setting

import "pos/internal/store"

// 已知配置键。
const (
	KeyShopName   = "shop_name"
	KeyShopPhone  = "shop_phone"
	KeyShopFooter = "shop_footer"
	// 权限开关：默认都关（收银员不能退款/打折），与 PRD 的「默认需店长」一致。
	KeyPermRefundCashier   = "perm_refund_cashier"
	KeyPermDiscountCashier = "perm_discount_cashier"
	KeyPermPriceCashier    = "perm_change_price_cashier"
	KeyPermVoidCashier     = "perm_void_cashier"
	// 抹零方式：none / jiao（抹分，向下取整到角）/ yuan（抹角，向下取整到元）
	KeyRoundMode = "round_mode"
	// 后厨联打：打印机名（空=不启用）与需要出后厨单的类目（空=全部）
	KeyKitchenPrinter    = "kitchen_printer"
	KeyKitchenCategories = "kitchen_categories"
	// 积分规则：每消费 1 元得多少分；多少积分抵 1 元。
	KeyPointsEarn   = "points_earn_per_yuan"
	KeyPointsRedeem = "points_per_yuan"
	// 会员等级：阈值(元):名称:折扣(%):积分倍率，逗号分隔，按阈值升序
	KeyMemberTiers = "member_tiers"
	// 积分有效期（月），0=永不过期
	KeyPointsExpireMonths = "points_expire_months"
	// 外带挂单超时自动取消（分钟）；0=不取消；桌台挂单永不自动取消。
	KeyHoldExpireMinutes = "hold_expire_minutes"
	// 储值赠送规则：满多少元送多少元，逗号分隔（如 "500:50,1000:150"）。
	KeyTopupBonusRules = "topup_bonus_rules"
	// 库存不足时是否拒绝下单（默认 false：小店常"先卖后补"）。
	KeyStockBlock = "stock_block_when_short"
	// 小票页脚（可多行，\n 分隔）。
	KeyReceiptFooter = "receipt_footer"
	// 小票 logo（数据目录下的文件名，空=不打印）
	KeyReceiptLogo = "receipt_logo"
	// 小票二维码内容（URL/文本，空=不打印）
	KeyReceiptQRText = "receipt_qr_text"
	// 开箱引导状态：是否已完成首次配置、选了什么业态。
	KeySetupDone = "setup_done"
	KeyIndustry  = "industry"
)

// 默认值：全新库开箱即用。
const (
	DefaultShopName   = "沪上·真茶铺"
	DefaultShopPhone  = ""
	DefaultShopFooter = "谢谢光临，欢迎再来"
	DefaultRoundMode  = "none"
	DefaultPointsEarn = "1"
	DefaultPointsRedeem = "100"
	DefaultMemberTiers  = "0:普通会员:0:1,1000:银卡会员:2:2,5000:金卡会员:5:3"
	DefaultPointsExpire = "0"
	DefaultHoldExpire  = "30"
	DefaultTopupBonus  = ""
	DefaultStockBlock  = "false"
	DefaultReceiptFooter = ""
	DefaultReceiptLogo   = ""
	DefaultReceiptQRText = ""
)

// Service 读写配置。
type Service struct {
	db *store.DB
}

// NewService 创建服务。
func NewService(db *store.DB) *Service { return &Service{db: db} }

// All 返回全部配置。
func (s *Service) All() (map[string]string, error) {
	rows, err := s.db.SQL().Query("SELECT k, v FROM t_pos_setting")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// Get 读取单个配置，缺失返回默认值。
func (s *Service) Get(key, def string) (string, error) {
	var v string
	err := s.db.SQL().QueryRow("SELECT v FROM t_pos_setting WHERE k=?", key).Scan(&v)
	if err != nil {
		if isNoRows(err) {
			return def, nil
		}
		return def, err
	}
	return v, nil
}

// Set 写入（已存在则覆盖）。
func (s *Service) Set(key, value string) error {
	_, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_setting(k,v,updated_at) VALUES(?,?,datetime('now','localtime'))"+
			" ON CONFLICT(k) DO UPDATE SET v=excluded.v, updated_at=excluded.updated_at",
		key, value)
	return err
}

// Bool 读取布尔配置（"true"/"1" 为真），缺失用默认值。
func (s *Service) Bool(key string, def bool) (bool, error) {
	raw, err := s.Get(key, "")
	if err != nil {
		return def, err
	}
	if raw == "" {
		return def, nil
	}
	return raw == "true" || raw == "1", nil
}

// Shop 返回小票抬头三件套（带默认值）。
func (s *Service) Shop() (name, phone, footer string, err error) {
	if name, err = s.Get(KeyShopName, DefaultShopName); err != nil {
		return
	}
	if phone, err = s.Get(KeyShopPhone, DefaultShopPhone); err != nil {
		return
	}
	footer, err = s.Get(KeyShopFooter, DefaultShopFooter)
	return
}

func isNoRows(err error) bool {
	return err != nil && err.Error() == "sql: no rows in result set"
}
