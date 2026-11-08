// Package purchase 是采购入库：记一张进货单，同时把数量加到库存。
//
// 取舍：
//   - 进价按「分/件」存，便于后面算毛利；
//   - 未建库存行的商品，入库时自动建行（进货就意味着要管它了）。
package purchase

import (
	"database/sql"
	"errors"
	"fmt"

	"pos/internal/store"
)

var (
	// ErrNotFound 进货单不存在。
	ErrNotFound = errors.New("进货单不存在")
	// ErrBadRequest 参数不合法。
	ErrBadRequest = errors.New("参数不合法")
)

// Item 是一条进货明细。
type Item struct {
	ProductID int64  `json:"productId"`
	Name      string `json:"name"`
	Qty       int    `json:"qty"`
	UnitCost  int64  `json:"unitCost"` // 分
	Amount    int64  `json:"amount"`   // 分
}

// Order 是一张进货单。
type Order struct {
	ID        int64  `json:"id"`
	Supplier  string `json:"supplier"`
	Note      string `json:"note"`
	Operator  string `json:"operator"`
	TotalCost int64  `json:"totalCost"` // 分
	CreatedAt string `json:"createdAt"`
	Items     []Item `json:"items"`
}

// InputItem 是入库入参。
type InputItem struct {
	ProductID int64
	Qty       int
	UnitCost  int64
}

// Service 管理进货单。
type Service struct {
	db *store.DB
}

// NewService 创建服务。
func NewService(db *store.DB) *Service { return &Service{db: db} }

// Create 建单并入库存（同一事务）。
func (s *Service) Create(supplier, note, operator string, items []InputItem) (Order, error) {
	if len(items) == 0 {
		return Order{}, fmt.Errorf("%w: 至少一条进货明细", ErrBadRequest)
	}
	for _, it := range items {
		if it.Qty <= 0 || it.UnitCost < 0 {
			return Order{}, fmt.Errorf("%w: 数量必须为正、进价不能为负", ErrBadRequest)
		}
	}

	tx, err := s.db.SQL().Begin()
	if err != nil {
		return Order{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var total int64
	for _, it := range items {
		total += int64(it.Qty) * it.UnitCost
	}
	res, err := tx.Exec(
		"INSERT INTO t_pos_purchase(supplier,note,operator,total_cost,created_at)"+
			" VALUES(?,?,?,?,datetime('now','localtime'))",
		supplier, note, operator, total)
	if err != nil {
		return Order{}, err
	}
	id, _ := res.LastInsertId()

	for _, it := range items {
		var name string
		if err := tx.QueryRow("SELECT name FROM t_pos_product WHERE id=?", it.ProductID).Scan(&name); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return Order{}, fmt.Errorf("%w: 商品 %d 不存在", ErrBadRequest, it.ProductID)
			}
			return Order{}, err
		}
		if _, err := tx.Exec(
			"INSERT INTO t_pos_purchase_item(purchase_id,product_id,qty,unit_cost) VALUES(?,?,?,?)"+
				" ON CONFLICT(purchase_id,product_id) DO UPDATE SET qty=qty+excluded.qty, unit_cost=excluded.unit_cost",
			id, it.ProductID, it.Qty, it.UnitCost); err != nil {
			return Order{}, err
		}
		// 入库：没有库存行就建一行（进货即纳入管理）
		if _, err := tx.Exec(
			"INSERT INTO t_pos_stock(product_id,qty,warn_qty,updated_at) VALUES(?,?,0,datetime('now','localtime'))"+
				" ON CONFLICT(product_id) DO UPDATE SET qty=qty+excluded.qty, updated_at=excluded.updated_at",
			it.ProductID, it.Qty); err != nil {
			return Order{}, err
		}
		if _, err := tx.Exec(
			"INSERT INTO t_pos_audit(action,order_no,product_id,detail,operator,created_at)"+
				" VALUES('purchase','',?,?,?,datetime('now','localtime'))",
			fmt.Sprint(it.ProductID),
			fmt.Sprintf("进货#%d +%d，进价 %d 分", id, it.Qty, it.UnitCost),
			operator); err != nil {
			return Order{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	return s.Get(id)
}

// List 返回进货单列表（不含明细）。
func (s *Service) List(limit int) ([]Order, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.SQL().Query(
		"SELECT id,supplier,note,operator,total_cost,created_at FROM t_pos_purchase ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Order{}
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.Supplier, &o.Note, &o.Operator, &o.TotalCost, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Get 读一张进货单（含明细与商品名）。
func (s *Service) Get(id int64) (Order, error) {
	var o Order
	err := s.db.SQL().QueryRow(
		"SELECT id,supplier,note,operator,total_cost,created_at FROM t_pos_purchase WHERE id=?", id).
		Scan(&o.ID, &o.Supplier, &o.Note, &o.Operator, &o.TotalCost, &o.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	rows, err := s.db.SQL().Query(
		"SELECT i.product_id, COALESCE(p.name,''), i.qty, i.unit_cost"+
			" FROM t_pos_purchase_item i LEFT JOIN t_pos_product p ON p.id=i.product_id"+
			" WHERE i.purchase_id=? ORDER BY p.name", id)
	if err != nil {
		return Order{}, err
	}
	defer rows.Close()
	o.Items = []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ProductID, &it.Name, &it.Qty, &it.UnitCost); err != nil {
			return Order{}, err
		}
		it.Amount = int64(it.Qty) * it.UnitCost
		o.Items = append(o.Items, it)
	}
	return o, rows.Err()
}
