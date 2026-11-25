# packages/platform/store.go

阶段01：[功能讲义](../01-foundation.md) · [分步编写](../write/031-store.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`Config`、`ExportConfig`、`ResourceConfig`、`ResourceOSSConfig`、`SearchConfig`、`SMTPConfig`、`RedisConfig`、`LoadConfig`、`Store`、`OpenStore`、`CheckReady`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Config每个进程只读取自己的DSN与证书，不把迁移/root账号注入应用服务。
type Config struct {
	Role                         string            `json:"role"`
	Host                         string            `json:"host"`
	HTTPPort                     int               `json:"httpPort"`
	RPCPort                      int               `json:"rpcPort"`
	DSN                          string            `json:"dsn"`
	CertFile                     string            `json:"certFile"`
	KeyFile                      string            `json:"keyFile"`
	CAFile                       string            `json:"caFile"`
	ExpectedSQLSHA               string            `json:"expectedSQLSHA"`
	ExpectedAdditionalMigrations map[int]string    `json:"expectedAdditionalMigrations"`
	Origin                       string            `json:"origin"`
	Root                         string            `json:"root"`
	Peers                        map[string]string `json:"peers"`
	Redis                        *RedisConfig      `json:"redis"`
	SMTP                         *SMTPConfig       `json:"smtp"`
	ChallengeKey                 string            `json:"challengeKey"`
	Search                       *SearchConfig     `json:"search"`
	Resources                    *ResourceConfig   `json:"resources"`
	Exports                      *ExportConfig     `json:"exports"`
}

// 转换程序路径显式装配，业务层不依赖开发者机器的安装目录。
type ExportConfig struct {
	Root    string `json:"root"`
	Pandoc  string `json:"pandoc"`
	Node    string `json:"node"`
	Browser string `json:"browser"`
	// 0沿用默认单worker；并发及临时磁盘总预算由文档服务一起校验。
	Workers int `json:"workers"`
}
type ResourceConfig struct {
	Root             string             `json:"root"`
	OSS              *ResourceOSSConfig `json:"oss,omitempty"`
	SpaceBytes       uint64             `json:"spaceBytes"`
	UploadBytes      uint64             `json:"uploadBytes"`
	TemporaryBytes   uint64             `json:"temporaryBytes"`
	UploadTTLSeconds int                `json:"uploadTTLSeconds"`
}

// OSS 只替换对象 IO；Root 仍是本机共享锁、校验临时文件与既有本地对象的目录。
// 私密配置只保存凭据文件路径，AK/SK 不进入服务配置或日志。
type ResourceOSSConfig struct {
	Region          string `json:"region"`
	Endpoint        string `json:"endpoint"`
	Bucket          string `json:"bucket"`
	Prefix          string `json:"prefix"`
	CredentialsFile string `json:"credentialsFile"`
}
type SearchConfig struct {
	Owner            string   `json:"owner"`
	Brokers          []string `json:"brokers"`
	ElasticEndpoint  string   `json:"elasticEndpoint"`
	Topic            string   `json:"topic"`
	Group            string   `json:"group"`
	Alias            string   `json:"alias"`
	InitialIndex     string   `json:"initialIndex"`
	CursorKey        string   `json:"cursorKey"`
	CursorTTLSeconds int      `json:"cursorTTLSeconds"`
}
type SMTPConfig struct {
	Address string `json:"address"`
	From    string `json:"from"`
	// 空Security只兼容旧的数值loopback开发收件器；远程必须明确选择tls或starttls。
	// 授权码只在私密JSON文件中，配置和HTTP响应均不携带用户名/密码。
	Security        string `json:"security,omitempty"`
	CredentialsFile string `json:"credentialsFile,omitempty"`
	CAFile          string `json:"caFile,omitempty"`
}
type RedisConfig struct {
	Host     string `json:"host"`
	User     string `json:"user"`
	Password string `json:"password"`
}

func LoadConfig(file string) (Config, error) {
	var cfg Config
	raw, err := os.ReadFile(file)
	if err != nil {
		return cfg, errors.New("SERVICE_CONFIG_UNAVAILABLE")
	}
	if json.Unmarshal(raw, &cfg) != nil {
		return cfg, errors.New("SERVICE_CONFIG_INVALID")
	}
	return cfg, nil
}

type Store struct {
	Context    context.Context
	ReadyExtra func(context.Context) error
	// 仅由启动装配登记已接管的持久流程；其事实完整性由 ReadyExtra 核验，未知种类仍拒绝。
	ManagedTasks                 []string
	Role                         string
	Conn                         sqlx.SqlConn
	DB                           *sql.DB
	ExpectedSQLSHA               string
	ExpectedAdditionalMigrations map[int]string
}

func OpenStore(cfg Config) (*Store, error) {
	// go-zero 的 NewMysql 按 DSN 共享全局连接池；这里由服务实例拥有池的生命周期，
	// 避免一个实例退出后把同 DSN 的新实例也变成“连接池已关闭”。查询仍使用 go-zero sqlx。
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, errors.New("DATABASE_UNAVAILABLE")
	}
	conn := sqlx.NewSqlConnFromDB(db)
	db.SetMaxOpenConns(8)
	// 后台领取者会短时同时使用8个连接。空闲上限低于总上限会在每轮归还时
	// 主动关闭健康连接，下一轮重新建连，长期耗尽Windows临时TCP端口。
	// 保留同一个有界池的全部连接，不增加并发上限，也不改变查询或事务期限。
	db.SetMaxIdleConns(8)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, done := context.WithTimeout(context.Background(), 2*time.Second)
	defer done()
	if db.PingContext(ctx) != nil {
		_ = db.Close()
		return nil, errors.New("DATABASE_UNAVAILABLE")
	}
	return &Store{
		Context:                      context.Background(),
		Role:                         cfg.Role,
		Conn:                         conn,
		DB:                           db,
		ExpectedSQLSHA:               cfg.ExpectedSQLSHA,
		ExpectedAdditionalMigrations: maps.Clone(cfg.ExpectedAdditionalMigrations),
	}, nil
}
func (s *Store) CheckReady(ctx context.Context) error {
	// 连通不代表可接业务：版本、原始迁移摘要和恢复事实必须同时成立。
	var version int
	var checksum string
	rows, err := s.DB.QueryContext(ctx, "SELECT version,LOWER(HEX(checksum)) FROM schema_migration")
	if err != nil {
		return errors.New("DATABASE_UNAVAILABLE")
	}
	defer rows.Close()
	installed := map[int]string{}
	for rows.Next() {
		if rows.Scan(&version, &checksum) != nil {
			return errors.New("SCHEMA_VERSION_MISMATCH")
		}
		if _, duplicate := installed[version]; duplicate {
			return errors.New("SCHEMA_VERSION_MISMATCH")
		}
		installed[version] = checksum
	}
	if rows.Err() != nil {
		return errors.New("DATABASE_UNAVAILABLE")
	}
	if !matchesSchemaMigrations(s.ExpectedSQLSHA, s.ExpectedAdditionalMigrations, installed) {
		return errors.New("SCHEMA_VERSION_MISMATCH")
	}
	var pending int
	query := "SELECT COUNT(*) FROM durable_task WHERE state NOT IN ('SUCCEEDED','REJECTED')"
	if s.ExpectedAdditionalMigrations[2] != "" {
		// 002提供覆盖索引；只读state/kind，容量写入不能迫使基础检查载入JSON聚簇页。
		query = "SELECT COUNT(*) FROM durable_task FORCE INDEX (task_pending) WHERE state NOT IN ('SUCCEEDED','REJECTED')"
	}
	var excluded []string
	if s.Role == "document" && s.ReadyExtra != nil {
		excluded = append(excluded, "OUTPUT_LEASE")
	} // 当前输出由实例恢复屏障核验，不能误当未恢复业务任务。
	if len(s.ManagedTasks) > 0 && s.ReadyExtra == nil {
		return errors.New("RECOVERY_BARRIER_REQUIRED")
	}
	excluded = append(excluded, s.ManagedTasks...)
	var args []any
	if len(excluded) > 0 {
		query += " AND kind NOT IN (" + strings.TrimSuffix(strings.Repeat("?,", len(excluded)), ",") + ")"
		for _, kind := range excluded {
			args = append(args, kind)
		}
	}
	if s.DB.QueryRowContext(ctx, query, args...).Scan(&pending) != nil {
		return errors.New("DATABASE_UNAVAILABLE")
	}
	// 未接管流程保持503。已装配恢复器允许正常排队，但最后仍须通过各自事实屏障。
	if pending != 0 {
		return errors.New("RECOVERY_REQUIRED")
	}
	// search没有gate表；其他服务的冻结事实在恢复器实现前必须保持不可用。
	if s.Role != "search" {
		gateQuery := "SELECT COUNT(*) FROM access_gate g LEFT JOIN authorization_change c ON c.revocation_id=g.last_revocation_id AND c.scope_key=g.scope_key AND c.epoch>=g.max_epoch WHERE g.state='BLOCKED' AND (c.state IS NULL OR c.state<>'SUCCEEDED')"
		if s.ExpectedAdditionalMigrations[2] != "" {
			gateQuery = "SELECT COUNT(*) FROM access_gate g FORCE INDEX (gate_recovery) LEFT JOIN authorization_change c FORCE INDEX (auth_recovery) ON c.revocation_id=g.last_revocation_id AND c.scope_key=g.scope_key AND c.epoch>=g.max_epoch WHERE g.state='BLOCKED' AND (c.state IS NULL OR c.state<>'SUCCEEDED')"
		}
		if s.DB.QueryRowContext(ctx, gateQuery).Scan(&pending) != nil {
			return errors.New("DATABASE_UNAVAILABLE")
		}
		if pending != 0 {
			return errors.New("GATE_RECOVERY_REQUIRED")
		}
	}
	if s.ReadyExtra != nil {
		return s.ReadyExtra(ctx)
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`a6970e02d5fdc562a04c0fafd4222823559911e572b60db5b805fa9b0dff0942`。[参考实现](../../../../packages/platform/store.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
