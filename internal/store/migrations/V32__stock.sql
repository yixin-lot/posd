-- V32：库存（MVP）
-- 只对「建了库存行」的商品做扣减；没建行的商品视为不跟踪库存（小店常见）。
CREATE TABLE IF NOT EXISTS t_pos_stock (
  product_id INTEGER PRIMARY KEY,
  qty        INTEGER NOT NULL DEFAULT 0,
  warn_qty   INTEGER NOT NULL DEFAULT 0,   -- 低于等于该值告警；0=不告警
  updated_at TEXT NOT NULL
);
