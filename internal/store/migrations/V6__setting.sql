-- V6：门店设置（键值），用于小票抬头/脚注等可配置项
CREATE TABLE IF NOT EXISTS t_pos_setting (
  k          TEXT PRIMARY KEY,
  v          TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL
);
