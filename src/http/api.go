package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/cihub/seelog"
	"github.com/smartping/smartping/src/funcs"
	"github.com/smartping/smartping/src/g"
	"github.com/smartping/smartping/src/nettools"
	"github.com/wcharczuk/go-chart"
	"github.com/wcharczuk/go-chart/drawing"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// queryProbeTimeSeries 统一查询 pinglog / tcppinglog 时序数据并按分钟桶对齐填充
func queryProbeTimeSeries(tableName, target, reqStartTime, reqEndTime string) map[string][]string {
	// Whitelist table names to prevent any injection risk
	if tableName != "pinglog" && tableName != "tcppinglog" {
		seelog.Error("[queryProbeTimeSeries] invalid tableName: ", tableName)
		return map[string][]string{}
	}
	var timeStart int64
	var timeEnd int64
	var timeStartStr string
	var timeEndStr string

	if reqStartTime != "" && reqEndTime != "" {
		tms, err := time.ParseInLocation("2006-01-02 15:04", reqStartTime, time.Local)
		if err != nil {
			timeStart = time.Now().Unix() - 2*60*60
			timeStartStr = time.Unix(timeStart, 0).Format("2006-01-02 15:04")
		} else {
			timeStart = tms.Unix()
			timeStartStr = reqStartTime
		}
		tmn, err := time.ParseInLocation("2006-01-02 15:04", reqEndTime, time.Local)
		if err != nil {
			timeEnd = time.Now().Unix()
			timeEndStr = time.Unix(timeEnd, 0).Format("2006-01-02 15:04")
		} else {
			timeEnd = tmn.Unix()
			timeEndStr = reqEndTime
		}
	} else {
		timeStart = time.Now().Unix() - 2*60*60
		timeStartStr = time.Unix(timeStart, 0).Format("2006-01-02 15:04")
		timeEnd = time.Now().Unix()
		timeEndStr = time.Unix(timeEnd, 0).Format("2006-01-02 15:04")
	}

	cnt := int((timeEnd - timeStart) / 60)
	if cnt < 0 {
		cnt = 0
	}
	var lastcheck []string
	var maxdelay []string
	var mindelay []string
	var avgdelay []string
	var losspk []string

	for i := 0; i <= cnt; i++ {
		ntime := time.Unix(timeStart+int64(i*60), 0).Format("2006-01-02 15:04")
		lastcheck = append(lastcheck, ntime)
		maxdelay = append(maxdelay, "0")
		mindelay = append(mindelay, "0")
		avgdelay = append(avgdelay, "0")
		losspk = append(losspk, "0")
	}

	querySql := fmt.Sprintf("SELECT logtime,maxdelay,mindelay,avgdelay,losspk FROM `%s` WHERE target=? AND logtime BETWEEN ? AND ?", tableName)
	rows, err := g.Db.Query(querySql, target, timeStartStr, timeEndStr)
	seelog.Debug("[queryProbeTimeSeries] Query ", querySql, " target=", target, " start=", timeStartStr, " end=", timeEndStr)
	if err != nil {
		seelog.Error("[queryProbeTimeSeries] Query ", err)
	} else {
		for rows.Next() {
			l := new(g.PingLog)
			err := rows.Scan(&l.Logtime, &l.Maxdelay, &l.Mindelay, &l.Avgdelay, &l.Losspk)
			if err != nil {
				seelog.Error("[queryProbeTimeSeries] Rows", err)
				continue
			}
			for n, v := range lastcheck {
				if v == l.Logtime {
					maxdelay[n] = l.Maxdelay
					mindelay[n] = l.Mindelay
					avgdelay[n] = l.Avgdelay
					losspk[n] = l.Losspk
					break
				}
			}
		}
		rows.Close()
	}

	return map[string][]string{
		"lastcheck": lastcheck,
		"maxdelay":  maxdelay,
		"mindelay":  mindelay,
		"avgdelay":  avgdelay,
		"losspk":    losspk,
	}
}

func configApiRoutes() {

	//配置文件API
	http.HandleFunc("/api/config.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, 401)
			return
		}
		r.ParseForm()
		g.CfgLock.RLock()
		cfgJson, _ := json.Marshal(g.Cfg)
		isAgent := AuthAgentIp(r.RemoteAddr, false)
		g.CfgLock.RUnlock()
		nconf := g.Config{}
		json.Unmarshal(cfgJson, &nconf)
		nconf.Password = ""
		if !isAgent {
			if nconf.Alert["SendEmailPassword"] != "" {
				nconf.Alert["SendEmailPassword"] = "samepasswordasbefore"
			}
		}
		//fmt.Print(g.Cfg.Alert["SendEmailPassword"])
		onconf, _ := json.Marshal(nconf)
		var out bytes.Buffer
		json.Indent(&out, onconf, "", "\t")
		o := out.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, o)
	})

	//Ping数据API
	http.HandleFunc("/api/ping.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, 401)
			return
		}
		r.ParseForm()
		if len(r.Form["ip"]) == 0 {
			http.Error(w, "Missing Param !", 406)
			return
		}
		startTime := ""
		endTime := ""
		if len(r.Form["starttime"]) > 0 {
			startTime = r.Form["starttime"][0]
		}
		if len(r.Form["endtime"]) > 0 {
			endTime = r.Form["endtime"][0]
		}
		preout := queryProbeTimeSeries("pinglog", r.Form["ip"][0], startTime, endTime)
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, preout)
	})

	//TcpPing数据API
	http.HandleFunc("/api/tcpping.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, 401)
			return
		}
		r.ParseForm()
		if len(r.Form["ip"]) == 0 {
			http.Error(w, "Missing Param !", 406)
			return
		}
		startTime := ""
		endTime := ""
		if len(r.Form["starttime"]) > 0 {
			startTime = r.Form["starttime"][0]
		}
		if len(r.Form["endtime"]) > 0 {
			endTime = r.Form["endtime"][0]
		}
		preout := queryProbeTimeSeries("tcppinglog", r.Form["ip"][0], startTime, endTime)
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, preout)
	})

	//Ping拓扑API
	http.HandleFunc("/api/topology.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, 401)
			return
		}
		preout := make(map[string]string)
		g.CfgLock.RLock()
		alertRules := make([]g.AlertRule, len(g.Cfg.AlertRules))
		copy(alertRules, g.Cfg.AlertRules)
		g.CfgLock.RUnlock()
		for _, v := range alertRules {
			if funcs.CheckAlertStatus(v) {
				preout[v.Addr] = "true"
			} else {
				preout[v.Addr] = "false"
			}
		}
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, preout)
	})

	//报警API
	http.HandleFunc("/api/alert.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, 401)
			return
		}
		type DateList struct {
			Ldate string
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		r.ParseForm()
		dtb := time.Unix(time.Now().Unix(), 0).Format("2006-01-02")
		if len(r.Form["date"]) > 0 {
			dtb = strings.Replace(r.Form["date"][0], "alertlog-", "", -1)
		}
		listpreout := []string{}
		datapreout := []g.AlertLog{}
		querySql := "select date(logtime) as ldate from alertlog group by date(logtime) order by logtime desc"
		rows, err := g.Db.Query(querySql)
		seelog.Debug("[func:/api/alert.json] Query ", querySql)
		if err != nil {
			seelog.Error("[func:/api/alert.json] Query ", err)
		} else {
			for rows.Next() {
				l := new(DateList)
				err := rows.Scan(&l.Ldate)
				if err != nil {
					seelog.Error("[/api/alert.json] Rows", err)
					continue
				}
				listpreout = append(listpreout, l.Ldate)
			}
			rows.Close()
		}
		querySql = "SELECT logtime,targetname,targetip,tracert FROM alertlog WHERE logtime BETWEEN ? AND ?"
		timeStartAlert := dtb + " 00:00:00"
		timeEndAlert := dtb + " 23:59:59"
		rows, err = g.Db.Query(querySql, timeStartAlert, timeEndAlert)
		seelog.Debug("[func:/api/alert.json] Query ", querySql, " start=", timeStartAlert, " end=", timeEndAlert)
		if err != nil {
			seelog.Error("[func:/api/alert.json] Query ", err)
		} else {
			for rows.Next() {
				l := new(g.AlertLog)
				err := rows.Scan(&l.Logtime, &l.Targetname, &l.Targetip, &l.Tracert)
				l.Fromname = g.Cfg.Name
				l.Fromip = g.Cfg.Addr
				if err != nil {
					seelog.Error("[/api/alert.json] Rows", err)
					continue
				}
				datapreout = append(datapreout, *l)
			}
			rows.Close()
		}
		lout, _ := json.Marshal(listpreout)
		dout, _ := json.Marshal(datapreout)
		fmt.Fprintln(w, "["+string(lout)+","+string(dout)+"]")
	})

	//全国延迟API
	http.HandleFunc("/api/mapping.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, 401)
			return
		}
		m, _ := time.ParseDuration("-1m")
		dataKey := time.Now().Add(m).Format("2006-01-02 15:04")
		r.ParseForm()
		if len(r.Form["d"]) > 0 {
			dataKey = r.Form["d"][0]
		}
		type Mapjson struct {
			Mapjson string
		}
		chinaMp := g.ChinaMp{}
		chinaMp.Text = g.Cfg.Name
		chinaMp.Subtext = dataKey
		chinaMp.Avgdelay = map[string][]g.MapVal{}
		chinaMp.Avgdelay["ctcc"] = []g.MapVal{}
		chinaMp.Avgdelay["cucc"] = []g.MapVal{}
		chinaMp.Avgdelay["cmcc"] = []g.MapVal{}
		querySql := "SELECT mapjson FROM mappinglog WHERE logtime = ?"
		rows, err := g.Db.Query(querySql, dataKey)
		seelog.Debug("[func:/api/mapping.json] Query ", querySql, " logtime=", dataKey)
		if err != nil {
			seelog.Error("[func:/api/mapping.json] Query ", err)
		} else {
			for rows.Next() {
				l := new(Mapjson)
				err := rows.Scan(&l.Mapjson)
				if err != nil {
					seelog.Error("[/api/mapping.json] Rows", err)
					continue
				}
				json.Unmarshal([]byte(l.Mapjson), &chinaMp.Avgdelay)
			}
			rows.Close()
		}
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, chinaMp)
	})

	//检测工具API
	http.HandleFunc("/api/tools.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, 401)
			return
		}
		preout := g.ToolsRes{}
		preout.Status = "false"
		r.ParseForm()
		if len(r.Form["t"]) == 0 {
			preout.Error = "target empty!"
			RenderJson(w, preout)
			return
		}
		nowtime := int(time.Now().Unix())
		g.ToolLimitLock.Lock()
		lastTime, ok := g.ToolLimit[r.RemoteAddr]
		if ok && (nowtime-lastTime) <= g.Cfg.Toollimit {
			g.ToolLimitLock.Unlock()
			preout.Error = "Time Limit Exceeded!"
			RenderJson(w, preout)
			return
		}
		g.ToolLimit[r.RemoteAddr] = nowtime
		g.ToolLimitLock.Unlock()
		target := strings.Replace(strings.Replace(r.Form["t"][0], "https://", "", -1), "http://", "", -1)
		//mode=tcp triggers a TCP-handshake ping instead of ICMP. The
		//target may be "host:port"; when no port is supplied, port 80
		//is used. An optional "port" form param overrides the port.
		mode := ""
		if len(r.Form["mode"]) > 0 {
			mode = r.Form["mode"][0]
		}
		preout.Ping = g.PingSt{}
		preout.Ping.MinDelay = -1
		lossPK := 0
		if mode == "tcp" {
			host, port := nettools.ParseTcpTarget(target)
			if len(r.Form["port"]) > 0 {
				if p, perr := strconv.Atoi(r.Form["port"][0]); perr == nil && p > 0 && p < 65536 {
					port = p
				}
			}
			ipaddr, err := net.ResolveIPAddr("ip", host)
			if err != nil {
				preout.Error = "Unable to resolve destination host"
				RenderJson(w, preout)
				return
			}
			preout.Ip = ipaddr.String() + ":" + strconv.Itoa(port)
			var channel chan float64 = make(chan float64, 5)
			var wg sync.WaitGroup
			for i := 0; i < 5; i++ {
				wg.Add(1)
				go func() {
					delay, err := nettools.RunTcpPing(ipaddr.IP.String(), port, 3*time.Second)
					if err != nil {
						channel <- -1.00
					} else {
						channel <- delay
					}
					wg.Done()
				}()
				time.Sleep(time.Duration(100 * time.Millisecond))
			}
			wg.Wait()
			for i := 0; i < 5; i++ {
				select {
				case delay := <-channel:
					if delay != -1.00 {
						preout.Ping.AvgDelay = preout.Ping.AvgDelay + delay
						if preout.Ping.MaxDelay < delay {
							preout.Ping.MaxDelay = delay
						}
						if preout.Ping.MinDelay == -1 || preout.Ping.MinDelay > delay {
							preout.Ping.MinDelay = delay
						}
						preout.Ping.RevcPk = preout.Ping.RevcPk + 1
					} else {
						lossPK = lossPK + 1
					}
					preout.Ping.SendPk = preout.Ping.SendPk + 1
					preout.Ping.LossPk = int((float64(lossPK) / float64(preout.Ping.SendPk)) * 100)
				}
			}
			if preout.Ping.RevcPk > 0 {
				preout.Ping.AvgDelay = preout.Ping.AvgDelay / float64(preout.Ping.RevcPk)
			} else {
				preout.Ping.AvgDelay = 3000
				preout.Ping.MinDelay = 3000
				preout.Ping.MaxDelay = 3000
			}
			preout.Status = "true"
			w.Header().Set("Content-Type", "application/json")
			RenderJson(w, preout)
			return
		}
		ipaddr, err := net.ResolveIPAddr("ip", target)
		if err != nil {
			preout.Error = "Unable to resolve destination host"
			RenderJson(w, preout)
			return
		}
		preout.Ip = ipaddr.String()
		var channel chan float64 = make(chan float64, 5)
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				delay, err := nettools.RunPing(ipaddr, 3*time.Second, 64, i)
				if err != nil {
					channel <- -1.00
				} else {
					channel <- delay
				}
				wg.Done()
			}()
			time.Sleep(time.Duration(100 * time.Millisecond))
		}
		wg.Wait()
		for i := 0; i < 5; i++ {
			select {
			case delay := <-channel:
				if delay != -1.00 {
					preout.Ping.AvgDelay = preout.Ping.AvgDelay + delay
					if preout.Ping.MaxDelay < delay {
						preout.Ping.MaxDelay = delay
					}
					if preout.Ping.MinDelay == -1 || preout.Ping.MinDelay > delay {
						preout.Ping.MinDelay = delay
					}
					preout.Ping.RevcPk = preout.Ping.RevcPk + 1
				} else {
					lossPK = lossPK + 1
				}
				preout.Ping.SendPk = preout.Ping.SendPk + 1
				preout.Ping.LossPk = int((float64(lossPK) / float64(preout.Ping.SendPk)) * 100)
			}
		}
		if preout.Ping.RevcPk > 0 {
			preout.Ping.AvgDelay = preout.Ping.AvgDelay / float64(preout.Ping.RevcPk)
		} else {
			preout.Ping.AvgDelay = 3000
			preout.Ping.MinDelay = 3000
			preout.Ping.MaxDelay = 3000
		}
		preout.Status = "true"
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, preout)
	})

	//管理员登录API
	http.HandleFunc("/api/login.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			RenderJson(w, map[string]interface{}{"status": false, "error": "Method not allowed", "info": "Method not allowed"})
			return
		}

		var password string
		if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			var body struct {
				Password string `json:"password"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				password = body.Password
			}
		} else {
			r.ParseForm()
			if len(r.Form["password"]) > 0 {
				password = r.Form["password"][0]
			}
		}

		if password == "" {
			w.WriteHeader(http.StatusBadRequest)
			RenderJson(w, map[string]interface{}{"status": false, "error": "Password required", "info": "密码不能为空"})
			return
		}

		g.CfgLock.RLock()
		storedPassword := g.Cfg.Password
		g.CfgLock.RUnlock()

		if !VerifyPassword(storedPassword, password) {
			w.WriteHeader(http.StatusUnauthorized)
			RenderJson(w, map[string]interface{}{"status": false, "error": "Invalid password", "info": "密码错误"})
			return
		}

		token, err := GenerateToken("admin", defaultTokenTTL)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			RenderJson(w, map[string]interface{}{"status": false, "error": "Failed to generate token", "info": "生成令牌失败"})
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     tokenCookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(defaultTokenTTL.Seconds()),
		})

		RenderJson(w, map[string]interface{}{
			"status":     true,
			"token":      token,
			"expires_in": int(defaultTokenTTL.Seconds()),
		})
	})

	//鉴权状态API
	http.HandleFunc("/api/auth/status.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		authed := IsAuthenticated(r)
		RenderJson(w, map[string]interface{}{
			"authenticated": authed,
			"username":      "admin",
		})
	})

	//登出API
	http.HandleFunc("/api/logout.json", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     tokenCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
		})
		w.Header().Set("Content-Type", "application/json")
		RenderJson(w, map[string]interface{}{
			"status": true,
		})
	})

	//保存配置文件
	http.HandleFunc("/api/saveconfig.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, 401)
			return
		}
		preout := make(map[string]string)
		r.ParseForm()
		preout["status"] = "false"
		// 校验鉴权：优先通过 JWT Token，同时兼容直接输入密码校验
		if !IsAuthenticated(r) {
			if len(r.Form["password"]) == 0 || !VerifyPassword(g.Cfg.Password, r.Form["password"][0]) {
				preout["info"] = "密码错误!"
				RenderJson(w, preout)
				return
			}
		}
		if len(r.Form["config"]) == 0 {
			preout["info"] = "参数错误!"
			RenderJson(w, preout)
			return
		}
		nconfig := g.Config{}
		err := json.Unmarshal([]byte(r.Form["config"][0]), &nconfig)
		if err != nil {
			preout["info"] = "配置文件解析错误!" + err.Error()
			RenderJson(w, preout)
			return
		}
		if nconfig.Name == "" {
			preout["info"] = "本机节点名称为空!"
			RenderJson(w, preout)
			return
		}
		if !ValidIP4(nconfig.Addr) {
			preout["info"] = "非法本机节点IP!"
			RenderJson(w, preout)
			return
		}
		//Base
		if _, ok := nconfig.Base["Timeout"]; !ok || nconfig.Base["Timeout"] <= 0 {
			preout["info"] = "非法超时时间!(>0)"
			RenderJson(w, preout)
			return
		}
		if _, ok := nconfig.Base["Archive"]; !ok || nconfig.Base["Archive"] <= 0 {
			preout["info"] = "非法存档天数!(>0)"
			RenderJson(w, preout)
			return
		}
		if _, ok := nconfig.Base["Refresh"]; !ok || nconfig.Base["Refresh"] <= 0 {
			preout["info"] = "非法刷新频率!(>0)"
			RenderJson(w, preout)
			return
		}
		//Topology
		if _, ok := nconfig.Topology["Tline"]; !ok || nconfig.Topology["Tline"] <= "0" {
			preout["info"] = "非法拓扑连线粗细(>0)"
			RenderJson(w, preout)
			return
		}
		if _, ok := nconfig.Topology["Tsymbolsize"]; !ok || nconfig.Topology["Tsymbolsize"] <= "0" {
			preout["info"] = "非法拓扑形状大小!(>0)"
			RenderJson(w, preout)
			return
		}
		if nconfig.Toollimit < 0 {
			preout["info"] = "非法检测工具限定频率!(>=0)"
			RenderJson(w, preout)
			return
		}
		//PingTargets / TcpTargets（检测目标）
		for target := range nconfig.PingTargets {
			if !ValidHost(target) {
				preout["info"] = "Ping节点测试网络信息错误!(非法 Ping 目标 " + target + ")"
				RenderJson(w, preout)
				return
			}
		}
		for target := range nconfig.TcpTargets {
			host := target
			if i := strings.LastIndex(target, ":"); i > -1 {
				host = target[:i]
			}
			if !ValidHost(host) {
				preout["info"] = "Ping节点测试网络信息错误!(非法 TcpPing 目标 " + target + ")"
				RenderJson(w, preout)
				return
			}
		}
		//AlertRules（拓扑告警规则）
		for _, rule := range nconfig.AlertRules {
			if rule.Thdchecksec == "" {
				preout["info"] = "Ping节点测试网络信息错误!( " + rule.Addr + " 非法拓扑报警规则，秒) "
				RenderJson(w, preout)
				return
			}
			Thdchecksec, err := strconv.Atoi(rule.Thdchecksec)
			if err != nil || Thdchecksec <= 0 {
				preout["info"] = "Ping节点测试网络信息错误!( " + rule.Addr + " 非法拓扑报警规则，>0 秒  ) "
				RenderJson(w, preout)
				return
			}
			if rule.Thdloss == "" {
				preout["info"] = "Ping节点测试网络信息错误!( " + rule.Addr + " 非法拓扑报警规则，%) "
				RenderJson(w, preout)
				return
			}
			Thdloss, err := strconv.Atoi(rule.Thdloss)
			if err != nil || (Thdloss < 0 || Thdloss > 100) {
				preout["info"] = "Ping节点测试网络信息错误!( " + rule.Addr + " 非法拓扑报警规则，0 <= % <=100  ) "
				RenderJson(w, preout)
				return
			}
			if rule.Thdavgdelay == "" {
				preout["info"] = "Ping节点测试网络信息错误!( " + rule.Addr + " 非法拓扑报警规则，ms) "
				RenderJson(w, preout)
				return
			}
			Thdavgdelay, err := strconv.Atoi(rule.Thdavgdelay)
			if err != nil || Thdavgdelay <= 0 {
				preout["info"] = "Ping节点测试网络信息错误!( " + rule.Addr + " 非法拓扑报警规则，> 0 ms  ) "
				RenderJson(w, preout)
				return
			}
			if rule.Thdoccnum == "" {
				preout["info"] = "Ping节点测试网络信息错误!( " + rule.Addr + " 非法拓扑报警规则，次) "
				RenderJson(w, preout)
				return
			}
			Thdoccnum, err := strconv.Atoi(rule.Thdoccnum)
			if err != nil || Thdoccnum <= 0 {
				preout["info"] = "Ping节点测试网络信息错误!( " + rule.Addr + " 非法拓扑报警规则，> 0 次  ) "
				RenderJson(w, preout)
				return
			}
		}
		//ChinaMap
		for _, provVal := range nconfig.Chinamap {
			for _, telcomVal := range provVal {
				for _, ip := range telcomVal {
					if ip != "" && !ValidIP4(ip) {
						preout["info"] = "Mapping Ip illegal!"
						RenderJson(w, preout)
						return
					}
				}
			}
		}
		nconfig.Ver = g.Cfg.Ver
		nconfig.Port = g.Cfg.Port
		// If password was updated, automatically hash it with bcrypt before persisting
		if nconfig.Password != "" && nconfig.Password != g.Cfg.Password && !strings.HasPrefix(nconfig.Password, "$2a$") && !strings.HasPrefix(nconfig.Password, "$2b$") {
			if hashed, err := HashPassword(nconfig.Password); err == nil {
				nconfig.Password = hashed
			}
		} else {
			nconfig.Password = g.Cfg.Password
		}
		if nconfig.Alert["SendEmailPassword"] == "samepasswordasbefore" {
			nconfig.Alert["SendEmailPassword"] = g.Cfg.Alert["SendEmailPassword"]
		}
		g.CfgLock.Lock()
		g.Cfg = nconfig
		g.CfgLock.Unlock()
		saveerr := g.SaveConfig()
		if saveerr != nil {
			preout["info"] = saveerr.Error()
			RenderJson(w, preout)
			return
		}
		preout["status"] = "true"
		RenderJson(w, preout)
	})

	//发送测试邮件
	http.HandleFunc("/api/sendmailtest.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) && !AuthAgentIp(r.RemoteAddr, true) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, 401)
			return
		}
		preout := make(map[string]string)
		r.ParseForm()
		preout["status"] = "false"
		if len(r.Form["EmailHost"]) == 0 {
			preout["info"] = "邮件服务器不能为空!"
			RenderJson(w, preout)
			return
		}
		if len(r.Form["SendEmailAccount"]) == 0 {
			preout["info"] = "发件邮件不能为空!"
			RenderJson(w, preout)
			return
		}
		if len(r.Form["SendEmailPassword"]) == 0 {
			preout["info"] = "发件邮箱密码不能为空!"
			RenderJson(w, preout)
			return
		}
		if len(r.Form["RevcEmailList"]) == 0 {
			preout["info"] = "收件邮箱列表不能为空!"
			RenderJson(w, preout)
			return
		}

		err := funcs.SendMail(r.Form["SendEmailAccount"][0], r.Form["SendEmailPassword"][0], r.Form["EmailHost"][0], r.Form["RevcEmailList"][0], "报警测试邮件 - SmartPing", "报警测试邮件")
		if err != nil {
			preout["info"] = err.Error()
			RenderJson(w, preout)
			return
		}
		preout["status"] = "true"
		RenderJson(w, preout)
	})

	//Ping画图
	http.HandleFunc("/api/graph.png", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, 401)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		r.ParseForm()
		if len(r.Form["g"]) == 0 {
			GraphText(83, 70, "GET PARAM ERROR").Save(w)
			return
		}
		rawUrl := r.Form["g"][0]
		targetUrl, err := ValidateProxyURL(rawUrl)
		if err != nil {
			seelog.Warn("[/api/graph.png] SSRF blocked: ", err)
			GraphText(80, 70, "FORBIDDEN TARGET").Save(w)
			return
		}
		config := g.PingStMini{}
		timeout := time.Duration(time.Duration(g.Cfg.Base["Timeout"]) * time.Second)
		client := SafeProxyClient(timeout)
		resp, err := client.Get(targetUrl.String())
		if err != nil {
			GraphText(80, 70, "REQUEST API ERROR").Save(w)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode == 401 {
			GraphText(80, 70, "401-UNAUTHORIZED").Save(w)
			return
		}
		if resp.StatusCode != 200 {
			GraphText(85, 70, "ERROR CODE "+strconv.Itoa(resp.StatusCode)).Save(w)
			return
		}
		body, err := io.ReadAll(resp.Body)
		err = json.Unmarshal(body, &config)
		if err != nil {
			GraphText(80, 70, "PARSE DATA ERROR").Save(w)
			return
		}
		Xals := []float64{}
		AvgDelay := []float64{}
		LossPk := []float64{}
		Bkg := []float64{}
		MaxDelay := 0.0
		for i := 0; i < len(config.LossPk); i = i + 1 {
			avg, _ := strconv.ParseFloat(config.AvgDelay[i], 64)
			if MaxDelay < avg {
				MaxDelay = avg
			}
			AvgDelay = append(AvgDelay, avg)
			losspk, _ := strconv.ParseFloat(config.LossPk[i], 64)
			LossPk = append(LossPk, losspk)
			Xals = append(Xals, float64(i))
			Bkg = append(Bkg, 100.0)
		}
		graph := chart.Chart{
			Width:  300 * 3,
			Height: 130 * 3,
			Background: chart.Style{
				FillColor: drawing.Color{R: 249, G: 246, B: 241, A: 255},
			},
			XAxis: chart.XAxis{
				Style: chart.Style{
					//Show:     true,
					FontSize: 20,
				},
				TickPosition: chart.TickPositionBetweenTicks,
				ValueFormatter: func(v interface{}) string {
					return config.Lastcheck[int(v.(float64))][11:16]
				},
			},
			YAxis: chart.YAxis{
				Style: chart.Style{
					//Show:     true,
					FontSize: 20,
				},
				Range: &chart.ContinuousRange{
					Min: 0.0,
					Max: 100.0,
				},
				ValueFormatter: func(v interface{}) string {
					if vf, isFloat := v.(float64); isFloat {
						return fmt.Sprintf("%0.0f", vf)
					}
					return ""
				},
			},
			YAxisSecondary: chart.YAxis{
				//NameStyle: chart.StyleShow(),
				Style: chart.Style{
					//Show:     true,
					FontSize: 20,
				},
				Range: &chart.ContinuousRange{
					Min: 0.0,
					Max: MaxDelay + MaxDelay/10,
				},
				ValueFormatter: func(v interface{}) string {
					if vf, isFloat := v.(float64); isFloat {
						return fmt.Sprintf("%0.0f", vf)
					}
					return ""
				},
			},
			Series: []chart.Series{
				chart.ContinuousSeries{
					Style: chart.Style{
						//Show:        true,
						StrokeColor: drawing.Color{R: 249, G: 246, B: 241, A: 255},
						FillColor:   drawing.Color{R: 249, G: 246, B: 241, A: 255},
					},
					XValues: Xals,
					YValues: Bkg,
				},
				chart.ContinuousSeries{
					Style: chart.Style{
						//Show:        true,
						StrokeColor: drawing.Color{R: 0, G: 204, B: 102, A: 200},
						FillColor:   drawing.Color{R: 0, G: 204, B: 102, A: 200},
					},
					XValues: Xals,
					YValues: AvgDelay,
					YAxis:   chart.YAxisSecondary,
				},
				chart.ContinuousSeries{
					Style: chart.Style{
						//Show:        true,
						StrokeColor: drawing.Color{R: 255, G: 0, B: 0, A: 200},
						FillColor:   drawing.Color{R: 255, G: 0, B: 0, A: 200},
					},
					XValues: Xals,
					YValues: LossPk,
				},
			},
		}
		graph.Render(chart.PNG, w)

	})

	//代理访问
	http.HandleFunc("/api/proxy.json", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, 401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		r.ParseForm()
		if len(r.Form["g"]) == 0 {
			o := "Url Param Error!"
			http.Error(w, o, 406)
			return
		}
		to := strconv.Itoa(g.Cfg.Base["Timeout"])
		if len(r.Form["t"]) > 0 {
			to = r.Form["t"][0]
		}
		rawUrl := strings.Replace(strings.Replace(r.Form["g"][0], "%26", "&", -1), " ", "%20", -1)
		targetUrl, err := ValidateProxyURL(rawUrl)
		if err != nil {
			seelog.Warn("[/api/proxy.json] SSRF blocked: ", err)
			http.Error(w, "Proxy target forbidden: "+err.Error(), 403)
			return
		}
		defaultto, err := strconv.Atoi(to)
		if err != nil {
			o := "Timeout Param Error!"
			http.Error(w, o, 406)
			return
		}
		timeout := time.Duration(time.Duration(defaultto) * time.Second)
		client := SafeProxyClient(timeout)
		resp, err := client.Get(targetUrl.String())
		if err != nil {
			o := "Request Remote Data Error:" + err.Error()
			http.Error(w, o, 503)
			return
		}
		defer resp.Body.Close()
		resCode := resp.StatusCode
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			o := "Read Remote Data Error:" + err.Error()
			http.Error(w, o, 503)
			return
		}
		if resCode != 200 {
			o := "Get Remote Data Status Error"
			http.Error(w, o, resCode)
			return
		}
		var out bytes.Buffer
		json.Indent(&out, body, "", "\t")
		o := out.String()
		fmt.Fprintln(w, o)
	})

}
