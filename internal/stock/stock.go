// Package stock 是商品库存（MVP）：建行即跟踪，扣减按订单、可手工调整。
//
// 设计取舍：
//   - 没建库存行的商品**不跟踪**（多数小店只关心少数几个原料/成品）；
//   - 套餐按**组件**扣减（套餐本身没有库存）；
//   - 默认允许扣成负数（店里先卖后补货是常态），是否拦截由设置决定。
package stock

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"pos/internal/store"
)

var (
	// ErrNotFound 该商品没有建库存行。
	ErrNotFound = errors.New("库存记录不存在")
	// ErrBadRequest 参数不合法。
	ErrBadRequest = errors.New("参数不合法")
)

// Row 是一条库存记录。
type Row struct {
	ProductID int64  `json:"productId"`
	Name      string `json:"name"`
	Qty       int    `json:"qty"`
	WarnQty   int    `json:"warnQty"`
	UpdatedAt string `json:"updatedAt"`
}

// Service 管理库存。
type Service struct {
	db *store.DB
}

// NewService 创建服务。
func NewService(db *store.DB) *Service { return &Service{db: db} }

// List 返回全部库存行（含商品名）。
func (s *Service) List() ([]Row, error) {
	rows, err := s.db.SQL().Query(
		"SELECT st.product_id, COALESCE(p.name,''), st.qty, st.warn_qty, st.updated_at"+
			" FROM t_pos_stock st LEFT JOIN t_pos_product p ON p.id=st.product_id"+
			" ORDER BY p.name, st.product_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Row
	for rows.Next() {
		var r Row
		if err := rows.Scan(&r.ProductID, &r.Name, &r.Qty, &r.WarnQty, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Alerts 返回需要补货的行（warn_qty>0 且 qty<=warn_qty）。
func (s *Service) Alerts() ([]Row, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	var out []Row
	for _, r := range all {
		if r.WarnQty > 0 && r.Qty <= r.WarnQty {
			out = append(out, r)
		}
	}
	return out, nil
}

// Set 建/改库存行（qty 为绝对值）。
func (s *Service) Set(productID int64, qty, warnQty int) (Row, error) {
	if productID <= 0 {
		return Row{}, fmt.Errorf("%w: 商品不存在", ErrBadRequest)
	}
	if warnQty < 0 {
		return Row{}, fmt.Errorf("%w: 告警阈值不能为负", ErrBadRequest)
	}
	// 商品必须存在：否则会建出一条指向不存在商品的孤儿库存
	var exists int
	if err := s.db.SQL().QueryRow("SELECT COUNT(*) FROM t_pos_product WHERE id=?", productID).Scan(&exists); err != nil {
		return Row{}, err
	}
	if exists == 0 {
		return Row{}, fmt.Errorf("%w: 商品不存在", ErrBadRequest)
	}
	if _, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_stock(product_id,qty,warn_qty,updated_at) VALUES(?,?,?,datetime('now','localtime'))"+
			" ON CONFLICT(product_id) DO UPDATE SET qty=excluded.qty, warn_qty=excluded.warn_qty,"+
			" updated_at=excluded.updated_at",
		productID, qty, warnQty); err != nil {
		return Row{}, err
	}
	return s.Get(productID)
}

// Adjust 增减库存（入库为正、报损为负）。
func (s *Service) Adjust(productID int64, delta int, reason string) (Row, error) {
	if delta == 0 {
		return Row{}, fmt.Errorf("%w: 调整量不能为 0", ErrBadRequest)
	}
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return Row{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var exists int
	if err := tx.QueryRow("SELECT COUNT(*) FROM t_pos_stock WHERE product_id=?", productID).Scan(&exists); err != nil {
		return Row{}, err
	}
	if exists == 0 {
		return Row{}, ErrNotFound
	}
	if _, err := tx.Exec(
		"UPDATE t_pos_stock SET qty=qty+?, updated_at=datetime('now','localtime') WHERE product_id=?",
		delta, productID); err != nil {
		return Row{}, err
	}
	if _, err := tx.Exec(
		"INSERT INTO t_pos_audit(action,order_no,product_id,detail,operator,created_at)"+
			" VALUES('stock_adjust','',?,?,?,datetime('now','localtime'))",
		fmt.Sprint(productID), fmt.Sprintf("%+d %s", delta, strings.TrimSpace(reason)), ""); err != nil {
		return Row{}, err
	}
	if err := tx.Commit(); err != nil {
		return Row{}, err
	}
	return s.Get(productID)
}

// Get 读一行。
func (s *Service) Get(productID int64) (Row, error) {
	var r Row
	err := s.db.SQL().QueryRow(
		"SELECT st.product_id, COALESCE(p.name,''), st.qty, st.warn_qty, st.updated_at"+
			" FROM t_pos_stock st LEFT JOIN t_pos_product p ON p.id=st.product_id WHERE st.product_id=?",
		productID).Scan(&r.ProductID, &r.Name, &r.Qty, &r.WarnQty, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Row{}, ErrNotFound
	}
	return r, err
}

// DeductTx 在调用方事务里扣减库存（未跟踪的商品跳过）。
// items 是「商品 ID + 数量」，套餐必须由调用方展开成组件。
// blockShort=true 时，扣完会负库存则返回错误。
func (s *Service) DeductTx(tx *sql.Tx, items map[int64]int, blockShort bool) ([]string, error) {
	var short []string
	for pid, qty := range items {
		if qty <= 0 {
			continue
		}
		var cur int
		err := tx.QueryRow("SELECT qty FROM t_pos_stock WHERE product_id=?", pid).Scan(&cur)
		if errors.Is(err, sql.ErrNoRows) {
			continue // 未跟踪
		}
		if err != nil {
			return nil, err
		}
		if blockShort && cur < qty {
			var name string
			_ = tx.QueryRow("SELECT name FROM t_pos_product WHERE id=?", pid).Scan(&name)
			short = append(short, fmt.Sprintf("%s（剩 %d，需 %d）", name, cur, qty))
			continue
		}
		if _, err := tx.Exec(
			"UPDATE t_pos_stock SET qty=qty-?, updated_at=datetime('now','localtime') WHERE product_id=?",
			qty, pid); err != nil {
			return nil, err
		}
	}
	return short, nil
}

// RestoreTx 在调用方事务里把库存加回来（退款/作废）。
func (s *Service) RestoreTx(tx *sql.Tx, items map[int64]int) error {
	for pid, qty := range items {
		if qty <= 0 {
			continue
		}
		if _, err := tx.Exec(
			"UPDATE t_pos_stock SET qty=qty+?, updated_at=datetime('now','localtime') WHERE product_id=?",
			qty, pid); err != nil {
			return err
		}
	}
	return nil
}
