-- V7：退款（整单）与权限开关落到订单
ALTER TABLE t_pos_order ADD COLUMN refund_amount INTEGER NOT NULL DEFAULT 0;
ALTER TABLE t_pos_order ADD COLUMN refunded_at TEXT;

CREATE TABLE IF NOT EXISTS t_pos_refund (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  order_id    INTEGER NOT NULL,
  order_no    TEXT    NOT NULL,
  amount      INTEGER NOT NULL,
  reason      TEXT    NOT NULL DEFAULT '',
  operator    TEXT    NOT NULL DEFAULT '',
  created_at  TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_refund_order ON t_pos_refund(order_id);
