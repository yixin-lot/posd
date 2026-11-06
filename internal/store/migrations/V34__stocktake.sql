-- V34：库存盘点单（生成快照 → 录实盘 → 关单生效）
CREATE TABLE IF NOT EXISTS t_pos_stocktake (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  status     TEXT NOT NULL DEFAULT 'open',   -- open / done
  note       TEXT NOT NULL DEFAULT '',
  operator   TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  closed_at  TEXT
);

CREATE TABLE IF NOT EXISTS t_pos_stocktake_item (
  stocktake_id INTEGER NOT NULL,
  product_id   INTEGER NOT NULL,
  system_qty   INTEGER NOT NULL,
  actual_qty   INTEGER,                      -- NULL = 未盘
  PRIMARY KEY (stocktake_id, product_id)
);
