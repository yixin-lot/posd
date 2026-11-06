-- V20：桌台（餐饮插件 MVP：开台挂单 → 结账清台）
CREATE TABLE IF NOT EXISTS t_pos_table (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  name       TEXT NOT NULL UNIQUE,
  status     TEXT NOT NULL DEFAULT 'free',   -- free / occupied
  order_no   TEXT NOT NULL DEFAULT '',
  guests     INTEGER NOT NULL DEFAULT 0,
  opened_at  TEXT,
  created_at TEXT NOT NULL
);
