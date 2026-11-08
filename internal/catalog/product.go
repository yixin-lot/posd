// Package catalog 是商品域（P3）：CRUD、上下架、条码唯一、删除保护。
//
// 金额一律以「分」存库；只在 HTTP 边界转成元（float64 仅用于展示，不参与计算）。
package catalog

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"

	"pos/internal/store"
)

var (
	// ErrNotFound 商品不存在。
	ErrNotFound = errors.New("商品不存在")
	// ErrConflict 条码冲突 / 有历史订单。
	ErrConflict = errors.New("商品状态冲突")
	// ErrBadRequest 参数不合法。
	ErrBadRequest = errors.New("参数不合法")
)

// Product 是商品记录（金额单位：分）。
type Product struct {
	ID       int64
	Name     string
	Barcode  string
	Category string
	Price    int64
	Cost     int64
	Hot      bool
	Active   bool
	Sort     int
	Source   string // seed / template / manual
	Kind     string // single / bundle
	// Image 是商品图片的相对路径（images/xxx.png），空表示没有图
	Image    string
	Bundle   []BundleItem // 套餐组件（Kind=bundle 时有值）
}

// BundleItem 是套餐里的一个组件。
type BundleItem struct {
	ComponentID int64
	Name        string
	Qty         int
	// GroupID > 0 表示「可选项组」：同组组件互为替代，下单时只出其中一个。
	// 0 表示固定组件（每次都有）。
	GroupID int64
}

// SaveInput 是新建/编辑的入参（金额单位：元）。
type SaveInput struct {
	Kind     string // single / bundle（空=single）
	Bundle   []BundleItem
	Name     string
	Barcode  string
	Category string
	Price    float64
	Cost     float64
	Hot      bool
	Sort     int
}

// Service 提供商品用例。
type Service struct {
	db *store.DB
}

// NewService 创建商品服务。
func NewService(db *store.DB) *Service { return &Service{db: db} }

const columns = "id, name, COALESCE(barcode,''), category, price, cost, hot, active, sort, source, kind, COALESCE(image,'')"

// List 查询商品；inactive=false 时只返回在售。
func (s *Service) List(keyword string, includeInactive bool) ([]Product, error) {
	sqlText := "SELECT " + columns + " FROM t_pos_product WHERE shop_id=1"
	args := []any{}
	if !includeInactive {
		sqlText += " AND active=1"
	}
	if kw := strings.TrimSpace(keyword); kw != "" {
		sqlText += " AND (name LIKE ? OR barcode = ?)"
		args = append(args, "%"+kw+"%", kw)
	}
	sqlText += " ORDER BY sort, id"

	rows, err := s.db.SQL().Query(sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Product
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 列表也要带套餐组件：收银台据 groupId 弹「选一个」。
	// 只加载 Get 不够——列表接口返回的套餐会被前端当成空套餐。
	for i := range out {
		if out[i].Kind == "bundle" {
			out[i].Bundle, _ = s.bundleOf(out[i].ID)
		}
	}
	return out, nil
}

// SetImage 记录商品图片相对路径（文件本身由上层保存）。
func (s *Service) SetImage(id int64, path string) error {
	res, err := s.db.SQL().Exec("UPDATE t_pos_product SET image=? WHERE id=?", path, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Get 按 ID 查询。
func (s *Service) Get(id int64) (Product, error) {
	row := s.db.SQL().QueryRow("SELECT "+columns+" FROM t_pos_product WHERE id=?", id)
	p, err := scan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	p.Bundle, _ = s.bundleOf(id)
	return p, nil
}

// bundleOf 读取套餐组件（含组件名，便于打印与前端展示）。
func (s *Service) bundleOf(productID int64) ([]BundleItem, error) {
	rows, err := s.db.SQL().Query(
		"SELECT b.component_id, COALESCE(p.name,''), b.qty, COALESCE(b.group_id,0) FROM t_pos_product_bundle b"+
			" LEFT JOIN t_pos_product p ON p.id=b.component_id WHERE b.product_id=? ORDER BY b.group_id, b.component_id", productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BundleItem
	for rows.Next() {
		var it BundleItem
		if err := rows.Scan(&it.ComponentID, &it.Name, &it.Qty, &it.GroupID); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// Create 新建商品。
func (s *Service) Create(in SaveInput) (Product, error) {
	p, err := normalize(in)
	if err != nil {
		return Product{}, err
	}
	if err := s.ensureBarcodeFree(p.Barcode, 0); err != nil {
		return Product{}, err
	}
	res, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_product(shop_id,name,barcode,category,price,cost,hot,active,sort,kind,created_at,updated_at)"+
			" VALUES(1,?,?,?,?,?,?,1,?,?,datetime('now','localtime'),datetime('now','localtime'))",
		p.Name, nullable(p.Barcode), p.Category, p.Price, p.Cost, boolInt(p.Hot), p.Sort, p.Kind)
	if err != nil {
		if isUnique(err) {
			return Product{}, fmt.Errorf("%w: 条码已存在", ErrConflict)
		}
		return Product{}, err
	}
	id, _ := res.LastInsertId()
	if err := s.saveBundle(id, in.Bundle); err != nil {
		return Product{}, err
	}
	return s.Get(id)
}

// saveBundle 覆盖写套餐组件（先删后插，保持与入参一致）。
func (s *Service) saveBundle(productID int64, items []BundleItem) error {
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec("DELETE FROM t_pos_product_bundle WHERE product_id=?", productID); err != nil {
		return err
	}
	for _, it := range items {
		if it.ComponentID <= 0 || it.Qty <= 0 {
			continue
		}
		if it.ComponentID == productID {
			return fmt.Errorf("%w: 套餐不能包含自己", ErrBadRequest)
		}
		if _, err := tx.Exec(
			"INSERT INTO t_pos_product_bundle(product_id,component_id,qty,group_id) VALUES(?,?,?,?)",
			productID, it.ComponentID, it.Qty, it.GroupID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Update 编辑商品。
func (s *Service) Update(id int64, in SaveInput) (Product, error) {
	old, err := s.Get(id)
	if err != nil {
		return Product{}, err
	}
	// PUT 是部分更新：客户端省略 kind/bundle 时保留原值。
	// 否则编辑一次套餐就会因为 kind 为空被 normalize 成 single，组件也被清空（线上事故）。
	if strings.TrimSpace(in.Kind) == "" {
		in.Kind = old.Kind
	}
	if in.Bundle == nil && old.Kind == "bundle" {
		in.Bundle = old.Bundle
	}
	p, err := normalize(in)
	if err != nil {
		return Product{}, err
	}
	if err := s.ensureBarcodeFree(p.Barcode, old.ID); err != nil {
		return Product{}, err
	}
	if _, err := s.db.SQL().Exec(
		"UPDATE t_pos_product SET name=?,barcode=?,category=?,price=?,cost=?,hot=?,sort=?,kind=?,"+
			"updated_at=datetime('now','localtime') WHERE id=?",
		p.Name, nullable(p.Barcode), p.Category, p.Price, p.Cost, boolInt(p.Hot), p.Sort, p.Kind, id); err != nil {
		if isUnique(err) {
			return Product{}, fmt.Errorf("%w: 条码已存在", ErrConflict)
		}
		return Product{}, err
	}
	if err := s.saveBundle(id, in.Bundle); err != nil {
		return Product{}, err
	}
	return s.Get(id)
}

// SetActive 上架/下架。
func (s *Service) SetActive(id int64, active bool) (Product, error) {
	if _, err := s.Get(id); err != nil {
		return Product{}, err
	}
	if _, err := s.db.SQL().Exec(
		"UPDATE t_pos_product SET active=?, updated_at=datetime('now','localtime') WHERE id=?",
		boolInt(active), id); err != nil {
		return Product{}, err
	}
	return s.Get(id)
}

// Delete 删除商品；有历史订单只能下架。
func (s *Service) Delete(id int64) error {
	if _, err := s.Get(id); err != nil {
		return err
	}
	var used int
	if err := s.db.SQL().QueryRow("SELECT COUNT(*) FROM t_pos_order_item WHERE product_id=?",
		fmt.Sprint(id)).Scan(&used); err != nil {
		return err
	}
	if used > 0 {
		return fmt.Errorf("%w: 该商品有历史订单，只能下架，不能删除", ErrConflict)
	}
	_, err := s.db.SQL().Exec("DELETE FROM t_pos_product WHERE id=?", id)
	return err
}

func (s *Service) ensureBarcodeFree(barcode string, selfID int64) error {
	if barcode == "" {
		return nil
	}
	var id int64
	err := s.db.SQL().QueryRow("SELECT id FROM t_pos_product WHERE barcode=?", barcode).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if id != selfID {
		return fmt.Errorf("%w: 条码已存在", ErrConflict)
	}
	return nil
}

func normalize(in SaveInput) (Product, error) {
	// 套餐校验：必须有组件、组件不能是套餐（不做嵌套，避免无限递归与库存口径混乱）
	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		kind = "single"
	}
	if kind != "single" && kind != "bundle" {
		return Product{}, fmt.Errorf("%w: 商品类型只能是 single 或 bundle", ErrBadRequest)
	}
	if kind == "bundle" && len(in.Bundle) == 0 {
		return Product{}, fmt.Errorf("%w: 套餐至少要有一个组件", ErrBadRequest)
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Product{}, fmt.Errorf("%w: 商品名不能为空", ErrBadRequest)
	}
	if in.Price <= 0 {
		return Product{}, fmt.Errorf("%w: 售价必须大于 0", ErrBadRequest)
	}
	if in.Cost < 0 {
		return Product{}, fmt.Errorf("%w: 进价不能为负", ErrBadRequest)
	}
	return Product{
		Name:     name,
		Barcode:  strings.TrimSpace(in.Barcode),
		Category: strings.TrimSpace(in.Category),
		Price:    yuanToCents(in.Price),
		Cost:     yuanToCents(in.Cost),
		Hot:      in.Hot,
		Active:   true,
		Sort:     in.Sort,
		Kind:     kind,
		Source:   "manual",
	}, nil
}

func scan(sc interface{ Scan(...any) error }) (Product, error) {
	var p Product
	err := sc.Scan(&p.ID, &p.Name, &p.Barcode, &p.Category, &p.Price, &p.Cost, &p.Hot, &p.Active, &p.Sort, &p.Source, &p.Kind, &p.Image)
	return p, err
}

func yuanToCents(y float64) int64 { return int64(math.Round(y * 100)) }

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isUnique(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
// ImportItem 是待导入的模板商品（金额单位：分）。
type ImportItem struct {
	Name     string
	Category string
	Barcode  string
	Price    int64
	Cost     int64
	Hot      bool
}

// ImportTemplate 按业态模板重置商品：
//
//   - 库里**没有**自建商品（source='manual'）时：删除示例商品（source='seed'）后整批导入，
//     得到「这就是我的店」的效果；
//   - 已有自建商品时：只做增量导入（条码已存在则跳过），绝不删除店主的东西。
//
// 返回导入条数与是否清过示例商品。
func (s *Service) ImportTemplate(items []ImportItem) (int, bool, error) {
	var manual int
	if err := s.db.SQL().QueryRow("SELECT COUNT(*) FROM t_pos_product WHERE source='manual'").Scan(&manual); err != nil {
		return 0, false, err
	}
	replaced := false
	if manual == 0 {
		if _, err := s.db.SQL().Exec("DELETE FROM t_pos_product WHERE source='seed'"); err != nil {
			return 0, false, err
		}
		replaced = true
	}

	tx, err := s.db.SQL().Begin()
	if err != nil {
		return 0, replaced, err
	}
	defer func() { _ = tx.Rollback() }()

	imported := 0
	for i, it := range items {
		res, err := tx.Exec(
			"INSERT OR IGNORE INTO t_pos_product(shop_id,name,barcode,category,price,cost,hot,active,sort,"+
				"source,created_at,updated_at)"+
				" VALUES(1,?,?,?,?,?,?,1,?,'template',datetime('now','localtime'),datetime('now','localtime'))",
			it.Name, it.Barcode, it.Category, it.Price, it.Cost, boolInt(it.Hot), i+1)
		if err != nil {
			return imported, replaced, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			imported++
		}
	}
	if err := tx.Commit(); err != nil {
		return imported, replaced, err
	}
	return imported, replaced, nil
}
