-- V35：采购入库（进货单 → 库存增加 → 留成本）
CREATE TABLE IF NOT EXISTS t_pos_purchase (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  supplier   TEXT NOT NULL DEFAULT '',
  note       TEXT NOT NULL DEFAULT '',
  operator   TEXT NOT NULL DEFAULT '',
  total_cost INTEGER NOT NULL DEFAULT 0,   -- 分
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS t_pos_purchase_item (
  purchase_id INTEGER NOT NULL,
  product_id  INTEGER NOT NULL,
  qty         INTEGER NOT NULL,
  unit_cost   INTEGER NOT NULL,            -- 分/件
  PRIMARY KEY (purchase_id, product_id)
);
