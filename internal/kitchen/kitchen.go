// Package kitchen 是后厨档口：一个档口 = 一台打印机 + 一组类目。
//
// 下单时按档口过滤菜品，每个档口单独出一张后厨单（不含价格）。
package kitchen

import (
	"errors"
	"strings"

	"pos/internal/print"

	"pos/internal/store"
)

var (
	// ErrNotFound 档口不存在。
	ErrNotFound = errors.New("档口不存在")
	// ErrConflict 重名。
	ErrConflict = errors.New("档口重名")
	// ErrBadRequest 参数不合法。
	ErrBadRequest = errors.New("参数不合法")
	// ErrBadPrinter 打印机地址不合法。
	ErrBadPrinter = errors.New("打印机地址不合法")
)

// Station 是一个档口。
type Station struct {
	ID         int64
	Name       string
	Printer    string
	Categories string
	Active     bool
}

// Service 管理档口。
type Service struct {
	db *store.DB
}

// NewService 创建服务。
func NewService(db *store.DB) *Service { return &Service{db: db} }

// List 返回全部档口。
func (s *Service) List() ([]Station, error) {
	rows, err := s.db.SQL().Query(
		"SELECT id,name,printer,categories,active FROM t_pos_kitchen_station ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Station{}
	for rows.Next() {
		var st Station
		var active int
		if err := rows.Scan(&st.ID, &st.Name, &st.Printer, &st.Categories, &active); err != nil {
			return nil, err
		}
		st.Active = active == 1
		out = append(out, st)
	}
	return out, rows.Err()
}

// Create 新建档口。
func (s *Service) Create(name, printer, categories string) (Station, error) {
	name, printer, categories, err := normalize(name, printer, categories)
	if err != nil {
		return Station{}, err
	}
	res, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_kitchen_station(name,printer,categories,active,created_at)"+
			" VALUES(?,?,?,1,datetime('now','localtime'))", name, printer, categories)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Station{}, ErrConflict
		}
		return Station{}, err
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

// Update 修改档口。
func (s *Service) Update(id int64, name, printer, categories string, active bool) (Station, error) {
	name, printer, categories, err := normalize(name, printer, categories)
	if err != nil {
		return Station{}, err
	}
	flag := 0
	if active {
		flag = 1
	}
	if _, err := s.db.SQL().Exec(
		"UPDATE t_pos_kitchen_station SET name=?, printer=?, categories=?, active=? WHERE id=?",
		name, printer, categories, flag, id); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Station{}, ErrConflict
		}
		return Station{}, err
	}
	return s.Get(id)
}

// Get 按 ID 查询。
func (s *Service) Get(id int64) (Station, error) {
	var st Station
	var active int
	err := s.db.SQL().QueryRow(
		"SELECT id,name,printer,categories,active FROM t_pos_kitchen_station WHERE id=?", id).
		Scan(&st.ID, &st.Name, &st.Printer, &st.Categories, &active)
	if err != nil {
		return Station{}, ErrNotFound
	}
	st.Active = active == 1
	return st, nil
}

// Delete 删除档口。
func (s *Service) Delete(id int64) error {
	_, err := s.db.SQL().Exec("DELETE FROM t_pos_kitchen_station WHERE id=?", id)
	return err
}

// Match 判断某个类目是否该出到本档口（空类目列表 = 全收）。
func (st Station) Match(category string) bool {
	raw := strings.TrimSpace(st.Categories)
	if raw == "" {
		return true
	}
	for _, c := range strings.Split(raw, ",") {
		if strings.TrimSpace(c) == category {
			return true
		}
	}
	return false
}

func normalize(name, printer, categories string) (string, string, string, error) {
	name = strings.TrimSpace(name)
	printer = strings.TrimSpace(printer)
	if name == "" || len([]rune(name)) > 20 {
		return "", "", "", ErrBadRequest
	}
	if printer == "" || len(printer) > 60 {
		return "", "", "", ErrBadRequest
	}
	// 网口打印机用 tcp://host:port，其它按 Windows 队列名
	if print.IsTCP(printer) {
		if _, err := print.ParseTCPAddr(printer); err != nil {
			return "", "", "", ErrBadPrinter
		}
	}
	cats := make([]string, 0, 4)
	for _, c := range strings.Split(categories, ",") {
		if c = strings.TrimSpace(c); c != "" {
			cats = append(cats, c)
		}
	}
	return name, printer, strings.Join(cats, ","), nil
}
