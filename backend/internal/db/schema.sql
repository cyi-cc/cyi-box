CREATE TABLE IF NOT EXISTS users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  name          TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role          TEXT NOT NULL DEFAULT 'user',
  status        TEXT NOT NULL DEFAULT 'active',
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  token      TEXT PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
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
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title        TEXT NOT NULL,
  username     TEXT NOT NULL DEFAULT '',
  password_enc TEXT NOT NULL DEFAULT '',
  url          TEXT NOT NULL DEFAULT '',
  note         TEXT NOT NULL DEFAULT '',
  totp_enc     TEXT NOT NULL DEFAULT '',
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_vault_user ON vault(user_id);

CREATE TABLE IF NOT EXISTS bookmarks (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title      TEXT NOT NULL,
  url        TEXT NOT NULL,
  icon       TEXT NOT NULL DEFAULT '',
  sort       INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_bookmarks_user ON bookmarks(user_id);

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
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  stored     TEXT NOT NULL,
  size       INTEGER NOT NULL DEFAULT 0,
  mime       TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_files_user ON files(user_id);

CREATE TABLE IF NOT EXISTS shares (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  code       TEXT NOT NULL UNIQUE,
  file_id    INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  file_name  TEXT NOT NULL DEFAULT '',
  expires_at INTEGER NOT NULL DEFAULT 0,
  downloads  INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_shares_user ON shares(user_id);
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

CREATE TABLE IF NOT EXISTS proxy_regions (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  code    TEXT NOT NULL UNIQUE,
  name    TEXT NOT NULL DEFAULT '',
  zh_name TEXT NOT NULL DEFAULT '',
  region  TEXT NOT NULL DEFAULT '',
  slug    TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS proxies (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  ip              TEXT NOT NULL,
  port            INTEGER NOT NULL,
  protocols       TEXT NOT NULL DEFAULT '',
  anonymity       TEXT NOT NULL DEFAULT '',
  source          TEXT NOT NULL DEFAULT '',
  region_id       INTEGER NOT NULL REFERENCES proxy_regions(id) ON DELETE CASCADE,
  latency         INTEGER NOT NULL DEFAULT 0,
  speed           INTEGER NOT NULL DEFAULT 0,
  uptime          INTEGER NOT NULL DEFAULT 0,
  alive           INTEGER NOT NULL DEFAULT 1,
  fail_count      INTEGER NOT NULL DEFAULT 0,
  last_checked_at INTEGER NOT NULL DEFAULT 0,
  last_alive_at   INTEGER NOT NULL DEFAULT 0,
  last_seen_at    INTEGER NOT NULL DEFAULT 0,
  created_at      INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_proxies_addr ON proxies(ip, port);
CREATE INDEX IF NOT EXISTS idx_proxies_region ON proxies(region_id);
CREATE INDEX IF NOT EXISTS idx_proxies_alive ON proxies(alive);
CREATE INDEX IF NOT EXISTS idx_proxies_check ON proxies(alive, last_checked_at);

CREATE TABLE IF NOT EXISTS proxy_syncs (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  started_at  INTEGER NOT NULL,
  finished_at INTEGER NOT NULL DEFAULT 0,
  status      TEXT NOT NULL DEFAULT '',
  source      TEXT NOT NULL DEFAULT '',
  total       INTEGER NOT NULL DEFAULT 0,
  added       INTEGER NOT NULL DEFAULT 0,
  removed     INTEGER NOT NULL DEFAULT 0,
  updated     INTEGER NOT NULL DEFAULT 0,
  detail      TEXT NOT NULL DEFAULT '',
  error       TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS proxy_keys (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  kkey         TEXT NOT NULL UNIQUE,
  name         TEXT NOT NULL DEFAULT '',
  region_id    INTEGER NOT NULL DEFAULT 0,
  enabled      INTEGER NOT NULL DEFAULT 1,
  used_count   INTEGER NOT NULL DEFAULT 0,
  last_used_at INTEGER NOT NULL DEFAULT 0,
  created_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
  k TEXT PRIMARY KEY,
  v TEXT NOT NULL DEFAULT ''
);

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
