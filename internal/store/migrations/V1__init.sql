-- V1：订单 / 明细 / 支付 / 打印任务（单店形态，ADR-002）
-- 金额一律用「分」存 INTEGER：不接受浮点，也不依赖数据库 decimal。
CREATE TABLE IF NOT EXISTS t_pos_order (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  shop_id         INTEGER NOT NULL,
  order_no        TEXT    NOT NULL,
  biz_date        TEXT    NOT NULL,
  status          TEXT    NOT NULL,
  total_amount    INTEGER NOT NULL,
  discount_amount INTEGER NOT NULL,
  payable_amount  INTEGER NOT NULL,
  cash_amount     INTEGER NOT NULL,
  change_amount   INTEGER NOT NULL,
  idem_key        TEXT,
  print_status    INTEGER NOT NULL DEFAULT 0,
  created_at      TEXT    NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_order_no ON t_pos_order(shop_id, order_no);
CREATE UNIQUE INDEX IF NOT EXISTS uk_order_idem ON t_pos_order(idem_key) WHERE idem_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS t_pos_order_item (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  order_id   INTEGER NOT NULL,
  product_id TEXT    NOT NULL,
  name       TEXT    NOT NULL,
  price      INTEGER NOT NULL,
  qty        INTEGER NOT NULL,
  amount     INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_item_order ON t_pos_order_item(order_id);

CREATE TABLE IF NOT EXISTS t_pos_payment (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  order_id   INTEGER NOT NULL,
  method     TEXT    NOT NULL,
  amount     INTEGER NOT NULL,
  created_at TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_payment_order ON t_pos_payment(order_id);

CREATE TABLE IF NOT EXISTS t_pos_print_job (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  order_id    INTEGER NOT NULL,
  order_no    TEXT    NOT NULL,
  target      TEXT    NOT NULL,
  payload     BLOB    NOT NULL,
  idem_key    TEXT    NOT NULL,
  status      TEXT    NOT NULL,
  attempts    INTEGER NOT NULL DEFAULT 0,
  error       TEXT    NOT NULL DEFAULT '',
  next_try_at INTEGER NOT NULL DEFAULT 0,
  created_at  INTEGER NOT NULL,
  printed_at  INTEGER
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_job_idem ON t_pos_print_job(idem_key);
CREATE INDEX IF NOT EXISTS idx_job_status ON t_pos_print_job(status, next_try_at);
