// posd 是单机版收银服务：一个二进制内含 HTTP 服务、SQLite、前端与（后续）出票。
//
// 用法：
//
//	posd --addr 127.0.0.1:8080 --db ./data/pos.db
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"image"
	"image/png"
	"io"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"pos/internal/backup"
	"pos/internal/catalog"
	"pos/internal/category"
	"pos/internal/kitchen"
	"pos/internal/member"
	"pos/internal/escpos"
	"pos/internal/order"
	"pos/internal/print"
	"pos/internal/report"
	"pos/internal/purchase"
	"pos/internal/setting"
	"pos/internal/stock"
	"pos/internal/stocktake"
	"pos/internal/table"
	"pos/internal/templates"
	"pos/internal/shift"
	"pos/internal/staff"
	"pos/internal/store"
	"pos/internal/webui"
)

// 用 var 而不是 const：打包脚本用 -ldflags -X main.version= 注入版本号
var version = "0.1.0-go"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("posd", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "监听地址")
	dbPath := fs.String("db", filepath.Join("data", "pos.db"), "SQLite 路径")
	printerName := fs.String("printer", "", "打印机名称（留空则写入 data/print.bin，便于无纸调试）")
	backupKeep := fs.Int("backup-keep", 7, "保留最近几份数据库快照")
	noBackup := fs.Bool("no-backup", false, "关闭自动备份")
	showVersion := fs.Bool("version", false, "打印版本")
	logPath := fs.String("log", "", "日志文件（桌面版默认 data/posd.log）")
	noOpen := fs.Bool("no-open", false, "启动后不自动打开浏览器（桌面版）")
	lan := fs.Bool("lan", false, "监听 0.0.0.0，允许同一 Wi-Fi 下的手机访问（默认仅本机）")
	noTray := fs.Bool("no-tray", false, "不创建系统托盘图标（桌面版）")
	localeFlag := fs.String("locale", "", "界面语言（zh-CN / en-US），国际版安装包用 en-US")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Printf("posd %s\n", version)
		return nil
	}

	if *lan && (*addr == "127.0.0.1:8080" || *addr == "") {
		*addr = "0.0.0.0:8080"
	}

	if dir := filepath.Dir(*dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("创建数据目录失败: %w", err)
		}
	}
	db, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	logOut := io.Writer(os.Stderr)
	if *logPath == "" && !hasConsole() {
		*logPath = filepath.Join(filepath.Dir(*dbPath), "posd.log")
	}
	if *logPath != "" {
		f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			defer f.Close()
			// 桌面版没有控制台：stderr 写入会失败，导致 MultiWriter 提前返回、文件也写不进去
			if hasConsole() {
				logOut = io.MultiWriter(os.Stderr, f)
			} else {
				logOut = f
			}
		}
	}
	log := slog.New(slog.NewTextHandler(logOut, nil))
	mux := http.NewServeMux()
	// 店名要显示在状态条上，从设置取（health 每几秒轮询，一次单表查询无所谓）。
	settingSvc := setting.NewService(db)
	if *localeFlag != "" {
		if err := settingSvc.Set(setting.KeyLocale, *localeFlag); err != nil {
			log.Warn("写入语言设置失败", "locale", *localeFlag, "err", err)
		}
		// 国际版安装包带 --locale en-US：首次启动把货币/符号/时区也切到美元，已有选择不覆盖
		if strings.HasPrefix(strings.ToLower(*localeFlag), "en") {
			if cur, _ := settingSvc.Get(setting.KeyCurrency, setting.DefaultCurrency); cur == setting.DefaultCurrency {
				_ = settingSvc.Set(setting.KeyCurrency, "USD")
				_ = settingSvc.Set(setting.KeyCurrencySymbol, "$")
				_ = settingSvc.Set(setting.KeyTimezone, "America/New_York")
			}
		}
	}
	// 状态条左段要显示「班次」，同样从当前开班班次取。
	shiftSvc := shift.NewService(db)
	mux.HandleFunc("GET /pos/v1/health", func(w http.ResponseWriter, r *http.Request) {		shopName, _, _, _ := settingSvc.Shop()
		products, _ := db.CountProducts()
		// v03 状态条右端「今日已收」按营业日口径，与订单落库的 biz_date 一致。
		paidCnt, salesToday, _ := db.TodayPaid(order.BizDate(time.Now()))
		// 顶栏班次：只报当前营业中的班次，已结班按未开班处理。
		var shiftInfo any
		if sh, ok, err := shiftSvc.Latest(); err == nil && ok && sh.Status == "open" {
			shiftInfo = map[string]string{"shiftNo": sh.ShiftNo, "status": sh.Status}
		}
		// 状态条「打印输出」随模式切文案：本地直连 / 云打印 / 不打印。
		printMode, _ := settingSvc.Get(setting.KeyPrintMode, setting.DefaultPrintMode)
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"mode":    "standalone",
			"version": version,
			"queue":   map[string]int{"queued": 0},
			"ordersToday": paidCnt,
			"salesToday":  salesToday,
			"syncedAt":    time.Now().Format("15:04"),
			"products":    products,
			"shop":        map[string]string{"name": shopName},
			"shift":       shiftInfo,
			// 状态条「打印输出」随模式切文案：本地直连 / 云打印 / 不打印
			"printMode": printMode,			// 单机版出票在进程内，没有外部助手；对契约而言「可达」恒为 true。
			// （打印机是否真的在线，出票失败时会以 print_status=3 体现。）
			"agent": map[string]bool{"reachable": true},
		})
	})
	staffSvc, err := staff.NewService(db)
	if err != nil {
		return err
	}

	// 国际版默认示例数据（ADR-008）：英语环境首次启动时，示例门店/店长用英文名
	if loc, _ := settingSvc.Get(setting.KeyLocale, setting.DefaultLocale); strings.HasPrefix(strings.ToLower(loc), "en") {
		if name, _, _, _ := settingSvc.Shop(); name == "" || name == setting.DefaultShopName {
			_ = settingSvc.Set(setting.KeyShopName, "Old Street Cafe")
		}
	}
	staffSvc.Register(mux)
	productSvc := catalog.NewService(db)
	catalog.Register(mux, productSvc)

	var printer print.Printer
	if *printerName != "" {
		printer = &print.Spooler{Printer: *printerName}
	} else {
		printer = &print.FilePrinter{Path: filepath.Join(filepath.Dir(*dbPath), "print.bin")}
	}
	printSvc := print.NewService(db, printer, log)
	dataDir := filepath.Dir(*dbPath)
	imagesDir := filepath.Join(dataDir, "images")
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		log.Warn("创建图片目录失败", "err", err)
	}
	// 商品图片与店铺 logo 的静态访问
	mux.Handle("GET /pos/v1/images/", http.StripPrefix("/pos/v1/images/",
		http.FileServer(http.Dir(imagesDir))))
	printSvc.SetPrinterFactory(func(name string) print.Printer {
		p, err := print.Resolve(name)
		if err != nil {
			log.Warn("打印机地址不合法，回落默认打印机", "printer", name, "err", err)
			return nil
		}
		return p
	})
	log.Info("出票通道", "name", printer.Name())
	stockSvc := stock.NewService(db)
	registerTemplates(mux, productSvc, staffSvc, settingSvc, log)
	memberH := member.NewHandler(member.NewService(db))
	memberH.SetBonusRuleSource(func() []member.BonusRule {
		raw, _ := settingSvc.Get(setting.KeyTopupBonusRules, setting.DefaultTopupBonus)
		return member.ParseBonusRules(raw)
	})
	memberH.Register(mux)
	purchase.NewHandler(purchase.NewService(db)).Register(mux, func(w http.ResponseWriter, r *http.Request) bool {
		return requireManager(w, r, staffSvc)
	})
	stocktake.NewHandler(stocktake.NewService(db)).Register(mux, func(w http.ResponseWriter, r *http.Request) bool {
		return requireManager(w, r, staffSvc)
	})
	stock.NewHandler(stockSvc).Register(mux, func(w http.ResponseWriter, r *http.Request) bool {
		return requireManager(w, r, staffSvc)
	})
	kitchen.NewHandler(kitchen.NewService(db)).Register(mux, func(w http.ResponseWriter, r *http.Request) bool {
		return requireManager(w, r, staffSvc)
	})
	category.NewHandler(category.NewService(db)).Register(mux, func(w http.ResponseWriter, r *http.Request) bool {
		return requireManager(w, r, staffSvc)
	})
	reportCSV := report.NewHandler(report.NewService(db))
	reportCSV.RegisterCSV(mux, func(w http.ResponseWriter, r *http.Request) bool {
		return requireManager(w, r, staffSvc)
	})
	report.NewHandler(report.NewService(db)).Register(mux, func(w http.ResponseWriter, r *http.Request) bool {
		return requireManager(w, r, staffSvc)
	})

	// 小票抬头来自门店设置（不再写死成某一家店）。
	// crossesDayNotice 跨天结账时在小票上说明（对账要看这个）。
	crossesDayNotice := func(o order.Order) string {
		if o.CrossDayFrom == "" {
			return ""
		}
		return "本单原营业日 " + o.CrossDayFrom + "，今日结账"
	}

	// noticeFor 生成小票底部的提醒（积分快过期了就让顾客知道）。
	noticeFor := func(o order.Order) string {
		if o.MemberID <= 0 {
			return ""
		}
		mem, err := member.NewService(db).Get(o.MemberID)
		if err != nil || mem.Points <= 0 || mem.PointsExpireAt == "" {
			return ""
		}
		exp, err := time.Parse("2006-01-02", mem.PointsExpireAt)
		if err != nil {
			return ""
		}
		if time.Until(exp) > 7*24*time.Hour {
			return ""
		}
		// 30 天内提醒过就不再打扰
		if mem.PointsNotifiedAt != "" {
			if last, err := time.Parse("2006-01-02", mem.PointsNotifiedAt); err == nil {
				if time.Since(last) < 30*24*time.Hour {
					return ""
				}
			}
		}
		return fmt.Sprintf("您有 %d 积分将于 %s 到期，请尽快使用", mem.Points, mem.PointsExpireAt)
	}

	// 小票页脚由设置决定（每次出票时读，改设置后无需重启）
	receiptQROf := func() string {
		v, _ := settingSvc.Get(setting.KeyReceiptQRText, setting.DefaultReceiptQRText)
		return v
	}
	receiptFooterOf := func() string {
		v, _ := settingSvc.Get(setting.KeyReceiptFooter, setting.DefaultReceiptFooter)
		return v
	}
	// 小票模板：logo 与二维码都来自设置，读不到就跳过（不能让出票失败）
	receiptLogoOf := func() image.Image {
		p, _ := settingSvc.Get(setting.KeyReceiptLogo, setting.DefaultReceiptLogo)
		if p == "" {
			return nil
		}
		f, err := os.Open(filepath.Join(dataDir, p))
		if err != nil {
			return nil
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			return nil
		}
		return img
	}
	buildTicket := func(o order.Order) print.Ticket {
		items := make([]escpos.Item, 0, len(o.Items))
		for _, it := range o.Items {
			items = append(items, escpos.Item{
				Name: it.Name, Price: money(it.Price), Qty: it.Qty, Amount: money(it.Amount),
			})
		}
		name, phone, footer, _ := settingSvc.Shop()
		pays := make([]escpos.Pay, 0, len(o.Payments))
		locale, _ := settingSvc.Get(setting.KeyLocale, setting.DefaultLocale)
		symbol, _ := settingSvc.Get(setting.KeyCurrencySymbol, setting.DefaultCurrencySymbol)
		for _, p := range o.Payments {
			pays = append(pays, escpos.Pay{Name: payMethodName(p.Method, locale), Amount: money(p.Amount)})
		}
		return print.Ticket{
			OrderID: o.ID, OrderNo: o.OrderNo, CreatedAt: receiptTime(o.CreatedAt, locale), Table: o.TableName,
			Lang: locale, Currency: symbol,
			Shop:  escpos.Shop{Name: name, Phone: phone, Footer: footer},
			Items: items, Total: o.Total, Discount: o.Discount, Cash: o.Cash, Change: o.Change, RoundOff: o.RoundOff,
			MemberDiscount: o.MemberDiscount, PointsEarned: o.PointsEarned, Payments: pays,
			Notice: crossesDayNotice(o) + noticeFor(o),
			Footer: receiptFooterOf(),
			QRText: receiptQROf(),
			Logo:   receiptLogoOf(),
		}
	}

	orderSvc := order.NewService(db, productSvc)
	orderSvc.SetStock(stockSvc)
	orderSvc.SetMembers(member.NewService(db))
	var enqueueKitchen func(order.Order)
	orderSvc.OnCreated(func(o order.Order) {
		ticket := buildTicket(o)
		// 设置页「打印模式」：none 表示只记账不出票（订单照常落库，print_status 留 0 未打印）
		mode, _ := settingSvc.Get(setting.KeyPrintMode, setting.DefaultPrintMode)
		if mode == "none" {
			log.Info("打印模式为不打印，跳过出票", "orderNo", o.OrderNo)
		} else if _, err := printSvc.Enqueue(ticket); err != nil {
			log.Warn("入打印队列失败", "orderNo", o.OrderNo, "err", err)
		} else if ticket.Notice != "" && o.MemberID > 0 {
			// 提醒已随小票打出，标记一下，30 天内不再重复
			if err := member.NewService(db).MarkPointsNotified(o.MemberID); err != nil {
				log.Warn("标记积分提醒失败", "memberId", o.MemberID, "err", err)
			}
		}
		enqueueKitchen(o)
	})
	orderSvc.OnReprint(func(o order.Order) {
		if _, err := printSvc.Reprint(buildTicket(o)); err != nil {
			log.Warn("重打入队失败", "orderNo", o.OrderNo, "err", err)
		}
	})
	// 权限钩子：角色来自中间件注入的上下文；收银员是否可退款/打折由设置开关决定。
	perm := func(key string) func(*http.Request) bool {
		return func(r *http.Request) bool {
			st, ok := staff.FromContext(r.Context())
			if !ok {
				return false
			}
			if st.Role == "manager" {
				return true
			}
			allow, _ := settingSvc.Bool(key, false)
			return allow
		}
	}
	// 后厨联打：一个档口一台打印机，按类目过滤；没有档口时回落到旧设置。

	// kitchenLines 交给 kitchen 包（套餐按组件分档口），这里只做类型转换
	kitchenLines := func(o order.Order, match func(string) bool) []escpos.KitchenItem {
		lines := kitchen.Lines(o.Items, match)
		out := make([]escpos.KitchenItem, 0, len(lines))
		for _, l := range lines {
			out = append(out, escpos.KitchenItem{Name: l.Name, Qty: l.Qty})
		}
		return out
	}

	enqueueKitchen = func(o order.Order) {
		stations, err := kitchen.NewService(db).List()
		if err != nil {
			return
		}
		type target struct {
			name    string
			printer string
			items   []escpos.KitchenItem
		}
		var targets []target

		if len(stations) == 0 {
			// 兼容老配置：kitchen_printer + kitchen_categories
			kitchenPrinter, _ := settingSvc.Get(setting.KeyKitchenPrinter, "")
			if kitchenPrinter == "" {
				return
			}
			rawCats, _ := settingSvc.Get(setting.KeyKitchenCategories, "")
			want := map[string]bool{}
			for _, c := range strings.Split(rawCats, ",") {
				if c = strings.TrimSpace(c); c != "" {
					want[c] = true
				}
			}
			t := target{name: "", printer: kitchenPrinter}
			t.items = kitchenLines(o, func(cat string) bool {
				return len(want) == 0 || want[cat]
			})
			if len(t.items) > 0 {
				targets = append(targets, t)
			}
		} else {
			for _, st := range stations {
				if !st.Active {
					continue
				}
				t := target{name: st.Name, printer: st.Printer}
				t.items = kitchenLines(o, st.Match)
				if len(t.items) > 0 {
					targets = append(targets, t)
				}
			}
		}

		for _, t := range targets {
			payload := escpos.RenderKitchen(o.OrderNo, o.TableName, t.name, o.CreatedAt, t.items)
			idem := o.OrderNo + ":kitchen:" + t.printer + ":" + t.name
			if _, err := printSvc.EnqueueKitchen(o.OrderNo, t.printer, payload, idem); err != nil {
				log.Warn("后厨单入队失败", "orderNo", o.OrderNo, "station", t.name, "err", err)
			}
		}
	}

	// 作废也要出纸：纸面留痕，账才对得上。
	orderSvc.OnVoid(func(o order.Order) {
		if _, err := printSvc.EnqueueVoid(escpos.Void{
			OrderNo: o.OrderNo, CreatedAt: o.VoidedAt, Amount: money(o.Payable),
			Reason: o.VoidReason, Operator: o.VoidOperator,
		}); err != nil {
			log.Warn("作废小票入队失败", "orderNo", o.OrderNo, "err", err)
		}
	})
	table.NewHandler(table.NewService(db), orderSvc).Register(mux, func(w http.ResponseWriter, r *http.Request) bool {
		return requireManager(w, r, staffSvc)
	})
	order.Register(mux, orderSvc, order.Options{
		CanDiscount:    perm(setting.KeyPermDiscountCashier),
		CanRefund:      perm(setting.KeyPermRefundCashier),
		CanChangePrice: perm(setting.KeyPermPriceCashier),
		CanVoid:        perm(setting.KeyPermVoidCashier),
		MemberBenefit: func(memberID int64) (int64, int64) {
			m, err := member.NewService(db).Get(memberID)
			if err != nil {
				return 0, 1
			}
			t := member.NewService(db).TierOf(m)
			return t.Discount, t.PointRate
		},
		PointsEarn: func(*http.Request) int64 {
			v, _ := settingSvc.Get(setting.KeyPointsEarn, setting.DefaultPointsEarn)
			n, _ := strconv.ParseInt(v, 10, 64)
			if n <= 0 {
				n = 1
			}
			return n
		},
		StockBlock: func(*http.Request) bool {
			v, _ := settingSvc.Get(setting.KeyStockBlock, setting.DefaultStockBlock)
			return v == "true"
		},
		PointsExpireMonths: func(*http.Request) int64 {
			v, _ := settingSvc.Get(setting.KeyPointsExpireMonths, setting.DefaultPointsExpire)
			n, _ := strconv.ParseInt(v, 10, 64)
			return n
		},
		PointsRedeem: func(*http.Request) int64 {
			v, _ := settingSvc.Get(setting.KeyPointsRedeem, setting.DefaultPointsRedeem)
			n, _ := strconv.ParseInt(v, 10, 64)
			if n <= 0 {
				n = 100
			}
			return n
		},
		RoundMode: func(*http.Request) string {
			mode, _ := settingSvc.Get(setting.KeyRoundMode, setting.DefaultRoundMode)
			return mode
		},
		Operator: func(r *http.Request) string {
			st, _ := staff.FromContext(r.Context())
			return st.Name
		},
	})
	registerSettings(mux, settingSvc, staffSvc, db)
	// 打印队列（设置页只读列表）：最近 10 条任务的单号 / 目标 / 打印机 / 状态 / 重试。
	mux.HandleFunc("GET /pos/v1/print/jobs", func(w http.ResponseWriter, r *http.Request) {
		if !requireManager(w, r, staffSvc) {
			return
		}
		limit := 10
		if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 {
			limit = n
		}
		jobs, err := printSvc.List(limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "读取打印队列失败"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
	})

	mux.HandleFunc("POST /pos/v1/printers/test", func(w http.ResponseWriter, r *http.Request) {
		if !requireManager(w, r, staffSvc) {
			return
		}
		var body struct {
			Target string `json:"target"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		target := strings.TrimSpace(body.Target)
		if target == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请填写打印机名或 tcp://地址"})
			return
		}
		if print.IsTCP(target) {
			// 网口打印机：连通性 + 端口可达即算成功（不发送纸，避免浪费）
			addr, err := print.ParseTCPAddr(target)
			if err != nil {
				writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
				return
			}
			conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
			if err != nil {
				writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "连不上 " + addr + "：" + err.Error()})
				return
			}
			_ = conn.Close()
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "已连通 " + addr})
			return
		}
		// Windows 队列：能写测试页即算成功
		p, err := print.Resolve(target)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
			return
		}
		test := []byte("\x1b@打印机连接测试 OK\n\n\n")
		if err := p.Write(test); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "已发送测试页"})
	})

	mux.HandleFunc("GET /pos/v1/audit", func(w http.ResponseWriter, r *http.Request) {
		if !requireManager(w, r, staffSvc) {
			return
		}
		limit := 50
		if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 200 {
			limit = n
		}
		offset := 0
		if n, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && n >= 0 {
			offset = n
		}
		var total int
		_ = db.SQL().QueryRow("SELECT COUNT(*) FROM t_pos_audit").Scan(&total)
		rows, err := db.SQL().Query(
			"SELECT action,order_no,detail,operator,created_at FROM t_pos_audit"+
				" ORDER BY id DESC LIMIT ? OFFSET ?", limit, offset)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "读取审计失败"})
			return
		}
		defer rows.Close()
		out := make([]map[string]string, 0, limit)
		for rows.Next() {
			var action, orderNo, detail, operator, createdAt string
			if err := rows.Scan(&action, &orderNo, &detail, &operator, &createdAt); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "读取审计失败"})
				return
			}
			out = append(out, map[string]string{
				"action": action, "orderNo": orderNo, "detail": detail,
				"operator": operator, "createdAt": createdAt,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"total": total, "items": out})
	})

	shiftSvc.OnClosed(func(sh shift.Shift) {
		report := escpos.Shift{
			ShiftNo: sh.ShiftNo, StaffName: sh.StaffName,
			OpenedAt: sh.OpenedAt, ClosedAt: sh.ClosedAt,
			OpeningCash: money(sh.OpeningCash), OrderCount: sh.OrderCount, Total: money(sh.Total),
			Expected: money(sh.Expected), Actual: money(sh.Actual), Difference: money(sh.Difference),
			RefundTotal: money(sh.RefundTotal), VoidTotal: money(sh.VoidTotal),
			DineInCount: sh.DineInCount, TakeawayCnt: sh.TakeawayCnt,
			MemberDiscountTotal: money(sh.MemberDiscountTotal),
			PointsEarned: sh.PointsEarned, PointsUsed: sh.PointsUsed,
			CrossRefundCount: sh.CrossRefundCount, CrossRefundTotal: money(sh.CrossRefundTotal),
			Note: sh.Note,
		}
		for _, p := range sh.Payments {
			report.Payments = append(report.Payments, escpos.Payment{Method: p.Method, Amount: money(p.Amount)})
		}
		if _, err := printSvc.EnqueueReport(report); err != nil {
			log.Warn("交接单入队失败", "shiftNo", sh.ShiftNo, "err", err)
		}
	})
	shift.Register(mux, shiftSvc)

	// 商品图片上传：multipart，限 3MB，只收 png/jpeg，统一转存 PNG（打印光栅只需要位图）。
	mux.HandleFunc("POST /pos/v1/products/{id}/image", func(w http.ResponseWriter, r *http.Request) {
		if !requireManager(w, r, staffSvc) {
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "商品编号不合法"})
			return
		}
		img, err := readUploadedImage(w, r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
			return
		}
		name := fmt.Sprintf("p%d.png", id)
		if err := os.WriteFile(filepath.Join(imagesDir, name), img, 0o644); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "保存图片失败"})
			return
		}
		if err := productSvc.SetImage(id, "images/"+name); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"message": "商品不存在"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"image": "images/" + name})
	})

	// 价签打印：即时出票，不入打印队列
	mux.HandleFunc("POST /pos/v1/print/labels", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ProductIDs []int64 `json:"productIds"`
			Qty        int     `json:"qty"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		if len(body.ProductIDs) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请选择商品"})
			return
		}
		if body.Qty <= 0 || body.Qty > 50 {
			body.Qty = 1
		}
		shopName, _, _, _ := settingSvc.Shop()
		labels := make([]escpos.Label, 0, len(body.ProductIDs)*body.Qty)
		for _, pid := range body.ProductIDs {
			p, err := productSvc.Get(pid)
			if err != nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"message": fmt.Sprintf("商品 %d 不存在", pid)})
				return
			}
			for i := 0; i < body.Qty; i++ {
				labels = append(labels, escpos.Label{
					ShopName: shopName, Name: p.Name,
					Price: money(p.Price), Barcode: p.Barcode,
				})
			}
		}
		n, err := printSvc.PrintLabels(labels)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"message": "打印失败：" + err.Error(), "printed": fmt.Sprint(n)})
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"printed": n})
	})

	// 店铺 logo 上传（小票顶部）
	mux.HandleFunc("POST /pos/v1/settings/logo", func(w http.ResponseWriter, r *http.Request) {
		if !requireManager(w, r, staffSvc) {
			return
		}
		img, err := readUploadedImage(w, r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
			return
		}
		if err := os.WriteFile(filepath.Join(dataDir, "logo.png"), img, 0o644); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "保存 logo 失败：" + err.Error()})
			return
		}
		if err := settingSvc.Set(setting.KeyReceiptLogo, "logo.png"); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "保存设置失败"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"logo": "logo.png"})
	})

	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		// db 路径用于自检：确认 E2E 连上的是本次实例，而不是残留进程
		abs, _ := filepath.Abs(*dbPath)
		writeJSON(w, http.StatusOK, map[string]string{"version": version, "db": abs})
	})
	mux.Handle("/", webui.Handler())

	srv := &http.Server{
		Addr:              *addr,
		Handler:           staffSvc.Middleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// 数据库是唯一真源：启动即打一份快照，之后每天一份，只留最近 N 份。
	if !*noBackup {
		backupSvc := backup.New(db, filepath.Join(filepath.Dir(*dbPath), "backups"), *backupKeep, log)
		backupSvc.Start(ctx)
		registerBackup(mux, backupSvc, staffSvc)
	}
	defer stop()
	go printSvc.Run(ctx)
	// 积分过期：启动一次，之后每 6 小时扫一遍。
	go func() {
		memSvc := member.NewService(db)
		sweep := func() {
			n, err := memSvc.ExpirePoints(time.Now().Format("2006-01-02"))
			if err != nil {
				log.Warn("积分过期扫描失败", "err", err)
			} else if n > 0 {
				log.Info("积分过期处理完成", "members", n)
			}
		}
		sweep()
		t := time.NewTicker(6 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				sweep()
			}
		}
	}()

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()

	// 启动先扫一次，之后每 60 秒清理。
	go func() {
		sweep := func() {
			minutes := 30
			if v, _ := settingSvc.Get(setting.KeyHoldExpireMinutes, setting.DefaultHoldExpire); v != "" {
				if parsed, err := strconv.Atoi(v); err == nil {
					minutes = parsed
				}
			}
			n, err := orderSvc.AutoCancelHeld(time.Now(), time.Duration(minutes)*time.Minute)
			if err != nil {
				log.Warn("挂单自动取消失败", "err", err)
			} else if n > 0 {
				log.Info("挂单超时自动取消", "orders", n)
			}
		}
		sweep()
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				sweep()
			}
		}
	}()

	log.Info("posd 已启动", "addr", *addr, "db", *dbPath, "version", version)
	if *lan {
		// 手机要连的是本机在局域网里的地址，不是 127.0.0.1
		for _, u := range lanURLs(*addr) {
			log.Info("手机访问地址", "url", u)
		}
		log.Info("手机访问提示", "说明", "手机与电脑连同一个 Wi-Fi；Windows 防火墙需放行 posd")
	}
	uiURL := "http://" + strings.Replace(*addr, "0.0.0.0", "127.0.0.1", 1) + "/"
	if !*noOpen && !hasConsole() {
		openBrowser(uiURL)
	}
	if !*noTray && !hasConsole() {
		// 桌面版：托盘常驻，双击打开收银台，右键可退出
		if err := startTray("Posd 单机收银", func() { openBrowser(uiURL) }, func() { os.Exit(0) }); err != nil {
			log.Warn("托盘图标创建失败（不影响收银）", "err", err)
		} else {
			log.Info("托盘图标已创建", "提示", "双击打开收银台，右键退出")
		}
	}
	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	case err := <-errc:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// registerSettings 提供门店设置读写；写入需要店长。
func registerSettings(mux *http.ServeMux, svc *setting.Service, staffSvc *staff.Service, db *store.DB) {
	mux.HandleFunc("GET /pos/v1/settings", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, settingsSnapshot(svc))
	})

	mux.HandleFunc("PUT /pos/v1/settings", func(w http.ResponseWriter, r *http.Request) {
		if !requireManager(w, r, staffSvc) {
			return
		}
		var body map[string]string
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "请求体不是合法 JSON"})
			return
		}
		allowed := map[string]bool{
			setting.KeyShopName: true, setting.KeyShopPhone: true, setting.KeyShopFooter: true,
			setting.KeyPermRefundCashier:   true,
			setting.KeyPermDiscountCashier: true,
			setting.KeyPermPriceCashier:    true,
			setting.KeyPermVoidCashier:     true,
			setting.KeySetupDone:           true,
			setting.KeyIndustry:            true,
			setting.KeyRoundMode:           true,
			setting.KeyKitchenPrinter:      true,
			setting.KeyKitchenCategories:   true,
			setting.KeyPointsEarn:          true,
			setting.KeyPointsRedeem:        true,
			setting.KeyMemberTiers:         true,
			setting.KeyPointsExpireMonths:  true,
			setting.KeyHoldExpireMinutes:   true,
			setting.KeyTopupBonusRules:     true,
			setting.KeyStockBlock:          true,
			setting.KeyReceiptFooter:       true,
			setting.KeyReceiptQRText:       true,
			setting.KeyReceiptLogo:         true,
			setting.KeyPrintMode:           true,
		}
		// 设置会直接影响折扣/抹零/积分，改了什么必须留痕。
		actor := "店长"
		if st, ok := staff.FromContext(r.Context()); ok && st.Name != "" {
			actor = st.Name
		}
		for k, v := range body {
			if !allowed[k] {
				writeJSON(w, http.StatusBadRequest, map[string]string{"message": "不支持的配置项：" + k})
				return
			}
			old, _ := svc.Get(k, "")
			if old == v {
				continue
			}
			if err := svc.Set(k, v); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "保存失败"})
				return
			}
			_, _ = db.SQL().Exec(
				"INSERT INTO t_pos_audit(action,order_no,product_id,detail,operator,created_at)"+
					" VALUES('setting_change','','',?,?,datetime('now','localtime'))",
				k+": "+old+" -> "+v, actor)
		}
		writeJSON(w, http.StatusOK, settingsSnapshot(svc))
	})
}

// settingsSnapshot 返回**完整**设置（含默认值）。
//
// GET 与 PUT 必须返回同一结构：前端保存后会用它回填本地状态，
// 少一个字段，前端就会把该项重置成默认值——曾因此把 round_mode 打回 none。
func settingsSnapshot(svc *setting.Service) map[string]string {
	all, _ := svc.All()
	name, phone, footer, _ := svc.Shop()
	refundPerm, _ := svc.Bool(setting.KeyPermRefundCashier, false)
	discountPerm, _ := svc.Bool(setting.KeyPermDiscountCashier, false)
	pricePerm, _ := svc.Bool(setting.KeyPermPriceCashier, false)
	voidPerm, _ := svc.Bool(setting.KeyPermVoidCashier, false)
	setupDone, _ := svc.Bool(setting.KeySetupDone, false)
	industry, _ := svc.Get(setting.KeyIndustry, "tea")
	roundMode, _ := svc.Get(setting.KeyRoundMode, setting.DefaultRoundMode)
	kitchenPrinter, _ := svc.Get(setting.KeyKitchenPrinter, "")
	kitchenCats, _ := svc.Get(setting.KeyKitchenCategories, "")
	locale, _ := svc.Get(setting.KeyLocale, setting.DefaultLocale)
	timezone, _ := svc.Get(setting.KeyTimezone, setting.DefaultTimezone)
	currency, _ := svc.Get(setting.KeyCurrency, setting.DefaultCurrency)
	currencySymbol, _ := svc.Get(setting.KeyCurrencySymbol, setting.DefaultCurrencySymbol)
	timeFormat, _ := svc.Get(setting.KeyTimeFormat, setting.DefaultTimeFormat)
	taxRate, _ := svc.Get(setting.KeyTaxRate, setting.DefaultTaxRate)
	taxInclusive, _ := svc.Get(setting.KeyTaxInclusive, setting.DefaultTaxInclusive)
	pointsEarn, _ := svc.Get(setting.KeyPointsEarn, setting.DefaultPointsEarn)
	pointsRedeem, _ := svc.Get(setting.KeyPointsRedeem, setting.DefaultPointsRedeem)
	memberTiers, _ := svc.Get(setting.KeyMemberTiers, setting.DefaultMemberTiers)
	pointsExpire, _ := svc.Get(setting.KeyPointsExpireMonths, setting.DefaultPointsExpire)
	holdExpire, _ := svc.Get(setting.KeyHoldExpireMinutes, setting.DefaultHoldExpire)
	topupBonus, _ := svc.Get(setting.KeyTopupBonusRules, setting.DefaultTopupBonus)
	printMode, _ := svc.Get(setting.KeyPrintMode, setting.DefaultPrintMode)

	out := map[string]string{
		setting.KeyShopName: name, setting.KeyShopPhone: phone, setting.KeyShopFooter: footer,
		setting.KeyPermRefundCashier:   boolStr(refundPerm),
		setting.KeyPermDiscountCashier: boolStr(discountPerm),
		setting.KeyPermPriceCashier:    boolStr(pricePerm),
		setting.KeyPermVoidCashier:     boolStr(voidPerm),
		setting.KeySetupDone:           boolStr(setupDone),
		setting.KeyIndustry:            industry,
		setting.KeyRoundMode:           roundMode,
		setting.KeyKitchenPrinter:      kitchenPrinter,
		setting.KeyKitchenCategories:   kitchenCats,
		setting.KeyLocale:              locale,
		setting.KeyTimezone:            timezone,
		setting.KeyCurrency:            currency,
		setting.KeyCurrencySymbol:      currencySymbol,
		setting.KeyTimeFormat:          timeFormat,
		setting.KeyTaxRate:             taxRate,
		setting.KeyTaxInclusive:        taxInclusive,
		setting.KeyPointsEarn:          pointsEarn,
		setting.KeyPointsRedeem:        pointsRedeem,
		setting.KeyMemberTiers:         memberTiers,
		setting.KeyPointsExpireMonths:  pointsExpire,
		setting.KeyHoldExpireMinutes:   holdExpire,
		setting.KeyTopupBonusRules:     topupBonus,
		setting.KeyPrintMode:           printMode,
	}
	for k, v := range all {
		out[k] = v
	}
	return out
}

// readUploadedImage 读取 multipart 里的图片，校验格式并统一编码为 PNG。
// 限 3MB、边长 1200px 内（小票/价签用不到更大）。
func readUploadedImage(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		return nil, fmt.Errorf("解析上传失败")
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		return nil, fmt.Errorf("缺少 file 字段")
	}
	defer file.Close()
	limited := io.LimitReader(file, 3<<20)
	src, _, err := image.Decode(limited)
	if err != nil {
		return nil, fmt.Errorf("只支持 png/jpeg 图片")
	}
	b := src.Bounds()
	if b.Dx() > 1200 || b.Dy() > 1200 {
		return nil, fmt.Errorf("图片过大（最长边 1200px）")
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		return nil, fmt.Errorf("转码失败")
	}
	return buf.Bytes(), nil
}

// requireManager 校验 Bearer token 且角色为店长。
func requireManager(w http.ResponseWriter, r *http.Request, staffSvc *staff.Service) bool {
	h := r.Header.Get("Authorization")
	token := ""
	if len(h) > 7 && h[:7] == "Bearer " {
		token = h[7:]
	}
	st, ok := staffSvc.Verify(token)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "登录已过期，请重新登录"})
		return false
	}
	if st.Role != "manager" {
		writeJSON(w, http.StatusForbidden, map[string]string{"message": "需要店长权限"})
		return false
	}
	return true
}

// registerTemplates 业态模板：列表 + 一键应用（店长）。
func registerTemplates(mux *http.ServeMux, products *catalog.Service, staffSvc *staff.Service,
	settings *setting.Service, log *slog.Logger) {
	mux.HandleFunc("GET /pos/v1/catalog/templates", func(w http.ResponseWriter, r *http.Request) {
		all, err := templates.All()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "读取模板失败"})
			return
		}
		out := make([]map[string]any, 0, len(all))
		for _, t := range all {
			out = append(out, map[string]any{
				"id": t.ID, "name": t.Name, "summary": t.Summary,
				"categories": t.Categories, "productCount": len(t.Products),
			})
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("POST /pos/v1/catalog/templates/{id}/apply", func(w http.ResponseWriter, r *http.Request) {
		if !requireManager(w, r, staffSvc) {
			return
		}
		tpl, err := templates.Load(r.PathValue("id"))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"message": err.Error()})
			return
		}
		items := make([]catalog.ImportItem, 0, len(tpl.Products))
		for _, it := range tpl.Products {
			items = append(items, catalog.ImportItem{
				Name: it.Name, Category: it.Category, Barcode: it.Barcode,
				Price: it.Price, Cost: it.Cost, Hot: it.Hot,
			})
		}
		imported, replaced, err := products.ImportTemplate(items)
		if err != nil {
			log.Warn("导入业态模板失败", "id", tpl.ID, "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "导入失败"})
			return
		}
		_ = settings.Set(setting.KeyIndustry, tpl.ID)
		_ = settings.Set(setting.KeySetupDone, "true")
		writeJSON(w, http.StatusOK, map[string]any{
			"id": tpl.ID, "imported": imported, "replacedSeeds": replaced,
		})
	})
}

// registerBackup 提供手动备份（店长）；自动备份见启动逻辑。
func registerBackup(mux *http.ServeMux, svc *backup.Service, staffSvc *staff.Service) {
	mux.HandleFunc("POST /pos/v1/admin/backup", func(w http.ResponseWriter, r *http.Request) {
		if !requireManager(w, r, staffSvc) {
			return
		}
		path, err := svc.RunOnce()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "备份失败"})
			return
		}
		info, _ := os.Stat(path)
		writeJSON(w, http.StatusOK, map[string]any{
			"file": filepath.Base(path), "bytes": info.Size(),
		})
	})
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func money(cents int64) string { return fmt.Sprintf("%.2f", float64(cents)/100) }

// receiptTime：存储用 ISO/固定格式，英语小票改用无歧义的 "02 Jan 2006 15:04"（ADR-008）。
func receiptTime(raw, locale string) string {
	if !strings.HasPrefix(strings.ToLower(locale), "en") {
		return raw
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", time.RFC3339} {
		if d, err := time.Parse(layout, raw); err == nil {
			return d.Format("02 Jan 2006 15:04")
		}
	}
	return raw
}

// payMethodName：小票上的收款方式，跟随语言。
func payMethodName(m, locale string) string {
	if strings.HasPrefix(strings.ToLower(locale), "zh") {
		return methodName(m)
	}
	switch m {
	case "cash":
		return "Cash"
	case "wechat":
		return "WeChat Pay"
	case "alipay":
		return "Alipay"
	case "member":
		return "Store credit"
	case "points":
		return "Points"
	case "card":
		return "Card"
	case "split":
		return "Split"
	}
	return m
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// methodName 把支付方式编码翻成纸面中文。
func methodName(m string) string {
	switch m {
	case "cash":
		return "现金"
	case "wechat":
		return "微信"
	case "alipay":
		return "支付宝"
	case "member":
		return "会员余额"
	case "points":
		return "积分抵扣"
	default:
		return m
	}
}
