-- V9：单品改价（记录原价 + 审计）
ALTER TABLE t_pos_order_item ADD COLUMN original_price INTEGER NOT NULL DEFAULT 0;
ALTER TABLE t_pos_order_item ADD COLUMN price_overridden INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS t_pos_audit (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  action     TEXT    NOT NULL,   -- price_override / refund / ...
  order_no   TEXT    NOT NULL DEFAULT '',
  product_id TEXT    NOT NULL DEFAULT '',
  detail     TEXT    NOT NULL DEFAULT '',
  operator   TEXT    NOT NULL DEFAULT '',
  created_at TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_order ON t_pos_audit(order_no);
