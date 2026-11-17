package common

import (
	"crypto/tls"
	//"os"
	//"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

var StartTime = time.Now().Unix() // unit: second
var Version = "v0.0.0"            // this hard coding will be replaced automatically when building, no need to manually change
var SystemName = "New API"
var Footer = ""
var Logo = ""
var TopUpLink = ""

// var ChatLink = ""
// var ChatLink2 = ""
var QuotaPerUnit = 500 * 1000.0 // $0.002 / 1K tokens
// 保留旧变量以兼容历史逻辑，实际展示由 general_setting.quota_display_type 控制
var DisplayInCurrencyEnabled = true
var DisplayTokenStatEnabled = true
var DrawingEnabled = true
var TaskEnabled = true
var DataExportEnabled = true
var DataExportInterval = 5         // unit: minute
var DataExportDefaultTime = "hour" // unit: minute
var DefaultCollapseSidebar = false // default value of collapse sidebar

// Any options with "Secret", "Token" in its key won't be return by GetOptions

var SessionSecret = uuid.New().String()
var CryptoSecret = uuid.New().String()
var SessionCookieSecure = false

// DefaultLanguage is the DEFAULT_LANGUAGE environment variable: the language of
// server log lines and of text for a reader who states no language: en, zh-CN
// or zh-TW. English when empty.
var DefaultLanguage string
var SessionCookieTrustedURLs []string

const (
	DefaultUserSessionActiveLimit           = 50
	DefaultUserSessionIssuanceLimit         = 100
	DefaultUserSessionIssuanceWindowSeconds = 24 * 60 * 60
	DefaultUserSessionRevokedRetentionDays  = 7
	DefaultUserSessionHourlyAlertThreshold  = 5000
)

var (
	UserSessionActiveLimit           = DefaultUserSessionActiveLimit
	UserSessionIssuanceLimit         = DefaultUserSessionIssuanceLimit
	UserSessionIssuanceWindowSeconds = int64(DefaultUserSessionIssuanceWindowSeconds)
	UserSessionRevokedRetentionDays  = DefaultUserSessionRevokedRetentionDays
	UserSessionHourlyAlertThreshold  = DefaultUserSessionHourlyAlertThreshold
)

var OptionMap map[string]string
var OptionMapRWMutex sync.RWMutex

var ItemsPerPage = 10
var MaxRecentItems = 1000

var PasswordLoginEnabled = true
var PasswordLoginEncryptionEnabled = false
var PasswordRegisterEnabled = true
var EmailVerificationEnabled = false
var GitHubOAuthEnabled = false
var LinuxDOOAuthEnabled = false
var WeChatAuthEnabled = false
var TelegramOAuthEnabled = false
var TurnstileCheckEnabled = false
var RegisterEnabled = true

var EmailDomainRestrictionEnabled = false // 是否启用邮箱域名限制
var EmailAliasRestrictionEnabled = false  // 是否启用邮箱别名限制
var EmailDomainWhitelist = []string{
	"gmail.com",
	"163.com",
	"126.com",
	"qq.com",
	"outlook.com",
	"hotmail.com",
	"icloud.com",
	"yahoo.com",
	"foxmail.com",
}
var EmailLoginAuthServerList = []string{
	"smtp.sendcloud.net",
	"smtp.azurecomm.net",
}

var DebugEnabled bool
var MemoryCacheEnabled bool

var LogConsumeEnabled = true

var TLSInsecureSkipVerify bool
var InsecureTLSConfig = &tls.Config{InsecureSkipVerify: true}

var SMTPServer = ""
var SMTPPort = 587
var SMTPSSLEnabled = false
var SMTPStartTLSEnabled = false
var SMTPInsecureSkipVerify = false
var SMTPForceAuthLogin = false
var SMTPAccount = ""
var SMTPFrom = ""
var SMTPToken = ""

var GitHubClientId = ""
var GitHubClientSecret = ""
var LinuxDOClientId = ""
var LinuxDOClientSecret = ""
var LinuxDOMinimumTrustLevel = 0

var WeChatServerAddress = ""
var WeChatServerToken = ""
var WeChatAccountQRCodeImageURL = ""

var TurnstileSiteKey = ""
var TurnstileSecretKey = ""

var TelegramBotToken = ""
var TelegramBotName = ""

var QuotaForNewUser = 0
var QuotaForInviter = 0
var QuotaForInvitee = 0
var ChannelDisableThreshold = 5.0
var AutomaticDisableChannelEnabled = false
var AutomaticEnableChannelEnabled = false
var QuotaRemindThreshold = 1000

// PreConsumedQuota is retained for old option clients; token reservations now
// use quota_setting.pre_consume_multiplier and the estimated input cost.
var PreConsumedQuota = 500

var RetryTimes = 0

// ConcurrencyRetryInterval is the wait between concurrency-limit probes so a
// saturated channel has a chance to free a slot before the next attempt.
var ConcurrencyRetryInterval = 100 * time.Millisecond

// ConcurrencyHighPriorityWaitWindow is how long a request keeps probing the
// highest priority tier (instead of dropping to lower tiers) after a
// concurrency-limit 429. Fast-but-small high-priority channels free slots
// continuously, so waiting for one beats running on a slow low-priority
// channel. 0 restores the previous immediate-fallback behavior. Overridden by
// the CONCURRENCY_HIGH_PRIORITY_WAIT_WINDOW_MS env var.
var ConcurrencyHighPriorityWaitWindow = 2 * time.Second

// UpstreamQuotaCooldown keeps a channel out of selection after an upstream 429
// that reports an exhausted quota: that refusal does not clear within a burst,
// so re-probing it only multiplies traffic against a dead account.
var UpstreamQuotaCooldown = 60 * time.Second

// UpstreamRateLimitCooldown is the same for an upstream 429 that reports a
// plain rate limit. Such a burst clears quickly, so the channel is only parked
// long enough to stop concurrent requests from hammering it.
var UpstreamRateLimitCooldown = 3 * time.Second

// ChannelConcurrencyCooldown parks a channel after its own concurrency limit
// rejected the attempt. The channel is healthy and a slot frees as soon as an
// in-flight request finishes, so the park is deliberately short: it only keeps
// a concurrent burst from selecting the same saturated channel over and over.
var ChannelConcurrencyCooldown = 1 * time.Second

// UpstreamServerErrorCooldown parks a channel after an upstream 5xx. A 502/503
// usually means the upstream itself is failing, and each attempt against it
// costs the client the full failure latency before the retry moves on. Parking
// the channel keeps later requests on healthy channels instead of every one of
// them paying that failure again.
var UpstreamServerErrorCooldown = 15 * time.Second

// UpstreamForbiddenCooldown parks a channel after an upstream 403. That is
// almost always a WAF or credential rejection rather than a per-request
// problem, so the channel stays out of selection while an operator looks at it.
var UpstreamForbiddenCooldown = 30 * time.Second

// The *MaxCooldown values cap the doubling applied when a channel keeps
// refusing. A fixed short park would send one request at a long window every
// few seconds; doubling pushes the probes out until the window reopens, while
// the cap keeps a recovered channel from being parked for hours by a streak.
var (
	UpstreamRateLimitMaxCooldown   = 60 * time.Second
	UpstreamQuotaMaxCooldown       = 300 * time.Second
	UpstreamServerErrorMaxCooldown = 120 * time.Second
	UpstreamForbiddenMaxCooldown   = 300 * time.Second
)

//var RootUserEmail = ""

var IsMasterNode bool

const (
	NodeNameSourceManual   = "manual"
	NodeNameSourceHostname = "hostname"
)

// NodeName 节点名称，优先从 NODE_NAME 环境变量读取，未配置时回退主机名。
// 用于审计日志和后台任务中标识节点身份；多实例部署时建议显式配置稳定 NODE_NAME。
var NodeName = ""

// NodeNameSource records how NodeName was chosen so future instance-management
// reporting can distinguish operator-configured names from automatic fallback.
var NodeNameSource = NodeNameSourceHostname

var NodeNameManuallyConfigured bool

var requestInterval int
var RequestInterval time.Duration

var SyncFrequency int // unit is second

var BatchUpdateEnabled = false
var BatchUpdateInterval int

var RelayTimeout int // unit is second

var RelayIdleConnTimeout int // unit is second

// RelayResponseHeaderTimeout limits how long the relay transport waits for the
// upstream response headers after the request has been fully written.
// 0 disables it (previous behaviour: wait forever).
//
// Note this is NOT the same as RelayTimeout (http.Client.Timeout), which covers
// the whole response read and therefore breaks legitimate long streaming calls.
// ResponseHeaderTimeout only bounds the wait for the response headers; once the
// headers arrive, streaming is unaffected.
var RelayResponseHeaderTimeout int // unit is second
var RelayMaxIdleConns int
var RelayMaxIdleConnsPerHost int

var GeminiSafetySetting string

// https://docs.cohere.com/docs/safety-modes Type; NONE/CONTEXTUAL/STRICT
var CohereSafetySetting string

const (
	RequestIdKey         = "X-Oneapi-Request-Id"
	UpstreamRequestIdKey = "X-Upstream-Request-Id"
)

const (
	RoleGuestUser  = 0
	RoleCommonUser = 1
	RoleAdminUser  = 10
	RoleRootUser   = 100
)

func IsValidateRole(role int) bool {
	return role == RoleGuestUser || role == RoleCommonUser || role == RoleAdminUser || role == RoleRootUser
}

var (
	FileUploadPermission    = RoleGuestUser
	FileDownloadPermission  = RoleGuestUser
	ImageUploadPermission   = RoleGuestUser
	ImageDownloadPermission = RoleGuestUser
)

// All duration's unit is seconds
// Shouldn't larger then RateLimitKeyExpirationDuration
var (
	GlobalApiRateLimitEnable   bool
	GlobalApiRateLimitNum      int
	GlobalApiRateLimitDuration int64

	GlobalWebRateLimitEnable   bool
	GlobalWebRateLimitNum      int
	GlobalWebRateLimitDuration int64

	GlobalStaticRateLimitEnable   bool
	GlobalStaticRateLimitNum      int
	GlobalStaticRateLimitDuration int64

	CriticalRateLimitEnable   bool
	CriticalRateLimitNum            = 20
	CriticalRateLimitDuration int64 = 20 * 60

	UploadRateLimitNum            = 10
	UploadRateLimitDuration int64 = 60

	DownloadRateLimitNum            = 10
	DownloadRateLimitDuration int64 = 60

	// Per-user search rate limit (applies after authentication, keyed by user ID)
	SearchRateLimitEnable         = true
	SearchRateLimitNum            = 10
	SearchRateLimitDuration int64 = 60
)

var RateLimitKeyExpirationDuration = 20 * time.Minute

const (
	UserStatusEnabled  = 1 // don't use 0, 0 is the default value!
	UserStatusDisabled = 2 // also don't use 0
)

const (
	TokenStatusEnabled   = 1 // don't use 0, 0 is the default value!
	TokenStatusDisabled  = 2 // also don't use 0
	TokenStatusExpired   = 3
	TokenStatusExhausted = 4
)

const (
	RedemptionCodeStatusEnabled  = 1 // don't use 0, 0 is the default value!
	RedemptionCodeStatusDisabled = 2 // also don't use 0
	RedemptionCodeStatusUsed     = 3 // also don't use 0
)

const (
	ChannelStatusUnknown          = 0
	ChannelStatusEnabled          = 1 // don't use 0, 0 is the default value!
	ChannelStatusManuallyDisabled = 2 // also don't use 0
	ChannelStatusAutoDisabled     = 3
)

const (
	TopUpStatusPending = "pending"
	TopUpStatusSuccess = "success"
	TopUpStatusFailed  = "failed"
	TopUpStatusExpired = "expired"
)
