// Package category 是商品类目主数据。
//
// 商品表仍用文本 category（避免大改），本包负责"类目清单、改名、删除保护"：
// 改名会级联更新所有商品的 category，保证不会出现"类目表里有、商品里是老名字"。
package category

import (
	"database/sql"
	"errors"
	"strings"

	"pos/internal/store"
)

var (
	// ErrNotFound 类目不存在。
	ErrNotFound = errors.New("类目不存在")
	// ErrConflict 重名 / 仍被商品引用。
	ErrConflict = errors.New("类目状态冲突")
	// ErrBadRequest 参数不合法。
	ErrBadRequest = errors.New("参数不合法")
)

// Category 是一个类目。
type Category struct {
	ID    int64
	Name  string
	Sort  int
	Count int
}

// Service 管理类目。
type Service struct {
	db *store.DB
}

// NewService 创建服务。
func NewService(db *store.DB) *Service { return &Service{db: db} }

// List 返回类目（带商品数）。
func (s *Service) List() ([]Category, error) {
	rows, err := s.db.SQL().Query(
		"SELECT c.id, c.name, c.sort, " +
			"(SELECT COUNT(*) FROM t_pos_product p WHERE p.category = c.name) " +
			"FROM t_pos_category c ORDER BY c.sort, c.id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Sort, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Create 新建类目。
func (s *Service) Create(name string, sort int) (Category, error) {
	name, err := normalize(name)
	if err != nil {
		return Category{}, err
	}
	res, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_category(name, sort, created_at) VALUES(?,?,datetime('now','localtime'))", name, sort)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Category{}, ErrConflict
		}
		return Category{}, err
	}
	id, _ := res.LastInsertId()
	return Category{ID: id, Name: name, Sort: sort}, nil
}

// Rename 改名并级联更新商品（同一事务）。
func (s *Service) Rename(id int64, name string, sort int) (Category, error) {
	name, err := normalize(name)
	if err != nil {
		return Category{}, err
	}
	var old string
	if err := s.db.SQL().QueryRow("SELECT name FROM t_pos_category WHERE id=?", id).Scan(&old); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Category{}, ErrNotFound
		}
		return Category{}, err
	}

	tx, err := s.db.SQL().Begin()
	if err != nil {
		return Category{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("UPDATE t_pos_category SET name=?, sort=? WHERE id=?", name, sort, id); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Category{}, ErrConflict
		}
		return Category{}, err
	}
	if old != name {
		if _, err := tx.Exec("UPDATE t_pos_product SET category=? WHERE category=?", name, old); err != nil {
			return Category{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Category{}, err
	}
	return Category{ID: id, Name: name, Sort: sort}, nil
}

// Delete 删除类目；仍被商品引用时拒绝。
func (s *Service) Delete(id int64) error {
	var name string
	if err := s.db.SQL().QueryRow("SELECT name FROM t_pos_category WHERE id=?", id).Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	var n int
	if err := s.db.SQL().QueryRow("SELECT COUNT(*) FROM t_pos_product WHERE category=?", name).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrConflict
	}
	_, err := s.db.SQL().Exec("DELETE FROM t_pos_category WHERE id=?", id)
	return err
}

func normalize(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 20 {
		return "", ErrBadRequest
	}
	return name, nil
}
