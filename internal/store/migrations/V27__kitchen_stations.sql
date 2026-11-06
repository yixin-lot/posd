-- V27：后厨档口（一个档口=一台打印机+一组类目）
CREATE TABLE IF NOT EXISTS t_pos_kitchen_station (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  name       TEXT NOT NULL UNIQUE,
  printer    TEXT NOT NULL,
  categories TEXT NOT NULL DEFAULT '',   -- 逗号分隔；空=全部类目
  active     INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL
);
