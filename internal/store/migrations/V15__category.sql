-- V15：类目（商品当前用文本类目，这里建立可管理的主数据）
CREATE TABLE IF NOT EXISTS t_pos_category (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  name       TEXT NOT NULL UNIQUE,
  sort       INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);

-- 用已有商品的类目名回填，避免"建了表却是空的"
INSERT OR IGNORE INTO t_pos_category(name, sort, created_at)
SELECT DISTINCT category, 0, datetime('now','localtime')
FROM t_pos_product WHERE TRIM(COALESCE(category,'')) <> '';
