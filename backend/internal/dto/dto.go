// Package dto RPC 线协议类型。遵守 fun DTO 约束：定宽整型、无 map/any，
// 可空字段一律 *T，响应键自动转 camelCase。
package dto

type OkView struct {
	Message string
}

// ---- auth ----

type LoginDto struct {
	Name     string
	Password string
}

type UserView struct {
	Id        int64
	Name      string
	Role      string
	Status    string
	CreatedAt int64
}

type SessionView struct {
	User  *UserView
	Token *string
}

type ChangePasswordDto struct {
	OldPassword string
	NewPassword string
}

// ---- user admin ----

type ListUsersDto struct {
	Page     int64
	PageSize int64
	Keyword  *string
}

type UserPageView struct {
	Total int64
	Items []UserView
}

// SaveUserDto 创建/更新一体：Id 为空为新建；Password 更新时可空（不填不改密码）
type SaveUserDto struct {
	Id       *int64
	Name     string
	Password *string
	Role     string
	Status   *string
}

type DeleteUserDto struct {
	Id int64
}

// ---- dashboard ----

type TrendPoint struct {
	Day   string
	Count int64
}

type StatsView struct {
	TotalUsers     int64
	ActiveSessions int64
	TodayLogins    int64
	UptimeSeconds  int64
	LoginTrend     []TrendPoint

	Servers        int64
	Proxies        int64
	AliveProxies   int64
	TodayOrders    int64
	TodayMoney     int64
	TotalMoney     int64
	PendingOrders  int64
	LicenseApps    int64
	LicenseCards   int64
	LicenseActive  int64
	Memories       int64
	MemoryProjects int64
	Bookmarks      int64
	VaultItems     int64
	Dbconns        int64
	Files          int64

	PayTrend       []PayTrendPoint
	RecentOrders   []RecentOrderView
	RecentMemories []RecentMemoryView
}

type PayTrendPoint struct {
	Day   string
	Money int64
	Count int64
}

type RecentOrderView struct {
	TradeNo string
	Subject string
	Money   int64
	PaidAt  int64
}

type RecentMemoryView struct {
	Key       string
	Project   string
	UpdatedAt int64
}

// ---- tools ----

type ToolView struct {
	Id          int64
	Name        string
	Icon        string
	Url         string
	Description string
	Sort        int64
	Enabled     bool
}

type SaveToolDto struct {
	Id          *int64
	Name        string
	Icon        *string
	Url         string
	Description *string
	Sort        *int64
	Enabled     *bool
}

type DeleteToolDto struct {
	Id int64
}

// ---- vault 密码箱 ----

// VaultItemView 列表视图：密文字段不下发，只给 hasXxx 标记
type VaultItemView struct {
	Id          int64
	Title       string
	Username    *string
	Url         *string
	Note        *string
	HasPassword bool
	HasTotp     bool
	UpdatedAt   int64
}

// SaveVaultDto Password/TotpSecret 更新时传空指针表示不变；
// TotpSecret 支持裸 base32 或 otpauth://totp/... 链接；传空字符串表示清除
type SaveVaultDto struct {
	Id          *int64
	Title       string
	Username    *string
	Password    *string
	Url         *string
	Note        *string
	TotpSecret  *string
}

type VaultIdDto struct {
	Id int64
}

type SecretView struct {
	Value string
}

type TotpView struct {
	Code        string
	SecondsLeft int64
}

// ---- bookmarks 书签 ----

type BookmarkView struct {
	Id    int64
	Title string
	Url   string
	Icon  string
	Sort  int64
}

type SaveBookmarkDto struct {
	Id    *int64
	Title string
	Url   string
	Icon  *string
	Sort  *int64
}

type BookmarkIdDto struct {
	Id int64
}

// ---- servers 服务器管理 ----

// ServerView 列表视图：secret 密文不下发，只给 hasSecret 标记
type ServerView struct {
	Id        int64
	Name      string
	Host      string
	Port      int64
	Username  string
	AuthType  string
	HasSecret bool
	Note      *string
	CreatedAt int64
}

// SaveServerDto Secret 为密码或 PEM 私钥（按 AuthType）；nil = 不变，"" = 清除
type SaveServerDto struct {
	Id       *int64
	Name     string
	Host     string
	Port     *int64
	Username string
	AuthType string
	Secret   *string
	Note     *string
}

type ServerIdDto struct {
	Id int64
}

type DiskMountView struct {
	Mount   string
	Fs      string
	Total   int64
	Used    int64
	Avail   int64
	Percent int64
}

// ServerMetricsView 百分比直接给 0-100 整数；字节量单位 byte；速率单位 byte/s
type ServerMetricsView struct {
	Hostname      string
	Os            string
	Kernel        string
	UptimeSeconds int64
	CpuPercent    int64
	Load1         string
	Load5         string
	Load15        string
	MemTotal      int64
	MemUsed       int64
	MemAvail      int64
	SwapTotal     int64
	SwapUsed      int64
	DiskTotal     int64
	DiskUsed      int64
	DiskAvail     int64
	NetRxRate     int64
	NetTxRate     int64
	NetRxTotal    int64
	NetTxTotal    int64
	Disks         []DiskMountView
	CollectedAt   int64
}

// ---- sftp ----

type SftpPathDto struct {
	Id   int64
	Path string
}

type SftpEntryView struct {
	Name    string
	Size    int64
	Mode    string
	IsDir   bool
	ModTime int64
}

type SftpListView struct {
	Path    string
	Entries []SftpEntryView
}

// SftpRenameDto Path 为原完整路径，NewName 为同目录新名字
type SftpRenameDto struct {
	Id      int64
	Path    string
	NewName string
}

// ---- dbm 通用数据库管理 ----

// DbconnView 连接视图：密码密文不下发
type DbconnView struct {
	Id          int64
	Name        string
	Engine      string
	Host        string
	Port        int64
	Username    string
	Database    string
	HasPassword bool
	CreatedAt   int64
}

// SaveDbconnDto Password 为 nil=不变、""=清除；sqlite 时 Host 可空、Database 为文件路径
type SaveDbconnDto struct {
	Id       *int64
	Name     string
	Engine   string
	Host     *string
	Port     *int64
	Username *string
	Password *string
	Database *string
	Params   *string
}

type DbconnIdDto struct {
	Id int64
}

// DbScopeDto 带可选库的查询范围
type DbScopeDto struct {
	Id       int64
	Database *string
}

type DbDatabasesView struct {
	Names []string
}

type DbTableView struct {
	Name string
	Type string
}

type DbColumnView struct {
	Table string
	Name  string
	Type  string
	Pk    bool
}

type DbTablesView struct {
	Tables  []DbTableView
	Columns []DbColumnView
}

// DbQueryDto Sql 单条语句；MaxRows 服务端封顶
type DbQueryDto struct {
	Id       int64
	Database *string
	Sql      string
	MaxRows  *int64
}

// DbQueryView RowsJson 为 [][]any 的 JSON 串（DTO 不支持嵌套数组/any，前端自行 parse）
type DbQueryView struct {
	IsSelect     bool
	Columns      []string
	RowsJson     string
	RowCount     int64
	AffectedRows int64
	Truncated    bool
	ElapsedMs    int64
}

type DbImportView struct {
	Inserted int64
	Skipped  int64
}

// ---- webreq HTTP 代理 ----

// WebFetchDto HeadersJson 为 {"k":"v"} 对象 JSON 串
type WebFetchDto struct {
	Method      string
	Url         string
	HeadersJson string
	Body        string
}

type WebFetchView struct {
	Status      int64
	HeadersJson string
	Body        string
	ElapsedMs   int64
	Truncated   bool
}

// ---- disk 网盘 ----

type FileView struct {
	Id        int64
	Name      string
	Size      int64
	Mime      string
	CreatedAt int64
}

// ShareView expiresAt=0 表示永久有效；expired 由服务端算好给前端直接显示
type ShareView struct {
	Id        int64
	Code      string
	FileName  string
	ExpiresAt int64
	Expired   bool
	Downloads int64
	CreatedAt int64
}

// CreateShareDto ExpireMinutes：0=永久，>0=从现在起 N 分钟
type CreateShareDto struct {
	FileId        int64
	ExpireMinutes int64
}

type IdDto struct {
	Id int64
}

// ---- proxy 代理池 ----

type ProxyRegionView struct {
	Id         int64
	Code       string
	Name       string
	ZhName     string
	Region     string
	AliveCount int64
}

type ProxyView struct {
	Id            int64
	Ip            string
	Port          int64
	Address       string
	Protocols     string
	Anonymity     string
	RegionId      int64
	RegionCode    string
	RegionName    string
	RegionZhName  string
	Latency       int64
	Speed         int64
	Uptime        int64
	Alive         bool
	FailCount     int64
	LastCheckedAt int64
	LastAliveAt   int64
	LastSeenAt    int64
}

type ProxyListDto struct {
	RegionId int64
	Protocol string
	Keyword  string
	Alive    int64
	Page     int64
	PageSize int64
}

type ProxyListView struct {
	Total int64
	Items []ProxyView
}

type ProxySyncView struct {
	Id         int64
	StartedAt  int64
	FinishedAt int64
	Status     string
	Source     string
	Total      int64
	Added      int64
	Removed    int64
	Updated    int64
	DetailJson string
	Error      string
}

type ProxyStatsView struct {
	Total       int64
	Regions     int64
	LastSyncAt  int64
	LastCheckAt int64
	LastStatus  string
	Checking    bool
	Syncing     bool
}

// ---- 代理提取密钥 ----

type ProxyKeyView struct {
	Id           int64
	Key          string
	Name         string
	RegionId     int64
	RegionCode   string
	RegionZhName string
	Enabled      bool
	UsedCount    int64
	LastUsedAt   int64
	CreatedAt    int64
}

// ProxyKeySaveDto regionId=0 表示全部地区；key 由服务端生成
type ProxyKeySaveDto struct {
	Name     string
	RegionId int64
}

// ---- settings ----

type SettingDto struct {
	Key   string
	Value string
}

type SettingView struct {
	Value string
}

// ---- pay ----

type PayOrderListDto struct {
	Page     int64
	PageSize int64
	Status   *int64
	Kw       *string
}

type PayOrderView struct {
	Id         int64
	TradeNo    string
	OutTradeNo string
	Channel    string
	Subject    string
	Money      int64
	Status     int64
	Notified   int64
	Payurl     string
	PaidAt     int64
	ExpiredAt  int64
	CreatedAt  int64
}

type PayOrderPageView struct {
	Total int64
	Items []PayOrderView
}

type PayStatsView struct {
	TodayCount    int64
	TodayMoney    int64
	TotalCount    int64
	TotalMoney    int64
	PendingCount  int64
	TodayPaidCount int64
	TotalPaidCount int64
}

type PayUpstreamDto struct {
	Username string
	Password string
}

type PayUpstreamStatusView struct {
	Configured   int64
	Username     string
	Nickname     string
	Shop         string
	TokenAge     int64
	GoodsId      int64
	GoodsKey     string
	GoodsName    string
	UnitPrice    int64
	StockCount   int64
	TargetMoney  int64
	WalletOk     int64
	WalletAvail  int64
	WalletFrozen int64
	Ensuring     int64
	LastError    string
}

type PayTestPayDto struct {
	Money string
}

type PayTestPayView struct {
	TradeNo   string
	Payurl    string
	Qrcode    string
	Money     int64
	Cashier   string
}

type PayCashierDto struct {
	TradeNo string
}

type PayCashierView struct {
	TradeNo    string
	OutTradeNo string
	Subject    string
	Money      int64
	Channel    string
	Status     int64
	Payurl     string
	Qrcode     string
	Cards      string
	CreatedAt  int64
	ExpiredAt  int64
}

type PayCashierStatusView struct {
	Status    int64
	ReturnUrl string
}

type LicenseListDto struct {
	AppId    int64
	Page     int64
	PageSize int64
	Kw       *string
}

type LicenseCardView struct {
	Id          int64
	Card        string
	Hours       int64
	Domain      string
	Status      int64
	ActivatedAt int64
	ExpiresAt   int64
	CreatedAt   int64
}

type LicensePageView struct {
	Total int64
	Items []LicenseCardView
}

type LicenseStatsView struct {
	Total   int64
	Unused  int64
	Active  int64
	Expired int64
}

type LicenseGenDto struct {
	AppId int64
	Count int64
	Hours int64
}

type LicenseGenView struct {
	Cards []string
}

type LicenseIdDto struct {
	Id int64
}

type LicenseStatsDto struct {
	AppId int64
}

type LicenseAppView struct {
	Id        int64
	Appid     string
	Name      string
	Enabled   int64
	CardCount int64
	CreatedAt int64
}

type LicenseAppSaveDto struct {
	Id      int64
	Name    string
	Enabled *int64
}

type LicenseAppsView struct {
	Items []LicenseAppView
}

type MemoryListDto struct {
	Project string
	Tag     string
	Query   string
	Limit   int64
}

type MemoryMetaView struct {
	Key       string
	Project   string
	Tags      []string
	WrittenBy string
	Revision  int64
	UpdatedAt int64
}

type MemoryListView struct {
	Items []MemoryMetaView
}

type MemoryView struct {
	Key       string
	Url       string
	Content   string
	Project   string
	Tags      []string
	WrittenBy string
	Revision  int64
	CreatedAt int64
	UpdatedAt int64
}

type MemoryKeyDto struct {
	Key string
}

type MemorySaveDto struct {
	Key     string
	Content string
	Project string
	Tags    []string
}

type MemorySaveView struct {
	Key      string
	Url      string
	Revision int64
}

type MemoryProjectView struct {
	Name  string
	Count int64
}

type MemoryProjectsView struct {
	Items []MemoryProjectView
}

type MemoryMcpView struct {
	Url   string
	Token string
}
