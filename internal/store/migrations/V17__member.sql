-- V17：会员基础（开卡 + 储值余额 + 流水）
CREATE TABLE IF NOT EXISTS t_pos_member (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  phone      TEXT NOT NULL UNIQUE,
  name       TEXT NOT NULL DEFAULT '',
  balance    INTEGER NOT NULL DEFAULT 0,   -- 分
  points     INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS t_pos_member_txn (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  member_id     INTEGER NOT NULL,
  kind          TEXT NOT NULL,             -- topup / consume / refund
  amount        INTEGER NOT NULL,          -- 分（正数；方向由 kind 决定）
  balance_after INTEGER NOT NULL,
  note          TEXT NOT NULL DEFAULT '',
  created_at    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_member_txn ON t_pos_member_txn(member_id);
