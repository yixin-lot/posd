// 员工管理（设置页）：CRUD + 上下架 + 删除保护。
//
// 表结构沿用 V5__staff.sql（t_pos_staff），不改 schema（红线）。
// 新建/改 PIN 一律 PBKDF2 哈希落库，明文不落库、不回传。
package staff

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrNotFound 员工不存在。
	ErrNotFound = errors.New("员工不存在")
	// ErrConflict 用户名冲突 / 删除保护命中。
	ErrConflict = errors.New("员工状态冲突")
	// ErrBadRequest 参数不合法。
	ErrBadRequest = errors.New("参数不合法")
)

// SaveInput 是新建/编辑员工的入参。Pin 传明文，内部哈希后落库；为空表示不改 PIN。
type SaveInput struct {
	Name     string
	Username string
	Pin      string
	Role     string
}

const staffColumns = "id, name, username, role, active, created_at"

// List 查询员工（全部；不返回 PIN 相关列）。
func (s *Service) List() ([]StaffRecord, error) {
	rows, err := s.db.SQL().Query(
		"SELECT "+staffColumns+" FROM t_pos_staff WHERE shop_id=1 ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StaffRecord
	for rows.Next() {
		var r StaffRecord
		var active int
		if err := rows.Scan(&r.ID, &r.Name, &r.Username, &r.Role, &active, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Active = active == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

// Get 按 ID 查询。
func (s *Service) Get(id int64) (StaffRecord, error) {
	var r StaffRecord
	var active int
	err := s.db.SQL().QueryRow(
		"SELECT "+staffColumns+" FROM t_pos_staff WHERE id=?", id).
		Scan(&r.ID, &r.Name, &r.Username, &r.Role, &active, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return StaffRecord{}, ErrNotFound
	}
	if err != nil {
		return StaffRecord{}, err
	}
	r.Active = active == 1
	return r, nil
}

// Create 新建员工：必填 name/username/pin/role，用户名唯一。
func (s *Service) Create(in SaveInput) (StaffRecord, error) {
	r, err := normalizeSave(in, true)
	if err != nil {
		return StaffRecord{}, err
	}
	salt := newSalt()
	hash := pbkdf2([]byte(in.Pin), salt, iterations, keyLen)
	res, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_staff(shop_id,name,username,pin_salt,pin_hash,role,active,created_at)"+
			" VALUES(1,?,?,?,?,?,1,datetime('now','localtime'))",
		r.Name, r.Username, hex.EncodeToString(salt), hex.EncodeToString(hash), r.Role)
	if err != nil {
		if isUnique(err) {
			return StaffRecord{}, fmt.Errorf("%w: 用户名已存在", ErrConflict)
		}
		return StaffRecord{}, err
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

// Update 编辑员工：可改名/改角色/重置 PIN。admin（默认店长）角色不可降级。
func (s *Service) Update(id int64, in SaveInput) (StaffRecord, error) {
	old, err := s.Get(id)
	if err != nil {
		return StaffRecord{}, err
	}
	r, err := normalizeSave(in, false)
	if err != nil {
		return StaffRecord{}, err
	}
	// admin 是内置店长：禁止改名/改角色（防锁死），只能改 PIN。
	if old.Username == "admin" {
		if r.Username != "admin" || r.Role != old.Role {
			return StaffRecord{}, fmt.Errorf("%w: 内置店长 admin 不可改名或改角色", ErrConflict)
		}
	}
	if in.Pin != "" {
		salt := newSalt()
		hash := pbkdf2([]byte(in.Pin), salt, iterations, keyLen)
		if _, err := s.db.SQL().Exec(
			"UPDATE t_pos_staff SET name=?,username=?,role=?,pin_salt=?,pin_hash=? WHERE id=?",
			r.Name, r.Username, r.Role, hex.EncodeToString(salt), hex.EncodeToString(hash), id); err != nil {
			if isUnique(err) {
				return StaffRecord{}, fmt.Errorf("%w: 用户名已存在", ErrConflict)
			}
			return StaffRecord{}, err
		}
	} else {
		if _, err := s.db.SQL().Exec(
			"UPDATE t_pos_staff SET name=?,username=?,role=? WHERE id=?",
			r.Name, r.Username, r.Role, id); err != nil {
			if isUnique(err) {
				return StaffRecord{}, fmt.Errorf("%w: 用户名已存在", ErrConflict)
			}
			return StaffRecord{}, err
		}
	}
	return s.Get(id)
}

// SetActive 上/下架员工。
func (s *Service) SetActive(id int64, active bool) (StaffRecord, error) {
	old, err := s.Get(id)
	if err != nil {
		return StaffRecord{}, err
	}
	if old.Username == "admin" && !active {
		return StaffRecord{}, fmt.Errorf("%w: 内置店长 admin 不可下架", ErrConflict)
	}
	if _, err := s.db.SQL().Exec(
		"UPDATE t_pos_staff SET active=? WHERE id=?", boolInt(active), id); err != nil {
		return StaffRecord{}, err
	}
	return s.Get(id)
}

// Delete 删除员工：admin 不可删；有会话记录（登录过）只能下架。
func (s *Service) Delete(id int64) error {
	old, err := s.Get(id)
	if err != nil {
		return err
	}
	if old.Username == "admin" {
		return fmt.Errorf("%w: 内置店长 admin 不可删除", ErrConflict)
	}
	var sessions int
	if err := s.db.SQL().QueryRow(
		"SELECT COUNT(*) FROM t_pos_session WHERE staff_id=?", id).Scan(&sessions); err != nil {
		return err
	}
	if sessions > 0 {
		return fmt.Errorf("%w: 该员工登录过系统，只能下架，不能删除", ErrConflict)
	}
	_, err = s.db.SQL().Exec("DELETE FROM t_pos_staff WHERE id=?", id)
	return err
}

// StaffRecord 是前端可见的员工记录（不含 PIN 相关列）。
type StaffRecord struct {
	ID        int64
	Name      string
	Username  string
	Role      string
	Active    bool
	CreatedAt string
}

func isUnique(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func normalizeSave(in SaveInput, requirePin bool) (StaffRecord, error) {
	name := strings.TrimSpace(in.Name)
	username := strings.TrimSpace(in.Username)
	role := strings.TrimSpace(in.Role)
	if name == "" {
		return StaffRecord{}, fmt.Errorf("%w: 姓名不能为空", ErrBadRequest)
	}
	if username == "" {
		return StaffRecord{}, fmt.Errorf("%w: 账号不能为空", ErrBadRequest)
	}
	if role != "manager" && role != "cashier" {
		return StaffRecord{}, fmt.Errorf("%w: 角色只能是店长或收银员", ErrBadRequest)
	}
	if requirePin && strings.TrimSpace(in.Pin) == "" {
		return StaffRecord{}, fmt.Errorf("%w: 初始 PIN 不能为空", ErrBadRequest)
	}
	if p := strings.TrimSpace(in.Pin); p != "" && len(p) < 4 {
		return StaffRecord{}, fmt.Errorf("%w: PIN 至少 4 位", ErrBadRequest)
	}
	return StaffRecord{Name: name, Username: username, Role: role}, nil
}
