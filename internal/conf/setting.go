// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package conf

import (
	"bytes"
	_ "embed"
	"strings"
	"time"

	pyroscope "github.com/grafana/pyroscope-go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm/logger"
)

//go:embed config.yaml
var configBytes []byte

type pyroscopeConf struct {
	AppName   string
	Endpoint  string
	AuthToken string
	Logger    string
}

type sentryConf struct {
	Dsn              string
	Debug            bool
	AttachStacktrace bool
	TracesSampleRate float64
	AttachLogrus     bool
	AttachGin        bool
}

type loggerConf struct {
	Level string
}

type loggerFileConf struct {
	SavePath string
	FileName string
	FileExt  string
}

type loggerOtlponf struct {
	Endpoint      string
	Authorization string
	Organization  string
	TraceStream   string
	MetricStream  string
	LogStream     string
	Insecure      bool
}

type httpServerConf struct {
	RunMode      string
	HttpIp       string
	HttpPort     string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type appConf struct {
	RunMode               string
	MaxCommentCount       int64
	MaxWhisperDaily       int64
	MaxCaptchaTimes       int
	DefaultContextTimeout time.Duration
	DefaultPageSize       int
	MaxPageSize           int
}

type cacheConf struct {
	KeyPoolSize          int
	CientSideCacheExpire time.Duration
	UnreadMsgExpire      int64
	UserTweetsExpire     int64
	IndexTweetsExpire    int64
	MessagesExpire       int64
	IndexTrendsExpire    int64
	TweetCommentsExpire  int64
	OnlineUserExpire     int64
	UserInfoExpire       int64
	UserProfileExpire    int64
	UserRelationExpire   int64
}

type eventManagerConf struct {
	MinWorker       int
	MaxTempWorker   int
	MaxEventBuf     int
	MaxTempEventBuf int
	MaxIdleTime     time.Duration
}

type metricManagerConf struct {
	MinWorker       int
	MaxTempWorker   int
	MaxEventBuf     int
	MaxTempEventBuf int
	MaxIdleTime     time.Duration
}

type jobManagerConf struct {
	MaxOnlineInterval     string
	UpdateMetricsInterval string
}

type cacheIndexConf struct {
	MaxUpdateQPS int
	MinWorker    int
}

type simpleCacheIndexConf struct {
	MaxIndexSize       int
	CheckTickDuration  time.Duration
	ExpireTickDuration time.Duration
}

type bigCacheIndexConf struct {
	MaxIndexPage     int
	HardMaxCacheSize int
	ExpireInSecond   time.Duration
	Verbose          bool
}

type redisCacheIndexConf struct {
	ExpireInSecond time.Duration
	Verbose        bool
}

type alipayConf struct {
	AppID             string
	PrivateKey        string
	RootCertFile      string
	PublicCertFile    string
	AppPublicCertFile string
	InProduction      bool
}

type smsJuheConf struct {
	Gateway string
	Key     string
	TplID   string
	TplVal  string
}

type tweetSearchConf struct {
	MaxUpdateQPS int
	MinWorker    int
}

type meiliConf struct {
	Host   string
	Index  string
	ApiKey string
	Secure bool
}

type databaseConf struct {
	TablePrefix  string
	LogLevel     string
	MaxIdleConns int
	MaxOpenConns int
}

// GetMaxIdleConns 连接池最大空闲连接数，缺省 10
func (s *databaseConf) GetMaxIdleConns() int {
	if s.MaxIdleConns <= 0 {
		return 10
	}
	return s.MaxIdleConns
}

// GetMaxOpenConns 连接池最大打开连接数，缺省 100
func (s *databaseConf) GetMaxOpenConns() int {
	if s.MaxOpenConns <= 0 {
		return 100
	}
	return s.MaxOpenConns
}

type postgresConf map[string]string

type objectStorageConf struct {
	RetainInDays int
	TempDir      string
}

type aliOSSConf struct {
	AccessKeyID     string
	AccessKeySecret string
	Endpoint        string
	Bucket          string
	Domain          string
}

type localossConf struct {
	SavePath string
	Secure   bool
	Bucket   string
	Domain   string
}

type redisConf struct {
	InitAddress      []string
	Username         string
	Password         string
	SelectDB         int
	ConnWriteTimeout time.Duration
}

type adminSettingsConf struct {
	EncryptionKey string
}

type jwtConf struct {
	Secret string
	Issuer string
	Expire time.Duration
}

type AuditConf struct {
	Enabled bool `json:"enabled"`
}

// OperatorConf 默认运维账号配置: 账号不存在时按配置创建，
// 存在时密码以配置为准(不一致则重置)；该段缺省或留空则不干预
type OperatorConf struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type WebProfileConf struct {
	EnableTrendsBar         bool   `json:"enable_trends_bar"`
	AllowTweetAttachment    bool   `json:"allow_tweet_attachment"`
	AllowTweetVideo         bool   `json:"allow_tweet_video"`
	AllowUserRegister       bool   `json:"allow_user_register"`
	AllowPhoneBind          bool   `json:"allow_phone_bind"`
	DefaultTweetMaxLength   int    `json:"default_tweet_max_length"`
	TweetWebEllipsisSize    int    `json:"tweet_web_ellipsis_size"`
	TweetMobileEllipsisSize int    `json:"tweet_mobile_ellipsis_size"`
	DefaultTweetVisibility  string `json:"default_tweet_visibility"`
	DefaultMsgLoopInterval  int    `json:"default_msg_loop_interval"`
	CopyrightTop            string `json:"copyright_top"`
	CopyrightLeft           string `json:"copyright_left"`
	CopyrightLeftLink       string `json:"copyright_left_link"`
	CopyrightRight          string `json:"copyright_right"`
	CopyrightRightLink      string `json:"copyright_right_link"`
}

func (s *httpServerConf) GetReadTimeout() time.Duration {
	return s.ReadTimeout * time.Second
}

func (s *httpServerConf) GetWriteTimeout() time.Duration {
	return s.WriteTimeout * time.Second
}

func (s postgresConf) Dsn() string {
	var params []string
	for k, v := range s {
		if len(v) == 0 {
			continue
		}
		lk := strings.ToLower(k)
		tv := strings.Trim(v, " ")
		switch lk {
		case "schema":
			params = append(params, "search_path="+tv)
		case "applicationname":
			params = append(params, "application_name="+tv)
		default:
			params = append(params, lk+"="+tv)
		}
	}
	return strings.Join(params, " ")
}

func (s *databaseConf) logLevel() logger.LogLevel {
	switch strings.ToLower(s.LogLevel) {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "warn":
		return logger.Warn
	case "info":
		return logger.Info
	default:
		return logger.Error
	}
}

func (s *databaseConf) TableNames() (res TableNameMap) {
	tableNames := []string{
		TableAnouncement,
		TableAnouncementContent,
		TableAttachment,
		TableCaptcha,
		TableComment,
		TableCourse,
		TableCourseGroup,
		TableCourseComment,
		TableCourseCommentContent,
		TableCourseCommentReply,
		TableCommentMetric,
		TableCommentContent,
		TableCommentReply,
		TableFollowing,
		TableMessage,
		TablePost,
		TablePostMetric,
		TablePostByComment,
		TablePostByMedia,
		TablePostCollection,
		TablePostContent,
		TablePostStar,
		TableTag,
		TableUser,
		TableUserRelation,
		TableUserMetric,
	}
	res = make(TableNameMap, len(tableNames))
	for _, name := range tableNames {
		res[name] = s.TablePrefix + name
	}
	return
}

func (s *loggerConf) logLevel() logrus.Level {
	switch strings.ToLower(s.Level) {
	case "panic":
		return logrus.PanicLevel
	case "fatal":
		return logrus.FatalLevel
	case "error":
		return logrus.ErrorLevel
	case "warn", "warning":
		return logrus.WarnLevel
	case "info":
		return logrus.InfoLevel
	case "debug":
		return logrus.DebugLevel
	case "trace":
		return logrus.TraceLevel
	default:
		return logrus.ErrorLevel
	}
}

func (s *objectStorageConf) TempDirSlash() string {
	return strings.Trim(s.TempDir, " /") + "/"
}

func (s *meiliConf) Endpoint() string {
	return endpoint(s.Host, s.Secure)
}

func (s *pyroscopeConf) GetLogger() (logger pyroscope.Logger) {
	switch strings.ToLower(s.Logger) {
	case "standard":
		logger = pyroscope.StandardLogger
	case "logrus":
		logger = logrus.StandardLogger()
	}
	return
}

func endpoint(host string, secure bool) string {
	schema := "http"
	if secure {
		schema = "https"
	}
	return schema + "://" + host
}

func newViper() (*viper.Viper, error) {
	vp := viper.New()
	vp.SetConfigName("config")
	vp.AddConfigPath(".")
	vp.AddConfigPath("custom/")
	vp.SetConfigType("yaml")
	err := vp.ReadConfig(bytes.NewReader(configBytes))
	if err != nil {
		return nil, err
	}
	if err = vp.MergeInConfig(); err != nil {
		return nil, err
	}
	return vp, nil
}

func featuresInfoFrom(vp *viper.Viper, k string) (map[string][]string, map[string]string) {
	sub := vp.Sub(k)
	keys := sub.AllKeys()

	suites := make(map[string][]string)
	kv := make(map[string]string, len(keys))
	for _, key := range sub.AllKeys() {
		val := sub.Get(key)
		switch v := val.(type) {
		case string:
			kv[key] = v
		case []any:
			suites[key] = sub.GetStringSlice(key)
		}
	}
	return suites, kv
}
