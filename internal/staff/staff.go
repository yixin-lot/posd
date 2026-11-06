// Package staff 是操作员与登录（P3）：PBKDF2 存 PIN、发会话令牌。
//
// 与 Java 版算法一致（PBKDF2-HMAC-SHA256，120000 次，16 字节盐），
// 但 Go 侧手写实现，避免为了一个函数引入 x/crypto 依赖。
package staff

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"pos/internal/store"
)

// ErrUnauthorized 账号或 PIN 不正确。
var ErrUnauthorized = errors.New("账号或 PIN 不正确")

// ErrLocked 登录失败次数过多，账号临时锁定。
var ErrLocked = errors.New("登录失败次数过多，已临时锁定")

// LockedError 携带剩余锁定时间（秒）。
type LockedError struct {
	Remaining int // 秒
}

func (e *LockedError) Error() string {
	return fmt.Sprintf("登录失败次数过多，请在 %d 秒后重试", e.Remaining)
}

const (
	iterations    = 120_000
	keyLen        = 32
	sessionTTL    = 12 * time.Hour
	maxLoginFails = 5
	lockDuration  = 5 * time.Minute
)

// loginGuard 只用于「库里不存在的用户名」——没有行可写，只能用内存兜底。
// 真实账号的失败计数与锁定已落库（t_pos_staff.fail_count / locked_until）。
type loginGuard struct {
	mu    sync.Mutex
	fails map[string]*failState
}

type failState struct {
	count       int
	lockedUntil time.Time
}

func newLoginGuard() *loginGuard { return &loginGuard{fails: map[string]*failState{}} }

// check 返回剩余锁定秒数；0 表示未锁定。
func (g *loginGuard) check(key string, now time.Time) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	st := g.fails[key]
	if st == nil || st.count < maxLoginFails {
		return 0
	}
	if now.Before(st.lockedUntil) {
		return int(st.lockedUntil.Sub(now).Seconds()) + 1
	}
	// 锁定到期：清空计数，允许重试
	delete(g.fails, key)
	return 0
}

func (g *loginGuard) recordFail(key string, now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	st := g.fails[key]
	if st == nil {
		st = &failState{}
		g.fails[key] = st
	}
	st.count++
	if st.count >= maxLoginFails {
		st.lockedUntil = now.Add(lockDuration)
	}
}

func (g *loginGuard) reset(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.fails, key)
}

// Staff 是操作员。
type Staff struct {
	ID       int64
	Name     string
	Username string
	Role     string
}

// Service 提供登录与账号查询。
type Service struct {
	db    *store.DB
	now   func() time.Time
	guard *loginGuard
}

// NewService 创建服务，并在库为空时建默认店长。
func NewService(db *store.DB) (*Service, error) {
	s := &Service{db: db, now: time.Now, guard: newLoginGuard()}
	if err := s.seedDefaultManager(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Service) seedDefaultManager() error {
	var n int
	if err := s.db.SQL().QueryRow("SELECT COUNT(*) FROM t_pos_staff").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	salt := newSalt()
	hash := pbkdf2([]byte("1234"), salt, iterations, keyLen)
	_, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_staff(shop_id,name,username,pin_salt,pin_hash,role,active,created_at)"+
			" VALUES(1,'店长','admin',?,?,'manager',1,datetime('now','localtime'))",
		hex.EncodeToString(salt), hex.EncodeToString(hash))
	return err
}

// Login 校验账号与 PIN，成功后写入会话。
//
// 失败锁定落库（t_pos_staff.fail_count / locked_until）：重启不会清零，
// 避免"重启服务就绕过锁定"。未知用户名仍用内存计数（没有行可写）。
func (s *Service) Login(username, pin string) (string, Staff, error) {
	username = strings.TrimSpace(username)
	key := strings.ToLower(username)

	var (
		st          Staff
		saltHex     string
		hashHex     string
		active      int
		failCount   int
		lockedUntil int64
	)
	err := s.db.SQL().QueryRow(
		"SELECT id,name,username,role,pin_salt,pin_hash,active,fail_count,locked_until"+
			" FROM t_pos_staff WHERE username=?",
		username).Scan(&st.ID, &st.Name, &st.Username, &st.Role, &saltHex, &hashHex, &active, &failCount, &lockedUntil)
	if errors.Is(err, sql.ErrNoRows) {
		// 库里没有这个账号：用内存计数防止无限试探（无法落库）
		if remain := s.guard.check(key, s.now()); remain > 0 {
			return "", Staff{}, &LockedError{Remaining: remain}
		}
		s.guard.recordFail(key, s.now())
		return "", Staff{}, ErrUnauthorized
	}
	if err != nil {
		return "", Staff{}, err
	}

	now := s.now()
	if lockedUntil > now.UnixMilli() {
		return "", Staff{}, &LockedError{Remaining: int(lockedUntil-now.UnixMilli())/1000 + 1}
	}

	if active != 1 {
		s.recordFail(st.ID)
		return "", Staff{}, ErrUnauthorized
	}
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return "", Staff{}, err
	}
	want, err := hex.DecodeString(hashHex)
	if err != nil {
		return "", Staff{}, err
	}
	got := pbkdf2([]byte(pin), salt, iterations, keyLen)
	if !hmac.Equal(got, want) {
		s.recordFail(st.ID)
		return "", Staff{}, ErrUnauthorized
	}
	s.resetFail(st.ID)

	token := newToken()
	expires := now.Add(sessionTTL).UnixMilli()
	if _, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_session(token,staff_id,expires_at,created_at) VALUES(?,?,?,datetime('now','localtime'))",
		token, st.ID, expires); err != nil {
		return "", Staff{}, err
	}
	return token, st, nil
}

// recordFail 记一次失败；达到阈值就锁定并清零计数（避免锁定到期后立刻又锁）。
func (s *Service) recordFail(staffID int64) {
	_, _ = s.db.SQL().Exec("UPDATE t_pos_staff SET fail_count=fail_count+1 WHERE id=?", staffID)
	var n int
	if err := s.db.SQL().QueryRow("SELECT fail_count FROM t_pos_staff WHERE id=?", staffID).Scan(&n); err != nil {
		return
	}
	if n >= maxLoginFails {
		until := s.now().Add(lockDuration).UnixMilli()
		_, _ = s.db.SQL().Exec(
			"UPDATE t_pos_staff SET fail_count=0, locked_until=? WHERE id=?", until, staffID)
	}
}

func (s *Service) resetFail(staffID int64) {
	_, _ = s.db.SQL().Exec(
		"UPDATE t_pos_staff SET fail_count=0, locked_until=0 WHERE id=?", staffID)
}

// Verify 校验令牌，返回操作员。
// Logout 让 token 立即失效：服务端删会话，不只是前端丢 token。
func (s *Service) Logout(token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.SQL().Exec("DELETE FROM t_pos_session WHERE token=?", token)
	return err
}

func (s *Service) Verify(token string) (Staff, bool) {
	if strings.TrimSpace(token) == "" {
		return Staff{}, false
	}
	var st Staff
	err := s.db.SQL().QueryRow(
		"SELECT s.id,s.name,s.username,s.role FROM t_pos_session t JOIN t_pos_staff s ON s.id=t.staff_id"+
			" WHERE t.token=? AND t.expires_at>? AND s.active=1",
		token, s.now().UnixMilli()).Scan(&st.ID, &st.Name, &st.Username, &st.Role)
	if err != nil {
		return Staff{}, false
	}
	return st, true
}

// Count 返回员工数（迁移期自检用）。
func (s *Service) Count() (int, error) {
	var n int
	if err := s.db.SQL().QueryRow("SELECT COUNT(*) FROM t_pos_staff").Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func newSalt() []byte {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("生成盐失败: %v", err))
	}
	return b
}

func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("生成令牌失败: %v", err))
	}
	return hex.EncodeToString(b)
}

// pbkdf2 是 RFC 2898 的最小实现（只需要 SHA-256 + 单块长度）。
func pbkdf2(password, salt []byte, iter, length int) []byte {
	var out []byte
	var idx [4]byte
	for block := uint32(1); len(out) < length; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		binary.BigEndian.PutUint32(idx[:], block)
		mac.Write(idx[:])
		u := mac.Sum(nil)
		t := make([]byte, len(u))
		copy(t, u)
		for i := 1; i < iter; i++ {
			mac.Reset()
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:length]
}
