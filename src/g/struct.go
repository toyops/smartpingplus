package g

//Config 是完整配置（内存与 API 契约）。
//落盘时拆分为两个文件：config.toml（通用配置）+ targets.toml（抓取配置）。
//带 toml:"-" 标记的字段属于抓取配置，写 config.toml 时会被跳过。
type Config struct {
	Ver        string
	Port       int
	Name       string
	Addr       string
	Mode       map[string]string
	Base       map[string]int
	Topology   map[string]string
	Alert      map[string]string
	//PingTargets 是 ICMP 检测目标：目标地址 -> 显示名称（存于 targets.toml）
	PingTargets map[string]string `toml:"-"`
	//TcpTargets 是 TCP 检测目标："host:port" -> 显示名称（存于 targets.toml）
	TcpTargets map[string]string `toml:"-"`
	//AlertRules 是拓扑告警规则列表（目标节点异常时触发报警，存于 targets.toml）
	AlertRules []AlertRule `toml:"-"`
	Chinamap   map[string]map[string][]string
	Toollimit  int
	Authiplist string
	Password   string
}

//AlertRule 定义拓扑告警强类型规则
type AlertRule struct {
	Addr        string `json:"Addr" toml:"Addr"`
	Name        string `json:"Name" toml:"Name"`
	Thdavgdelay string `json:"Thdavgdelay" toml:"Thdavgdelay"`
	Thdchecksec string `json:"Thdchecksec" toml:"Thdchecksec"`
	Thdloss     string `json:"Thdloss" toml:"Thdloss"`
	Thdoccnum   string `json:"Thdoccnum" toml:"Thdoccnum"`
}

//TargetsConfig 是抓取配置（targets.toml 的落盘结构）。
type TargetsConfig struct {
	//PingTargets 是 ICMP 检测目标：目标地址 -> 显示名称
	PingTargets map[string]string
	//TcpTargets 是 TCP 检测目标："host:port" -> 显示名称
	TcpTargets map[string]string
	//AlertRules 是拓扑告警规则列表（目标节点异常时触发报警）
	AlertRules []AlertRule
}

//Ping Struct
type PingSt struct {
	SendPk   int
	RevcPk   int
	LossPk   int
	MinDelay float64
	AvgDelay float64
	MaxDelay float64
}

//Ping mini graph Struct
type PingStMini struct {
	Lastcheck []string `json:"lastcheck"`
	LossPk    []string `json:"losspk"`
	AvgDelay  []string `json:"avgdelay"`
}

type PingLog struct {
	Logtime  string
	Maxdelay string
	Mindelay string
	Avgdelay string
	Losspk   string
}

type AlertLog struct {
	Logtime    string
	Targetip   string
	Targetname string
	Tracert    string
	Fromip     string
	Fromname   string
}

type ChinaMp struct {
	Text     string              `json:"text"`
	Subtext  string              `json:"subtext"`
	Avgdelay map[string][]MapVal `json:"avgdelay"`
}

type MapVal struct {
	Value float64 `json:"value"`
	Name  string  `json:"name"`
}

type ToolsRes struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Ip     string `json:"ip"`
	Ping   PingSt `json:"ping"`
}
