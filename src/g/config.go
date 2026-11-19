package g

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"github.com/BurntSushi/toml"
	"github.com/cihub/seelog"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	Root           string
	Cfg            Config
	CfgLock        sync.RWMutex
	AlertStatus    map[string]bool
	AlertLock      sync.RWMutex
	AuthUserIpMap  map[string]bool
	AuthAgentIpMap map[string]bool
	AuthLock       sync.RWMutex
	ToolLimit      map[string]int
	ToolLimitLock  sync.Mutex
	Db             *sql.DB
	DLock          sync.Mutex
)

func IsExist(fp string) bool {
	_, err := os.Stat(fp)
	return err == nil || os.IsExist(err)
}

func ReadConfig(filename string) Config {
	config := Config{}
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal("Config Not Found!")
	}
	defer file.Close()
	_, err = toml.DecodeReader(file, &config)
	if err != nil {
		log.Fatal(err)
	}
	return config
}

func GetRoot() string {
	//return "D:\\gopath\\src\\github.com\\smartping\\smartping"
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal("Get Root Path Error:", err)
	}
	dirctory := strings.Replace(dir, "\\", "/", -1)
	runes := []rune(dirctory)
	l := 0 + strings.LastIndex(dirctory, "/")
	if l > len(runes) {
		l = len(runes)
	}
	return string(runes[0:l])
}

func ParseConfig(ver string) {
	Root = GetRoot()
	//config.toml 是通用配置；config-base.toml 是通用配置出厂模板。
	//首次启动（config.toml 不存在）时自动从模板复制生成，此后所有修改
	//只作用于 config.toml。
	if !IsExist(Root + "/conf/" + "config.toml") {
		if !IsExist(Root + "/conf/" + "config-base.toml") {
			log.Fatalln("[Fault]config file:", Root+"/conf/"+"config(-base).toml", "both not existent.")
		}
		src, err := os.Open(Root + "/conf/" + "config-base.toml")
		if err != nil {
			log.Fatalln("[Fault]config-base.toml open error.")
		}
		defer src.Close()
		dst, err := os.OpenFile(Root+"/conf/"+"config.toml", os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			log.Fatalln("[Fault]config.toml create error.")
		}
		defer dst.Close()
		io.Copy(dst, src)
		seelog.Info("config.toml not found, generated from config-base.toml")
	}
	logger, err := seelog.LoggerFromConfigAsFile(Root + "/conf/" + "seelog.xml")
	if err != nil {
		log.Fatalln("[Fault]log config open fail .", err)
	}
	seelog.ReplaceLogger(logger)
	Cfg = ReadConfig(Root + "/conf/" + "config.toml")
	if Cfg.Name == "" {
		Cfg.Name, _ = os.Hostname()
	}
	if Cfg.Addr == "" {
		Cfg.Addr = "127.0.0.1"
	}
	Cfg.Ver = ver
	loadTargets()
	if !IsExist(Root + "/db/" + "database.db") {
		if !IsExist(Root + "/db/" + "database-base.db") {
			log.Fatalln("[Fault]db file:", Root+"/db/"+"database(-base).db", "both not existent.")
		}
		src, err := os.Open(Root + "/db/" + "database-base.db")
		if err != nil {
			log.Fatalln("[Fault]db-base file open error.")
		}
		defer src.Close()
		dst, err := os.OpenFile(Root+"/db/"+"database.db", os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			log.Fatalln("[Fault]db-base file copy error.")
		}
		defer dst.Close()
		io.Copy(dst, src)
	}
	seelog.Info("Config loaded")
	Db, err = sql.Open("sqlite", Root+"/db/database.db")
	if err != nil {
		log.Fatalln("[Fault]db open fail .", err)
	}
	// SQLite 单写者模型：连接池限制为 1，从根上避免并发访问导致的
	// "database is locked (SQLITE_BUSY)" 错误
	Db.SetMaxOpenConns(1)
	Db.Exec("PRAGMA journal_mode = WAL;")
	Db.Exec("PRAGMA busy_timeout = 5000;")
	Db.Exec("PRAGMA synchronous = NORMAL;")
	initDbSchema()
	AlertStatus = map[string]bool{}
	ToolLimit = map[string]int{}
	saveAuth()
}

//defaultTargets 是 targets.toml 缺失时的内置默认抓取配置（AliDNS + 本机）。
var defaultTargets = TargetsConfig{
	PingTargets: map[string]string{
		"119.29.29.29": "AliDNS",
		"127.0.0.1":    "本机",
	},
	TcpTargets: map[string]string{
		"119.29.29.29:53": "AliDNS",
		"127.0.0.1:8899":  "本机",
	},
	AlertRules: []AlertRule{
		{Addr: "119.29.29.29", Name: "AliDNS", Thdavgdelay: "200", Thdchecksec: "900", Thdloss: "30", Thdoccnum: "3"},
	},
}

//loadTargets 加载抓取配置（targets.toml）到 Cfg：
//  1. targets.toml 存在 -> 直接读取；
//  2. 不存在但 config.toml 内嵌了旧版抓取配置 -> 迁移到 targets.toml 并清理 config.toml；
//  3. 都没有 -> 使用内置默认并落盘 targets.toml。
func loadTargets() {
	targetsFp := Root + "/conf/" + "targets.toml"
	if IsExist(targetsFp) {
		t := ReadTargets()
		Cfg.PingTargets = t.PingTargets
		Cfg.TcpTargets = t.TcpTargets
		Cfg.AlertRules = t.AlertRules
		return
	}
	//探测 config.toml 中是否残留旧版内嵌抓取配置（Config 的 toml:"-" 会忽略 decode）
	var legacy TargetsConfig
	if _, err := toml.DecodeFile(Root+"/conf/"+"config.toml", &legacy); err != nil {
		log.Fatal(err)
	}
	if len(legacy.PingTargets) > 0 || len(legacy.TcpTargets) > 0 || len(legacy.AlertRules) > 0 {
		Cfg.PingTargets = legacy.PingTargets
		Cfg.TcpTargets = legacy.TcpTargets
		Cfg.AlertRules = legacy.AlertRules
		if err := SaveTargets(TargetsConfig{PingTargets: legacy.PingTargets, TcpTargets: legacy.TcpTargets, AlertRules: legacy.AlertRules}); err != nil {
			log.Fatalln("[Fault]targets.toml write error:", err)
		}
		seelog.Info("targets.toml generated from legacy config.toml (migrated)")
	} else {
		Cfg.PingTargets = defaultTargets.PingTargets
		Cfg.TcpTargets = defaultTargets.TcpTargets
		Cfg.AlertRules = defaultTargets.AlertRules
		if err := SaveTargets(defaultTargets); err != nil {
			log.Fatalln("[Fault]targets.toml write error:", err)
		}
		seelog.Info("targets.toml not found, generated with default targets")
	}
	//重写 config.toml，清理其中内嵌的抓取配置（encode 时 toml:"-" 自动跳过）
	if err := saveBaseConfig(); err != nil {
		log.Fatalln("[Fault]config.toml rewrite error:", err)
	}
}

func SaveCloudConfig(url string) (Config, error) {
	config := Config{}
	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	resp, err := client.Get(url)
	if err != nil {
		return config, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &config)
	if err != nil {
		config.Name = string(body)
		return config, err
	}
	CfgLock.Lock()
	Name := Cfg.Name
	Addr := Cfg.Addr
	Ver := Cfg.Ver
	Password := Cfg.Password
	Port := Cfg.Port
	Endpoint := Cfg.Mode["Endpoint"]
	Cfg = config
	Cfg.Name = Name
	Cfg.Addr = Addr
	Cfg.Ver = Ver
	Cfg.Port = Port
	Cfg.Password = Password
	Cfg.Mode["LastSuccTime"] = time.Now().Format("2006-01-02 15:04:05")
	Cfg.Mode["Status"] = "true"
	Cfg.Mode["Endpoint"] = Endpoint
	Cfg.Mode["Type"] = "cloud"
	CfgLock.Unlock()
	saveAuth()
	return config, nil
}

//configHeader 是写入 config.toml 顶部的说明注释，与 conf/config-base.toml
//保持一致，便于用户理解各配置文件的关系与配置节含义。
const configHeader = `# ============================================================
# SmartPing 通用配置 (TOML)
#
# 本目录下配置文件的关系：
#   - config.toml       通用配置（服务参数）。启动时读取，Web 界面"保存配置"写回。
#   - config-base.toml  通用配置出厂模板。仅当 config.toml 不存在时，首次启动
#                       会自动复制它为 config.toml（用于恢复出厂设置）。
#   - targets.toml      抓取配置（Ping/TcpPing 检测目标与告警规则）。
#                       缺失时首次启动自动生成默认目标（AliDNS + 本机）。
# ============================================================

`

//targetsHeader 是写入 targets.toml 顶部的说明注释。
const targetsHeader = `# ============================================================
# SmartPing 抓取配置 (TOML)
#
# 检测目标与告警规则：
#   [PingTargets] ICMP 检测目标：目标地址 = 显示名称
#   [TcpTargets]  TCP 检测目标：host:port = 显示名称
#   [AlertRules]  拓扑告警规则（目标异常时触发报警的阈值）
# 本文件由 Web 界面"保存配置"写回。
# ============================================================

`

//SaveConfig 将完整配置拆分为两个文件落盘：
//config.toml 写通用配置（toml:"-" 标记的抓取字段自动跳过），
//targets.toml 写抓取配置。
func SaveConfig() error {
	saveAuth()
	if err := saveBaseConfig(); err != nil {
		return err
	}
	return SaveTargets(TargetsConfig{PingTargets: Cfg.PingTargets, TcpTargets: Cfg.TcpTargets, AlertRules: Cfg.AlertRules})
}

//saveBaseConfig 只写通用配置到 config.toml。
func saveBaseConfig() error {
	var out bytes.Buffer
	err := toml.NewEncoder(&out).Encode(Cfg)
	if err != nil {
		seelog.Error("[func:SaveConfig] Toml Encode ", err)
		return err
	}
	err = os.WriteFile(Root+"/conf/"+"config.toml", []byte(configHeader+out.String()), 0644)
	if err != nil {
		seelog.Error("[func:SaveConfig] Config File Write", err)
		return err
	}
	return nil
}

//ReadTargets 读取抓取配置 targets.toml；文件不存在时使用内置默认并落盘。
func ReadTargets() TargetsConfig {
	t := TargetsConfig{}
	fp := Root + "/conf/" + "targets.toml"
	if !IsExist(fp) {
		t = defaultTargets
		if err := SaveTargets(t); err != nil {
			seelog.Error("[func:ReadTargets] write default targets error: ", err)
		}
		seelog.Info("targets.toml not found, generated with default targets")
		return t
	}
	if _, err := toml.DecodeFile(fp, &t); err != nil {
		log.Fatal(err)
	}
	return t
}

//SaveTargets 写抓取配置到 targets.toml。
func SaveTargets(t TargetsConfig) error {
	var out bytes.Buffer
	if err := toml.NewEncoder(&out).Encode(t); err != nil {
		seelog.Error("[func:SaveTargets] Toml Encode ", err)
		return err
	}
	if err := os.WriteFile(Root+"/conf/"+"targets.toml", []byte(targetsHeader+out.String()), 0644); err != nil {
		seelog.Error("[func:SaveTargets] Config File Write", err)
		return err
	}
	return nil
}

func saveAuth() {
	AuthLock.Lock()
	defer AuthLock.Unlock()
	AuthUserIpMap = map[string]bool{}
	AuthAgentIpMap = map[string]bool{}
	for addr := range Cfg.PingTargets {
		AuthAgentIpMap[addr] = true
	}
	for target := range Cfg.TcpTargets {
		//"host:port" 取 host 部分（与前端 splitHostPort 逻辑一致，仅最后一个冒号为分隔）
		if i := strings.LastIndex(target, ":"); i > -1 {
			AuthAgentIpMap[target[:i]] = true
		} else {
			AuthAgentIpMap[target] = true
		}
	}
	Cfg.Authiplist = strings.Replace(Cfg.Authiplist, " ", "", -1)
	if Cfg.Authiplist != "" {
		authiplist := strings.Split(Cfg.Authiplist, ",")
		for _, ip := range authiplist {
			AuthUserIpMap[ip] = true
		}
	}
}

// initDbSchema creates any tables that have been added after the initial
// database-base.db shipped with the project. Existing tables are left
// untouched (CREATE TABLE IF NOT EXISTS).
func initDbSchema() {
	if Db == nil {
		return
	}
	// tcppinglog mirrors pinglog but stores "host:port" as the target so
	// that multiple TCP probes against the same host can be tracked
	// independently.
	_, err := Db.Exec(`CREATE TABLE IF NOT EXISTS [tcppinglog] (
		[id] INTEGER PRIMARY KEY AUTOINCREMENT,
		[logtime] VARCHAR(20),
		[target] VARCHAR(50),
		[maxdelay] VARCHAR(20),
		[mindelay] VARCHAR(20),
		[avgdelay] VARCHAR(20),
		[sendpk] INTEGER,
		[revcpk] INTEGER,
		[losspk] INTEGER
	)`)
	if err != nil {
		seelog.Error("[func:initDbSchema] create tcppinglog table error: ", err)
	}
	// Index for time-range queries used by /api/tcpping.json.
	_, err = Db.Exec(`CREATE INDEX IF NOT EXISTS [idx_tcppinglog_target_logtime] ON [tcppinglog] ([target], [logtime])`)
	if err != nil {
		seelog.Error("[func:initDbSchema] create tcppinglog index error: ", err)
	}
}
