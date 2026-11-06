-- V25：会员等级与积分有效期默认值（否则设置页空着，等级永远只有"普通会员"）
INSERT OR IGNORE INTO t_pos_setting(k, v, updated_at)
VALUES ('member_tiers', '0:普通会员:0:1,1000:银卡会员:2:2,5000:金卡会员:5:3', datetime('now','localtime'));

INSERT OR IGNORE INTO t_pos_setting(k, v, updated_at)
VALUES ('points_expire_months', '0', datetime('now','localtime'));

INSERT OR IGNORE INTO t_pos_setting(k, v, updated_at)
VALUES ('points_earn_per_yuan', '1', datetime('now','localtime'));

INSERT OR IGNORE INTO t_pos_setting(k, v, updated_at)
VALUES ('points_per_yuan', '100', datetime('now','localtime'));
