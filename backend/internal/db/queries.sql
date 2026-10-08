-- name: GetUserByName :one
SELECT * FROM users WHERE name = ? LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: ListUsers :many
SELECT * FROM users
WHERE name LIKE '%' || sqlc.arg(keyword) || '%'
ORDER BY id LIMIT ? OFFSET ?;

-- name: CountUsers :one
SELECT COUNT(*) FROM users WHERE name LIKE '%' || sqlc.arg(keyword) || '%';

-- name: CountAllUsers :one
SELECT COUNT(*) FROM users;

-- name: CountActiveUsers :one
SELECT COUNT(*) FROM users WHERE status = 'active';

-- name: CountAdmins :one
SELECT COUNT(*) FROM users WHERE role = 'admin' AND status = 'active';

-- name: InsertUser :execlastid
INSERT INTO users(name, password_hash, role, status, created_at, updated_at)
VALUES(?, ?, ?, 'active', ?, ?);

-- name: UpdateUser :exec
UPDATE users SET name = ?, role = ?, status = ?, updated_at = ? WHERE id = ?;

-- name: UpdateUserPassword :exec
UPDATE users SET name = ?, role = ?, status = ?, password_hash = ?, updated_at = ? WHERE id = ?;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = ?;

-- name: InsertSession :exec
INSERT INTO sessions(token, user_id, created_at, expires_at) VALUES(?, ?, ?, ?);

-- name: GetSessionUser :one
SELECT u.id, u.name, u.password_hash, u.role, u.status, u.created_at, u.updated_at, s.expires_at
FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token = ?;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token = ?;

-- name: DeleteOtherSessions :exec
DELETE FROM sessions WHERE user_id = ? AND token != ?;

-- name: TouchSession :exec
UPDATE sessions SET expires_at = ? WHERE token = ?;

-- name: CountActiveSessions :one
SELECT COUNT(*) FROM sessions WHERE expires_at > ?;

-- name: CountTodayLogins :one
SELECT COUNT(*) FROM sessions
WHERE date(created_at, 'unixepoch', 'localtime') = date('now', 'localtime');

-- name: LoginTrend :many
SELECT date(created_at, 'unixepoch', 'localtime') AS d, COUNT(*) AS c
FROM sessions WHERE created_at >= ? GROUP BY d;

-- name: ListTools :many
SELECT * FROM tools ORDER BY sort, id;

-- name: ListEnabledTools :many
SELECT * FROM tools WHERE enabled = 1 ORDER BY sort, id;

-- name: InsertTool :execlastid
INSERT INTO tools(name, icon, url, description, sort, enabled) VALUES(?, ?, ?, ?, ?, ?);

-- name: UpdateTool :exec
UPDATE tools SET name = ?, icon = ?, url = ?, description = ?, sort = ?, enabled = ? WHERE id = ?;

-- name: DeleteTool :exec
DELETE FROM tools WHERE id = ?;

-- name: CountTools :one
SELECT COUNT(*) FROM tools WHERE enabled = 1;

-- name: ListVault :many
SELECT * FROM vault WHERE user_id = ?
  AND (title LIKE '%' || sqlc.arg(keyword) || '%'
       OR username LIKE '%' || sqlc.arg(keyword) || '%'
       OR url LIKE '%' || sqlc.arg(keyword) || '%')
ORDER BY updated_at DESC;

-- name: GetVault :one
SELECT * FROM vault WHERE user_id = ? AND id = ?;

-- name: InsertVault :execlastid
INSERT INTO vault(user_id, title, username, password_enc, url, note, totp_enc, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateVault :execrows
UPDATE vault SET title = ?, username = ?, password_enc = ?, url = ?, note = ?, totp_enc = ?, updated_at = ?
WHERE id = ? AND user_id = ?;

-- name: DeleteVault :execrows
DELETE FROM vault WHERE user_id = ? AND id = ?;

-- name: ListBookmarks :many
SELECT * FROM bookmarks WHERE user_id = ? ORDER BY sort, id;

-- name: InsertBookmark :execlastid
INSERT INTO bookmarks(user_id, title, url, icon, sort, created_at) VALUES(?, ?, ?, ?, ?, ?);

-- name: UpdateBookmark :execrows
UPDATE bookmarks SET title = ?, url = ?, icon = ?, sort = ? WHERE id = ? AND user_id = ?;

-- name: DeleteBookmark :execrows
DELETE FROM bookmarks WHERE user_id = ? AND id = ?;

-- name: ListServers :many
SELECT * FROM servers ORDER BY id;

-- name: GetServer :one
SELECT * FROM servers WHERE id = ?;

-- name: InsertServer :execlastid
INSERT INTO servers(name, host, port, username, auth_type, secret_enc, note, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateServer :execrows
UPDATE servers SET name = ?, host = ?, port = ?, username = ?, auth_type = ?, secret_enc = ?, note = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteServer :execrows
DELETE FROM servers WHERE id = ?;

-- name: CountServers :one
SELECT COUNT(*) FROM servers;

-- name: ListDbconns :many
SELECT * FROM dbconns ORDER BY id;

-- name: GetDbconn :one
SELECT * FROM dbconns WHERE id = ?;

-- name: InsertDbconn :execlastid
INSERT INTO dbconns(name, engine, host, port, username, password_enc, database, params, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateDbconn :execrows
UPDATE dbconns SET name = ?, engine = ?, host = ?, port = ?, username = ?, password_enc = ?, database = ?, params = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteDbconn :execrows
DELETE FROM dbconns WHERE id = ?;

-- name: InsertFile :execlastid
INSERT INTO files(user_id, name, stored, size, mime, created_at) VALUES(?, ?, ?, ?, ?, ?);

-- name: ListFiles :many
SELECT * FROM files WHERE user_id = ? ORDER BY id DESC;

-- name: GetFile :one
SELECT * FROM files WHERE id = ?;

-- name: DeleteFile :execrows
DELETE FROM files WHERE id = ? AND user_id = ?;

-- name: InsertShare :execlastid
INSERT INTO shares(code, file_id, user_id, file_name, expires_at, created_at) VALUES(?, ?, ?, ?, ?, ?);

-- name: ListShares :many
SELECT * FROM shares WHERE user_id = ? ORDER BY id DESC;

-- name: GetShareByCode :one
SELECT sh.id, sh.code, sh.file_id, sh.user_id, sh.file_name, sh.expires_at, sh.downloads, sh.created_at,
       f.name, f.stored, f.size, f.mime
FROM shares sh JOIN files f ON f.id = sh.file_id WHERE sh.code = ?;

-- name: DeleteShare :execrows
DELETE FROM shares WHERE id = ? AND user_id = ?;

-- name: DeleteSharesByFile :exec
DELETE FROM shares WHERE file_id = ?;

-- name: IncShareDownloads :exec
UPDATE shares SET downloads = downloads + 1 WHERE code = ?;

-- name: UpsertProxyRegion :exec
INSERT INTO proxy_regions(code, name, zh_name, region, slug) VALUES(?, ?, ?, ?, ?)
ON CONFLICT(code) DO UPDATE SET name = excluded.name, zh_name = excluded.zh_name,
  region = excluded.region, slug = excluded.slug;

-- name: GetProxyRegionByCode :one
SELECT * FROM proxy_regions WHERE code = ?;

-- name: ListProxyRegions :many
SELECT r.*, (SELECT COUNT(*) FROM proxies p WHERE p.region_id = r.id AND p.alive = 1) AS alive_count
FROM proxy_regions r ORDER BY alive_count DESC, r.code;

-- name: ListProxyKeys :many
SELECT id, ip, port, source FROM proxies;

-- name: InsertProxy :execlastid
INSERT INTO proxies(ip, port, protocols, anonymity, source, region_id, latency, speed, uptime, alive, last_seen_at, created_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?);

-- name: UpdateProxySeen :exec
UPDATE proxies SET protocols = ?, anonymity = ?, region_id = ?, latency = ?, speed = ?, uptime = ?, last_seen_at = ?, alive = 1, fail_count = 0
WHERE id = ?;

-- name: DeleteProxy :exec
DELETE FROM proxies WHERE id = ?;

-- name: ListProxiesDue :many
SELECT id, ip, port, protocols, fail_count FROM proxies WHERE last_checked_at < ? ORDER BY last_checked_at LIMIT 20000;

-- name: MarkProxyAlive :exec
UPDATE proxies SET last_checked_at = ?, alive = 1, fail_count = 0, last_alive_at = ?, latency = ? WHERE id = ?;

-- name: MarkProxyFailed :exec
UPDATE proxies SET last_checked_at = ?, alive = 0, fail_count = ? WHERE id = ?;

-- name: DeleteDeadProxies :execrows
DELETE FROM proxies WHERE alive = 0 AND last_checked_at < ?;

-- name: CountProxies :one
SELECT COUNT(*) FROM proxies WHERE alive = 1;

-- name: CountProxiesFiltered :one
SELECT COUNT(*) FROM proxies p WHERE (? = 0 OR p.region_id = ?) AND (? = '' OR p.protocols LIKE '%' || ? || '%')
AND (? = '' OR p.ip LIKE '%' || ? || '%') AND (? < 0 OR p.alive = ?);

-- name: ListProxiesFiltered :many
SELECT p.*, r.code AS region_code, r.name AS region_name, r.zh_name AS region_zh_name
FROM proxies p JOIN proxy_regions r ON r.id = p.region_id
WHERE (? = 0 OR p.region_id = ?) AND (? = '' OR p.protocols LIKE '%' || ? || '%')
AND (? = '' OR p.ip LIKE '%' || ? || '%') AND (? < 0 OR p.alive = ?)
ORDER BY p.alive DESC, p.latency LIMIT ? OFFSET ?;

-- name: InsertProxySync :execlastid
INSERT INTO proxy_syncs(started_at, source) VALUES(?, ?);

-- name: FinishProxySync :exec
UPDATE proxy_syncs SET finished_at = ?, status = ?, total = ?, added = ?, removed = ?, updated = ?, detail = ?, error = ? WHERE id = ?;

-- name: ListProxySyncs :many
SELECT * FROM proxy_syncs ORDER BY id DESC LIMIT ?;

-- name: LastProxyCheckAt :one
SELECT COALESCE(MAX(last_checked_at), 0) FROM proxies;

-- name: InsertProxyApiKey :execlastid
INSERT INTO proxy_keys(kkey, name, region_id, created_at) VALUES(?, ?, ?, ?);

-- name: ListProxyApiKeys :many
SELECT k.*, r.code AS region_code, r.zh_name AS region_zh_name, r.name AS region_name
FROM proxy_keys k LEFT JOIN proxy_regions r ON r.id = k.region_id
ORDER BY k.id DESC;

-- name: GetProxyApiKey :one
SELECT * FROM proxy_keys WHERE kkey = ? AND enabled = 1;

-- name: TouchProxyApiKey :exec
UPDATE proxy_keys SET used_count = used_count + 1, last_used_at = ? WHERE id = ?;

-- name: DeleteProxyApiKey :execrows
DELETE FROM proxy_keys WHERE id = ?;

-- name: RandomProxies :many
SELECT p.id, p.ip, p.port, p.protocols, p.fail_count, r.code AS region_code, r.zh_name AS region_zh_name
FROM proxies p JOIN proxy_regions r ON r.id = p.region_id
WHERE p.alive = 1 AND (? = 0 OR p.region_id = ?) AND (? = '' OR p.protocols LIKE '%' || ? || '%')
AND (? = '' OR r.code = ?)
ORDER BY RANDOM() LIMIT ?;

-- name: GetSetting :one
SELECT v FROM settings WHERE k = ?;

-- name: UpsertSetting :exec
INSERT INTO settings (k, v) VALUES (?, ?) ON CONFLICT(k) DO UPDATE SET v = excluded.v;

-- name: CreatePayOrder :execlastid
INSERT INTO pay_orders (trade_no, out_trade_no, channel, subject, money, fee, notify_url, return_url, client_ip, expired_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetPayOrderByTradeNo :one
SELECT * FROM pay_orders WHERE trade_no = ?;

-- name: GetPayOrderByOutNo :one
SELECT * FROM pay_orders WHERE out_trade_no = ?;

-- name: SetPayOrderUpstream :exec
UPDATE pay_orders SET upstream_trade_no = ?, goods_key = ?, quantity = ?, unit_price = ?, payurl = ?, buyer_contact = ?, query_pwd = ? WHERE id = ?;

-- name: SetPayOrderQrcode :exec
UPDATE pay_orders SET qrcode = ? WHERE id = ?;

-- name: SetPayOrderStatus :exec
UPDATE pay_orders SET status = ?, paid_at = ?, cards = ? WHERE id = ?;

-- name: ListPendingPayOrders :many
SELECT * FROM pay_orders WHERE status = 0 AND upstream_trade_no != '' ORDER BY id LIMIT 200;

-- name: ListPayOrders :many
SELECT * FROM pay_orders
WHERE (? = -1 OR status = ?) AND (? = '' OR trade_no LIKE '%' || ? || '%' OR out_trade_no LIKE '%' || ? || '%')
ORDER BY id DESC LIMIT ? OFFSET ?;

-- name: CountPayOrders :one
SELECT COUNT(*) FROM pay_orders
WHERE (? = -1 OR status = ?) AND (? = '' OR trade_no LIKE '%' || ? || '%' OR out_trade_no LIKE '%' || ? || '%');

-- name: PayOrderStats :one
SELECT COUNT(*) AS cnt, COALESCE(SUM(CASE WHEN status = 1 THEN money ELSE 0 END), 0) AS money,
  COALESCE(SUM(CASE WHEN status = 0 THEN 1 ELSE 0 END), 0) AS pending,
  COALESCE(SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END), 0) AS paid
FROM pay_orders WHERE created_at >= ?;

-- name: IncPayNotifyAttempts :exec
UPDATE pay_orders SET notify_attempts = notify_attempts + 1 WHERE id = ?;

-- name: SetPayNotified :exec
UPDATE pay_orders SET notified = 1 WHERE id = ?;

-- name: GetPayOrder :one
SELECT * FROM pay_orders WHERE id = ?;

-- name: CreateLicenseApp :execlastid
INSERT INTO license_apps (appid, secret, name, created_at) VALUES (?, ?, ?, ?);

-- name: GetLicenseAppByAppid :one
SELECT * FROM license_apps WHERE appid = ?;

-- name: ListLicenseApps :many
SELECT a.*, (SELECT COUNT(*) FROM license_cards c WHERE c.app_id = a.id) AS card_count
FROM license_apps a ORDER BY a.id DESC;

-- name: UpdateLicenseApp :exec
UPDATE license_apps SET name = ?, enabled = ? WHERE id = ?;

-- name: DeleteLicenseApp :exec
DELETE FROM license_apps WHERE id = ?;

-- name: CreateLicenseCard :exec
INSERT INTO license_cards (app_id, card, hours, created_at) VALUES (?, ?, ?, ?);

-- name: GetLicenseCard :one
SELECT * FROM license_cards WHERE app_id = ? AND card = ?;

-- name: ActivateLicenseCard :exec
UPDATE license_cards SET domain = ?, activated_at = ?, expires_at = ? WHERE id = ?;

-- name: ListLicenseCards :many
SELECT * FROM license_cards
WHERE app_id = ? AND (? = '' OR card LIKE '%' || ? || '%' OR domain LIKE '%' || ? || '%')
ORDER BY id DESC LIMIT ? OFFSET ?;

-- name: CountLicenseCards :one
SELECT COUNT(*) FROM license_cards
WHERE app_id = ? AND (? = '' OR card LIKE '%' || ? || '%' OR domain LIKE '%' || ? || '%');

-- name: LicenseCardStats :one
SELECT COUNT(*) AS total,
  COALESCE(SUM(CASE WHEN activated_at = 0 THEN 1 ELSE 0 END), 0) AS unused,
  COALESCE(SUM(CASE WHEN activated_at > 0 AND (expires_at = 0 OR expires_at > ?) THEN 1 ELSE 0 END), 0) AS active,
  COALESCE(SUM(CASE WHEN activated_at > 0 AND expires_at > 0 AND expires_at <= ? THEN 1 ELSE 0 END), 0) AS expired
FROM license_cards WHERE app_id = ?;

-- name: DeleteLicenseCard :exec
DELETE FROM license_cards WHERE id = ?;

-- name: UpsertMemory :exec
INSERT INTO memories (k, content, project, tags, written_by, revision, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, 1, ?, ?)
ON CONFLICT(k) DO UPDATE SET
  content = excluded.content,
  project = excluded.project,
  tags = excluded.tags,
  written_by = excluded.written_by,
  revision = memories.revision + 1,
  updated_at = excluded.updated_at;

-- name: GetMemory :one
SELECT * FROM memories WHERE k = ?;

-- name: DeleteMemory :execrows
DELETE FROM memories WHERE k = ?;

-- name: ListMemories :many
SELECT * FROM memories
WHERE (? = '' OR project = ?) AND (? = '' OR tags LIKE '%"' || ? || '"%')
ORDER BY updated_at DESC LIMIT ?;

-- name: MemoryProjects :many
SELECT project AS name, COUNT(*) AS cnt FROM memories
WHERE project != '' GROUP BY project ORDER BY MAX(updated_at) DESC;

-- name: CountMemories :one
SELECT COUNT(*) FROM memories;

-- name: PayDailyPaid :many
SELECT CAST(strftime('%Y-%m-%d', paid_at, 'unixepoch', 'localtime') AS TEXT) AS day,
  COALESCE(SUM(money), 0) AS money, COUNT(*) AS cnt
FROM pay_orders WHERE status = 1 AND paid_at >= ? GROUP BY day;

-- name: RecentPaidOrders :many
SELECT trade_no, subject, money, paid_at FROM pay_orders WHERE status = 1 ORDER BY paid_at DESC LIMIT ?;

-- name: LicenseCardStatsAll :one
SELECT COUNT(*) AS total,
  COALESCE(SUM(CASE WHEN activated_at = 0 THEN 1 ELSE 0 END), 0) AS unused,
  COALESCE(SUM(CASE WHEN activated_at > 0 AND (expires_at = 0 OR expires_at > ?) THEN 1 ELSE 0 END), 0) AS active
FROM license_cards;

-- name: CountLicenseApps :one
SELECT COUNT(*) FROM license_apps;

-- name: CountProxiesAll :one
SELECT COUNT(*) FROM proxies;

-- name: CountBookmarksByUser :one
SELECT COUNT(*) FROM bookmarks WHERE user_id = ?;

-- name: CountVaultByUser :one
SELECT COUNT(*) FROM vault WHERE user_id = ?;

-- name: CountDbconns :one
SELECT COUNT(*) FROM dbconns;

-- name: CountFilesAll :one
SELECT COUNT(*) FROM files;
