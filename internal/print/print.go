// Package print 是出票编排（P5）：渲染 → 入队 → 落盘出纸 → 回写状态。
//
// 单机版把出票放在进程内，不再单独跑 cpagent（ADR-005）。
// 传输层是接口：Windows 走 spooler（下一版接），测试走文件。
package print

import (
	"image"
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"pos/internal/escpos"
	"pos/internal/store"
)

// Printer 是出纸通道。
type Printer interface {
	Name() string
	Write(payload []byte) error
}

// FilePrinter 把字节写到文件（开发/测试用，也可用于"只存不出纸"的门店）。
type FilePrinter struct {
	Path string
}

// Name 返回通道名。
func (f *FilePrinter) Name() string { return "file:" + f.Path }

// Write 追加写入。
func (f *FilePrinter) Write(payload []byte) error {
	fh, err := os.OpenFile(f.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer fh.Close()
	_, err = fh.Write(payload)
	return err
}

// Ticket 是渲染一张小票需要的数据（金额单位：分）。
type Ticket struct {
	OrderID   int64
	OrderNo   string
	CreatedAt string
	Table     string
	Shop      escpos.Shop
	Items     []escpos.Item
	Total     int64
	Discount  int64
	Cash      int64
	Change    int64
	RoundOff  int64
	MemberDiscount int64
	PointsEarned  int64
	Notice        string
	Footer        string
	QRText        string
	Logo          image.Image
	Payments  []escpos.Pay
}

// Service 管理打印任务。
type Service struct {
	db      *store.DB
	printer Printer
	// printerFor 按名字取打印机（后厨联打用）；为空则一律用默认打印机。
	printerFor func(name string) Printer
	log     *slog.Logger
	maxTry  int
}

// NewService 创建打印服务。
func NewService(db *store.DB, printer Printer, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{db: db, printer: printer, log: log, maxTry: 3}
}

// renderTicket 是订单小票的**唯一**渲染入口。
//
// 之前 Enqueue 和 Reprint 各拼一份 escpos.Order，Reprint 那份少拼了
// 桌台、支付方式、会员折扣、积分、抹零 —— 重打出来的小票就丢行。
// 现在只有这里能改，两条路径必然一致。
func renderTicket(t Ticket) []byte {
	return escpos.RenderOrder(t.Shop, escpos.Order{
		OrderNo:        t.OrderNo,
		CreatedAt:      t.CreatedAt,
		Table:          t.Table,
		Items:          t.Items,
		Total:          money(t.Total),
		Discount:       money(t.Discount),
		Cash:           money(t.Cash),
		Change:         money(t.Change),
		RoundOff:       money(t.RoundOff),
		MemberDiscount: money(t.MemberDiscount),
		Points:         fmt.Sprintf("%d", t.PointsEarned),
		Notice:         t.Notice,
		Footer:         t.Footer,
		QRText:         t.QRText,
		Logo:           t.Logo,
		Payments:       t.Payments,
	})
}

// Enqueue 渲染并入队，同时把订单标成「已入队」(print_status=1)。
func (s *Service) Enqueue(t Ticket) (int64, error) {
	payload := renderTicket(t)
	res, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_print_job(order_id,order_no,target,payload,idem_key,status,created_at)"+
			" VALUES(?,?,'front',?,?,'queued',?)",
		t.OrderID, t.OrderNo, payload, fmt.Sprintf("%s:front:1", t.OrderNo), time.Now().UnixMilli())
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if _, err := s.db.SQL().Exec("UPDATE t_pos_order SET print_status=1 WHERE id=?", t.OrderID); err != nil {
		return 0, err
	}
	return id, nil
}

// Reprint 重打：新开一条任务（幂等键带序号），避免覆盖历史。
func (s *Service) Reprint(t Ticket) (int64, error) {
	var n int
	if err := s.db.SQL().QueryRow("SELECT COUNT(*) FROM t_pos_print_job WHERE order_no=?", t.OrderNo).Scan(&n); err != nil {
		return 0, err
	}
	payload := renderTicket(t)
	res, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_print_job(order_id,order_no,target,payload,idem_key,status,created_at)"+
			" VALUES(?,?,'front',?,?,'queued',?)",
		t.OrderID, t.OrderNo, payload, fmt.Sprintf("%s:front:%d", t.OrderNo, n+1), time.Now().UnixMilli())
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if _, err := s.db.SQL().Exec("UPDATE t_pos_order SET print_status=1 WHERE id=?", t.OrderID); err != nil {
		return 0, err
	}
	return id, nil
}

// PrintLabels 直接打印价签（不入队：价签是即时操作，失败重试意义不大）。
func (s *Service) PrintLabels(labels []escpos.Label) (int, error) {
	n := 0
	for _, l := range labels {
		if err := s.printer.Write(escpos.RenderLabel(l)); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// SetPrinterFactory 注入"按名字取打印机"（后厨联打）。
func (s *Service) SetPrinterFactory(fn func(name string) Printer) { s.printerFor = fn }

// EnqueueKitchen 把后厨单送入队列并指定打印机。
func (s *Service) EnqueueKitchen(orderNo, printer string, payload []byte, idem string) (int64, error) {
	res, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_print_job(order_id,order_no,target,printer,payload,idem_key,status,created_at)"+
			" VALUES(0,?,'kitchen',?,?,?,'queued',?)",
		orderNo, printer, payload, idem, time.Now().UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// EnqueueReport 把交接单送入打印队列（order_id 记 0，表示与订单无关）。
func (s *Service) EnqueueReport(sh escpos.Shift) (int64, error) {
	payload := escpos.RenderShift(sh)
	res, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_print_job(order_id,order_no,target,payload,idem_key,status,created_at)"+
			" VALUES(0,?,'report',?,?,'queued',?)",
		sh.ShiftNo, payload, fmt.Sprintf("%s:report:1", sh.ShiftNo), time.Now().UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Run 消费打印队列，直到 ctx 结束。
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		for s.step() {
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// step 处理一条任务；返回 false 表示队列为空。
func (s *Service) step() bool {
	now := time.Now().UnixMilli()
	var (
		id      int64
		orderID int64
		payload []byte
		printer string
	)
	err := s.db.SQL().QueryRow(
		"SELECT id,order_id,payload,printer FROM t_pos_print_job"+
			" WHERE status='queued' AND next_try_at<=? ORDER BY id LIMIT 1",
		now).Scan(&id, &orderID, &payload, &printer)
	if err != nil {
		return false
	}
	// 先认领：并发/重启下只有一方能拿到。
	res, err := s.db.SQL().Exec(
		"UPDATE t_pos_print_job SET status='printing', next_try_at=? WHERE id=? AND status='queued'",
		time.Now().Add(60*time.Second).UnixMilli(), id)
	if err != nil {
		return false
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return true
	}

	dev := s.printer
	if printer != "" && s.printerFor != nil {
		if alt := s.printerFor(printer); alt != nil {
			dev = alt
		}
	}
	if err := dev.Write(payload); err != nil {
		s.fail(id, orderID, err)
	} else {
		_, _ = s.db.SQL().Exec("UPDATE t_pos_print_job SET status='done', printed_at=?, error='' WHERE id=?",
			time.Now().UnixMilli(), id)
		_, _ = s.db.SQL().Exec("UPDATE t_pos_order SET print_status=2 WHERE id=?", orderID)
	}
	return true
}

func (s *Service) fail(id, orderID int64, cause error) {
	var attempts int
	if err := s.db.SQL().QueryRow("SELECT attempts FROM t_pos_print_job WHERE id=?", id).Scan(&attempts); err != nil {
		return
	}
	attempts++
	msg := cause.Error()
	if attempts >= s.maxTry {
		_, _ = s.db.SQL().Exec(
			"UPDATE t_pos_print_job SET status='dead', attempts=?, error=? WHERE id=?", attempts, msg, id)
		_, _ = s.db.SQL().Exec("UPDATE t_pos_order SET print_status=3 WHERE id=?", orderID)
		s.log.Warn("出票失败，已转待重打", "jobId", id, "err", msg)
		return
	}
	_, _ = s.db.SQL().Exec(
		"UPDATE t_pos_print_job SET status='queued', attempts=?, error=?, next_try_at=? WHERE id=?",
		attempts, msg, time.Now().Add(time.Duration(attempts)*3*time.Second).UnixMilli(), id)
	s.log.Warn("出票失败，稍后重试", "jobId", id, "attempts", attempts, "err", msg)
}

// Job 是打印队列的一条任务（供排障展示，不含 payload）。
type Job struct {
	ID        int64  `json:"id"`
	OrderNo   string `json:"orderNo"`
	Target    string `json:"target"`
	Printer   string `json:"printer"`
	Status    string `json:"status"`
	Attempts  int    `json:"attempts"`
	Error     string `json:"error"`
	CreatedAt string `json:"createdAt"`
	PrintedAt string `json:"printedAt,omitempty"`
}

// Jobs 返回最近的打印任务（默认 20 条，最多 100）。
// 只读、不含 payload——排障要的是"哪张票、去哪台、失败几次、什么错"。
func (s *Service) Jobs(limit int) ([]Job, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.SQL().Query(
		"SELECT id, order_no, target, COALESCE(printer,''), status, attempts, COALESCE(error,''),"+
			" datetime(created_at/1000,'unixepoch','localtime'),"+
			" COALESCE(datetime(printed_at/1000,'unixepoch','localtime'),'')"+
			" FROM t_pos_print_job ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.OrderNo, &j.Target, &j.Printer, &j.Status, &j.Attempts,
			&j.Error, &j.CreatedAt, &j.PrintedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func money(cents int64) string { return fmt.Sprintf("%.2f", float64(cents)/100) }

// EnqueueVoid 把作废小票送入打印队列（与订单无关的 target='void'）。
func (s *Service) EnqueueVoid(v escpos.Void) (int64, error) {
	payload := escpos.RenderVoid(escpos.Shop{}, v)
	res, err := s.db.SQL().Exec(
		"INSERT INTO t_pos_print_job(order_id,order_no,target,payload,idem_key,status,created_at)"+
			" VALUES(0,?,'void',?,?,'queued',?)",
		v.OrderNo, payload, fmt.Sprintf("%s:void:%d", v.OrderNo, time.Now().UnixMilli()), time.Now().UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
