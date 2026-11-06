// 员工管理 CRUD 单元测试（设置页）。
package staff

import (
	"errors"
	"testing"
)

func TestCreateStaff(t *testing.T) {
	svc := newService(t)
	st, err := svc.Create(SaveInput{Name: "张三", Username: "zhangsan", Pin: "8888", Role: "cashier"})
	if err != nil {
		t.Fatalf("新建失败: %v", err)
	}
	if st.Username != "zhangsan" || st.Role != "cashier" || !st.Active {
		t.Fatalf("记录不正确: %+v", st)
	}
	// 新建的员工应能登录
	token, _, err := svc.Login("zhangsan", "8888")
	if err != nil {
		t.Fatalf("新建员工登录失败: %v", err)
	}
	if st, ok := svc.Verify(token); !ok || st.Username != "zhangsan" {
		t.Fatalf("令牌应有效: %+v %v", st, ok)
	}
}

func TestCreateRejectsDuplicateUsername(t *testing.T) {
	svc := newService(t)
	_, err := svc.Create(SaveInput{Name: "张三", Username: "zhangsan", Pin: "8888", Role: "cashier"})
	if err != nil {
		t.Fatalf("首次新建应成功: %v", err)
	}
	_, err = svc.Create(SaveInput{Name: "李四", Username: "zhangsan", Pin: "8888", Role: "cashier"})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("重名应返回 ErrConflict，实际 %v", err)
	}
}

func TestCreateRequiresPinAndValidRole(t *testing.T) {
	svc := newService(t)
	if _, err := svc.Create(SaveInput{Name: "无 PIN", Username: "x1", Role: "cashier"}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("缺 PIN 应 ErrBadRequest，实际 %v", err)
	}
	if _, err := svc.Create(SaveInput{Name: "坏角色", Username: "x2", Pin: "1234", Role: "boss"}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("非法角色应 ErrBadRequest，实际 %v", err)
	}
}

func TestUpdateStaffResetsPinAndRole(t *testing.T) {
	svc := newService(t)
	st, _ := svc.Create(SaveInput{Name: "张三", Username: "zhangsan", Pin: "8888", Role: "cashier"})

	// 改角色 + 重设 PIN
	upd, err := svc.Update(st.ID, SaveInput{Name: "张三", Username: "zhangsan", Pin: "9999", Role: "manager"})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if upd.Role != "manager" {
		t.Fatalf("角色未更新: %s", upd.Role)
	}
	if _, _, err := svc.Login("zhangsan", "9999"); err != nil {
		t.Fatalf("新 PIN 应可登录: %v", err)
	}
	if _, _, err := svc.Login("zhangsan", "8888"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("旧 PIN 应失效，实际 %v", err)
	}
}

func TestAdminCannotBeRenamedOrDemoted(t *testing.T) {
	svc := newService(t)
	admin, err := svc.Get(1)
	if err != nil {
		t.Fatalf("取 admin 失败: %v", err)
	}
	if _, err := svc.Update(admin.ID, SaveInput{Name: "店长", Username: "admin", Role: "cashier"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("降级 admin 应 ErrConflict，实际 %v", err)
	}
	if _, err := svc.Update(admin.ID, SaveInput{Name: "店长", Username: "boss", Role: "manager"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("改名 admin 应 ErrConflict，实际 %v", err)
	}
	// admin 改 PIN 应允许
	if _, err := svc.Update(admin.ID, SaveInput{Name: "店长", Username: "admin", Pin: "5678", Role: "manager"}); err != nil {
		t.Fatalf("admin 改 PIN 应允许: %v", err)
	}
	if _, _, err := svc.Login("admin", "5678"); err != nil {
		t.Fatalf("新 PIN 应可登录: %v", err)
	}
}

func TestSetActiveAndLoginGate(t *testing.T) {
	svc := newService(t)
	st, _ := svc.Create(SaveInput{Name: "张三", Username: "zhangsan", Pin: "8888", Role: "cashier"})

	// 下架后不能登录
	if _, err := svc.SetActive(st.ID, false); err != nil {
		t.Fatalf("下架失败: %v", err)
	}
	if _, _, err := svc.Login("zhangsan", "8888"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("下架后登录应拒绝，实际 %v", err)
	}
	// 上架后可登录
	if _, err := svc.SetActive(st.ID, true); err != nil {
		t.Fatalf("上架失败: %v", err)
	}
	if _, _, err := svc.Login("zhangsan", "8888"); err != nil {
		t.Fatalf("上架后登录应成功: %v", err)
	}
	// admin 不可下架
	admin, _ := svc.Get(1)
	if _, err := svc.SetActive(admin.ID, false); !errors.Is(err, ErrConflict) {
		t.Fatalf("下架 admin 应 ErrConflict，实际 %v", err)
	}
}

func TestDeleteProtection(t *testing.T) {
	svc := newService(t)
	// 登录过（有 session）的员工不可删
	st, _ := svc.Create(SaveInput{Name: "张三", Username: "zhangsan", Pin: "8888", Role: "cashier"})
	if _, _, err := svc.Login("zhangsan", "8888"); err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if err := svc.Delete(st.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("有 session 应 ErrConflict，实际 %v", err)
	}
	// 从未登录的新员工可删
	st2, _ := svc.Create(SaveInput{Name: "李四", Username: "lisi", Pin: "7777", Role: "cashier"})
	if err := svc.Delete(st2.ID); err != nil {
		t.Fatalf("未登录员工应可删: %v", err)
	}
	// admin 不可删
	admin, _ := svc.Get(1)
	if err := svc.Delete(admin.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("删除 admin 应 ErrConflict，实际 %v", err)
	}
}
