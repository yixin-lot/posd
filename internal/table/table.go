// Package table 是桌台（餐饮插件 MVP）。
//
// 语义：开台 = 建一张挂单并绑定到桌台；结账 = 给挂单收款并释放桌台。
// 桌台本身不存钱，账仍在订单上（单一真源）。
package table

import (
	"database/sql"
	"errors"
	"strings"

	"pos/internal/store"
)

var (
	// ErrNotFound 桌台不存在。
	ErrNotFound = errors.New("桌台不存在")
	// ErrConflict 重名 / 状态不允许。
	ErrConflict = errors.New("桌台状态冲突")
	// ErrBadRequest 参数不合法。
	ErrBadRequest = errors.New("参数不合法")
)

// Table 是一张桌台。
type Table struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"` // free / occupied
	OrderNo  string `json:"orderNo"`
	Guests   int    `json:"guests"`
	OpenedAt string `json:"openedAt"`
}

// Service 管理桌台。
type Service struct {
	db *store.DB
}

// NewService 创建服务。
func NewService(db *store.DB) *Service { return &Service{db: db} }

// List 返回全部桌台。
func (s *Service) List() ([]Table, error) {
	rows, err := s.db.SQL().Query(
		"SELECT id,name,status,order_no,guests,COALESCE(opened_at,'') FROM t_pos_table ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Table{}
	for rows.Next() {
		var t Table
		if err := rows.Scan(&t.ID, &t.Name, &t.Status, &t.OrderNo, &t.Guests, &t.OpenedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Create 新建桌台。
func (s *Service) Create(name string) (Table, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 20 {
		return Table{}, ErrBadRequest
	}
	res, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_table(name,status,created_at) VALUES(?,'free',datetime('now','localtime'))", name)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Table{}, ErrConflict
		}
		return Table{}, err
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

// Get 按 ID 查询。
func (s *Service) Get(id int64) (Table, error) {
	var t Table
	err := s.db.SQL().QueryRow(
		"SELECT id,name,status,order_no,guests,COALESCE(opened_at,'') FROM t_pos_table WHERE id=?", id).
		Scan(&t.ID, &t.Name, &t.Status, &t.OrderNo, &t.Guests, &t.OpenedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Table{}, ErrNotFound
	}
	return t, err
}

// Bind 开台：把挂单绑定到桌台（订单由调用方创建）。
func (s *Service) Bind(id int64, orderNo string, guests int) (Table, error) {
	t, err := s.Get(id)
	if err != nil {
		return Table{}, err
	}
	if t.Status == "occupied" {
		return Table{}, ErrConflict
	}
	if strings.TrimSpace(orderNo) == "" {
		return Table{}, ErrBadRequest
	}
	if _, err := s.db.SQL().Exec(
		"UPDATE t_pos_table SET status='occupied', order_no=?, guests=?,"+
			" opened_at=datetime('now','localtime') WHERE id=?", orderNo, guests, id); err != nil {
		return Table{}, err
	}
	return s.Get(id)
}

// Transfer 转台：把订单从一个桌台挪到另一个空闲桌台。
func (s *Service) Transfer(fromID, toID int64) (Table, Table, error) {
	from, err := s.Get(fromID)
	if err != nil {
		return Table{}, Table{}, err
	}
	if from.Status != "occupied" || from.OrderNo == "" {
		return Table{}, Table{}, ErrConflict
	}
	if fromID == toID {
		return Table{}, Table{}, ErrBadRequest
	}
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return Table{}, Table{}, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(
		"UPDATE t_pos_table SET status='occupied', order_no=?, guests=?,"+
			" opened_at=datetime('now','localtime') WHERE id=? AND status='free'",
		from.OrderNo, from.Guests, toID)
	if err != nil {
		return Table{}, Table{}, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return Table{}, Table{}, ErrConflict
	}
	if _, err := tx.Exec(
		"UPDATE t_pos_table SET status='free', order_no='', guests=0, opened_at=NULL WHERE id=?", fromID); err != nil {
		return Table{}, Table{}, err
	}
	if err := tx.Commit(); err != nil {
		return Table{}, Table{}, err
	}
	f, _ := s.Get(fromID)
	to, _ := s.Get(toID)
	return f, to, nil
}

// Release 清台（结账后调用）。
func (s *Service) Release(id int64) (Table, error) {
	if _, err := s.db.SQL().Exec(
		"UPDATE t_pos_table SET status='free', order_no='', guests=0, opened_at=NULL WHERE id=?", id); err != nil {
		return Table{}, err
	}
	return s.Get(id)
}

// Delete 删除桌台（占用中不可删）。
func (s *Service) Delete(id int64) error {
	t, err := s.Get(id)
	if err != nil {
		return err
	}
	if t.Status == "occupied" {
		return ErrConflict
	}
	_, err = s.db.SQL().Exec("DELETE FROM t_pos_table WHERE id=?", id)
	return err
}
