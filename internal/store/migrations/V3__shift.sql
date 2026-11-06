-- V3：交接班（开班 / 结班 / 交接单）
CREATE TABLE IF NOT EXISTS t_pos_shift (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  shop_id       INTEGER NOT NULL,
  shift_no      TEXT    NOT NULL,
  staff_name    TEXT    NOT NULL DEFAULT '',
  status        TEXT    NOT NULL,              -- open / closed
  opening_cash  INTEGER NOT NULL,
  expected_cash INTEGER,
  actual_cash   INTEGER,
  difference    INTEGER,
  order_count   INTEGER,
  note          TEXT    NOT NULL DEFAULT '',
  opened_at     TEXT    NOT NULL,
  closed_at     TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_shift_no ON t_pos_shift(shop_id, shift_no);
CREATE UNIQUE INDEX IF NOT EXISTS uk_shift_open ON t_pos_shift(shop_id) WHERE status='open';

ALTER TABLE t_pos_order ADD COLUMN shift_id INTEGER;

-- 修正历史数据：支付记录的金额应是「抵扣金额」，不是「实收现金」。
-- 单支付方式下二者相差一个找零，这里用订单应收覆盖。
UPDATE t_pos_payment
SET amount = (SELECT payable_amount FROM t_pos_order o WHERE o.id = t_pos_payment.order_id)
WHERE EXISTS (SELECT 1 FROM t_pos_order o WHERE o.id = t_pos_payment.order_id);
