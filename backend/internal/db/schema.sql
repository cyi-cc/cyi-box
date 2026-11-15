-- 单用户模型：无用户表，登录密码 bcrypt 存 settings(k='admin_password_hash')；
-- 会话只记 token/有效期，不设属主
CREATE TABLE IF NOT EXISTS sessions (
  token      TEXT PRIMARY KEY,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS tools (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT NOT NULL,
  icon        TEXT NOT NULL DEFAULT '',
  url         TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  sort        INTEGER NOT NULL DEFAULT 0,
  enabled     INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS vault (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  title        TEXT NOT NULL,
  username     TEXT NOT NULL DEFAULT '',
  password_enc TEXT NOT NULL DEFAULT '',
  url          TEXT NOT NULL DEFAULT '',
  note         TEXT NOT NULL DEFAULT '',
  totp_enc     TEXT NOT NULL DEFAULT '',
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS bookmarks (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  title      TEXT NOT NULL,
  url        TEXT NOT NULL,
  icon       TEXT NOT NULL DEFAULT '',
  sort       INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS servers (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  name       TEXT NOT NULL,
  host       TEXT NOT NULL,
  port       INTEGER NOT NULL DEFAULT 22,
  username   TEXT NOT NULL,
  auth_type  TEXT NOT NULL DEFAULT 'password',
  secret_enc TEXT NOT NULL DEFAULT '',
  note       TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS files (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  name       TEXT NOT NULL,
  stored     TEXT NOT NULL,
  size       INTEGER NOT NULL DEFAULT 0,
  mime       TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS shares (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  code       TEXT NOT NULL UNIQUE,
  file_id    INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
  file_name  TEXT NOT NULL DEFAULT '',
  expires_at INTEGER NOT NULL DEFAULT 0,
  downloads  INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_shares_file ON shares(file_id);

CREATE TABLE IF NOT EXISTS dbconns (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  name         TEXT NOT NULL,
  engine       TEXT NOT NULL DEFAULT 'mysql',
  host         TEXT NOT NULL DEFAULT '',
  port         INTEGER NOT NULL DEFAULT 0,
  username     TEXT NOT NULL DEFAULT '',
  password_enc TEXT NOT NULL DEFAULT '',
  database     TEXT NOT NULL DEFAULT '',
  params       TEXT NOT NULL DEFAULT '',
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
  k TEXT PRIMARY KEY,
  v TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS captcha_keys (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  sitekey      TEXT NOT NULL UNIQUE,
  skey         TEXT NOT NULL UNIQUE,
  name         TEXT NOT NULL DEFAULT '',
  domains      TEXT NOT NULL DEFAULT '',
  enabled      INTEGER NOT NULL DEFAULT 1,
  used_count   INTEGER NOT NULL DEFAULT 0,
  created_at   INTEGER NOT NULL
);

-- 接入方调用流水：widget 出题 / verify 判定 / check 核销，留最近若干条供排障
CREATE TABLE IF NOT EXISTS captcha_logs (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  key_id     INTEGER NOT NULL DEFAULT 0,
  sitekey    TEXT NOT NULL DEFAULT '',
  event      TEXT NOT NULL DEFAULT '',
  ok         INTEGER NOT NULL DEFAULT 0,
  ip         TEXT NOT NULL DEFAULT '',
  msg        TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_captcha_logs_time ON captcha_logs(created_at);

CREATE TABLE IF NOT EXISTS pay_orders (
  id                INTEGER PRIMARY KEY AUTOINCREMENT,
  trade_no          TEXT NOT NULL UNIQUE,
  out_trade_no      TEXT NOT NULL UNIQUE,
  channel           TEXT NOT NULL DEFAULT 'alipay',
  subject           TEXT NOT NULL DEFAULT '',
  money             INTEGER NOT NULL,
  fee               INTEGER NOT NULL DEFAULT 0,
  status            INTEGER NOT NULL DEFAULT 0,   -- 0 待支付 1 已支付 3 已关闭
  notify_url        TEXT NOT NULL DEFAULT '',
  return_url        TEXT NOT NULL DEFAULT '',
  client_ip         TEXT NOT NULL DEFAULT '',
  notified          INTEGER NOT NULL DEFAULT 0,
  notify_attempts   INTEGER NOT NULL DEFAULT 0,
  paid_at           INTEGER NOT NULL DEFAULT 0,
  expired_at        INTEGER NOT NULL DEFAULT 0,
  upstream_trade_no TEXT NOT NULL DEFAULT '',
  goods_key         TEXT NOT NULL DEFAULT '',
  quantity          INTEGER NOT NULL DEFAULT 0,
  unit_price        INTEGER NOT NULL DEFAULT 0,
  payurl            TEXT NOT NULL DEFAULT '',
  qrcode            TEXT NOT NULL DEFAULT '',
  buyer_contact     TEXT NOT NULL DEFAULT '',
  query_pwd         TEXT NOT NULL DEFAULT '',
  cards             TEXT NOT NULL DEFAULT '',
  created_at        INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pay_orders_status ON pay_orders(status, created_at);

CREATE TABLE IF NOT EXISTS license_apps (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  appid      TEXT NOT NULL UNIQUE,
  secret     TEXT NOT NULL,
  name       TEXT NOT NULL DEFAULT '',
  enabled    INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS license_cards (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  app_id       INTEGER NOT NULL DEFAULT 0,
  card         TEXT NOT NULL UNIQUE,
  hours        INTEGER NOT NULL DEFAULT 24,
  domain       TEXT NOT NULL DEFAULT '',
  activated_at INTEGER NOT NULL DEFAULT 0,
  expires_at   INTEGER NOT NULL DEFAULT 0,
  created_at   INTEGER NOT NULL
);

-- 卡密校验流水：/v1/license 每次调用的成败与原因，留最近若干条供排障
CREATE TABLE IF NOT EXISTS license_logs (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  app_id     INTEGER NOT NULL DEFAULT 0,
  appid      TEXT NOT NULL DEFAULT '',
  card       TEXT NOT NULL DEFAULT '',
  event      TEXT NOT NULL DEFAULT '',
  ok         INTEGER NOT NULL DEFAULT 0,
  domain     TEXT NOT NULL DEFAULT '',
  ip         TEXT NOT NULL DEFAULT '',
  msg        TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_license_logs_time ON license_logs(created_at);

-- agent 共享记忆库：key 形态 "<project>/<topic>"，跨会话/跨 agent 持久事实
CREATE TABLE IF NOT EXISTS memories (
  k          TEXT PRIMARY KEY,
  content    TEXT NOT NULL,
  project    TEXT NOT NULL DEFAULT '',
  tags       TEXT NOT NULL DEFAULT '[]',
  written_by TEXT NOT NULL DEFAULT '',
  revision   INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_memories_project ON memories(project, updated_at);
