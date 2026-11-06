-- V5：操作员与登录会话（店长 / 收银员）
CREATE TABLE IF NOT EXISTS t_pos_staff (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  shop_id    INTEGER NOT NULL,
  name       TEXT    NOT NULL,
  username   TEXT    NOT NULL,
  pin_salt   TEXT    NOT NULL,
  pin_hash   TEXT    NOT NULL,
  role       TEXT    NOT NULL,              -- manager / cashier
  active     INTEGER NOT NULL DEFAULT 1,
  created_at TEXT    NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_staff_username ON t_pos_staff(shop_id, username);

CREATE TABLE IF NOT EXISTS t_pos_session (
  token      TEXT PRIMARY KEY,
  staff_id   INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  created_at TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_session_expire ON t_pos_session(expires_at);
