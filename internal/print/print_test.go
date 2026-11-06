package print

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"

	"pos/internal/escpos"
	"pos/internal/store"
)

type failPrinter struct{}

func (failPrinter) Name() string       { return "fail" }
func (failPrinter) Write([]byte) error { return errors.New("printer offline") }

func newDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatalf("打开库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedOrder(t *testing.T, db *store.DB, orderNo string) {
	t.Helper()
	if _, err := db.SQL().Exec(
		"INSERT INTO t_pos_order(shop_id,order_no,biz_date,status,total_amount,discount_amount,payable_amount,"+
			"cash_amount,change_amount,print_status,created_at) VALUES(1,?,'20261102','paid',1300,0,1300,2000,700,0,"+
			"datetime('now'))", orderNo); err != nil {
		t.Fatalf("造订单失败: %v", err)
	}
}

func ticket(orderID int64, orderNo string) Ticket {
	return Ticket{
		OrderID: orderID, OrderNo: orderNo, CreatedAt: "2026-11-02 13:00:00",
		Items: []escpos.Item{{Name: "珍珠奶茶", Price: "13.00", Qty: 1, Amount: "13.00"}},
		Total: 1300, Cash: 2000, Change: 700,
	}
}

func TestEnqueueAndPrint(t *testing.T) {
	db := newDB(t)
	seedOrder(t, db, "L20261102-0001")
	out := filepath.Join(t.TempDir(), "print.bin")
	svc := NewService(db, &FilePrinter{Path: out}, nil)

	if _, err := svc.Enqueue(ticket(1, "L20261102-0001")); err != nil {
		t.Fatalf("入队失败: %v", err)
	}
	var status int
	_ = db.SQL().QueryRow("SELECT print_status FROM t_pos_order WHERE id=1").Scan(&status)
	if status != 1 {
		t.Fatalf("入队后 print_status 应为 1，实际 %d", status)
	}

	if !svc.step() {
		t.Fatal("应处理到一条任务")
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("读取纸面失败: %v", err)
	}
	// 纸面是 GBK 字节，先解码再断言，避免拿 UTF-8 去比。
	plain, err := simplifiedchinese.GBK.NewDecoder().Bytes(body)
	if err != nil || !bytes.Contains(plain, []byte("珍珠")) {
		t.Fatalf("纸面内容不对: err=%v len=%d", err, len(body))
	}
	_ = db.SQL().QueryRow("SELECT print_status FROM t_pos_order WHERE id=1").Scan(&status)
	if status != 2 {
		t.Fatalf("出纸后 print_status 应为 2，实际 %d", status)
	}
}

func TestEnqueueReport(t *testing.T) {
	db := newDB(t)
	out := filepath.Join(t.TempDir(), "print.bin")
	svc := NewService(db, &FilePrinter{Path: out}, nil)

	if _, err := svc.EnqueueReport(escpos.Shift{
		ShiftNo: "S20261102-001", StaffName: "陈晓",
		OpenedAt: "2026-11-02 07:00:00", ClosedAt: "2026-11-02 14:00:00",
		OpeningCash: "200.00", OrderCount: 1, Total: "13.00",
		Expected: "213.00", Actual: "213.00", Difference: "0.00",
		Payments: []escpos.Payment{{Method: "cash", Amount: "13.00"}},
	}); err != nil {
		t.Fatalf("交接单入队失败: %v", err)
	}
	if !svc.step() {
		t.Fatal("应处理到交接单任务")
	}
	body, _ := os.ReadFile(out)
	plain, _ := simplifiedchinese.GBK.NewDecoder().Bytes(body)
	if !bytes.Contains(plain, []byte("交接班单")) {
		t.Fatalf("纸面不是交接单: %q", string(plain))
	}
}

func TestFailureBecomesRetryThenDead(t *testing.T) {
	db := newDB(t)
	seedOrder(t, db, "L20261102-0002")
	svc := NewService(db, failPrinter{}, nil)
	if _, err := svc.Enqueue(ticket(1, "L20261102-0002")); err != nil {
		t.Fatalf("入队失败: %v", err)
	}
	for i := 0; i < 3; i++ {
		svc.step()
		// 让退避时间过期，模拟"过了一会儿"
		_, _ = db.SQL().Exec("UPDATE t_pos_print_job SET next_try_at=0")
	}
	var jobStatus string
	_ = db.SQL().QueryRow("SELECT status FROM t_pos_print_job").Scan(&jobStatus)
	if jobStatus != "dead" {
		t.Fatalf("三次失败后应为 dead，实际 %s", jobStatus)
	}
	var orderStatus int
	_ = db.SQL().QueryRow("SELECT print_status FROM t_pos_order WHERE id=1").Scan(&orderStatus)
	if orderStatus != 3 {
		t.Fatalf("订单应转待重打(3)，实际 %d", orderStatus)
	}
}
