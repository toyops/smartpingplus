package funcs

import (
	"github.com/cihub/seelog"
	_ "modernc.org/sqlite"
	"github.com/smartping/smartping/src/g"
	"github.com/smartping/smartping/src/nettools"
	"net"
	"strconv"
	"sync"
	"time"
)

// TcpPing runs the TCP-ping probe against all Tcpping targets of the local node.
// It mirrors Ping(): one goroutine per target, then triggers StartAlert.
func TcpPing() {
	var wg sync.WaitGroup
	for target := range g.Cfg.TcpTargets {
		wg.Add(1)
		go TcpPingTask(target, &wg)
	}
	wg.Wait()
	go StartAlert()
}

// TcpPingTask performs 20 TCP handshakes against "host:port" and aggregates the
// statistics, mirroring PingTask in ping.go.
func TcpPingTask(target string, wg *sync.WaitGroup) {
	seelog.Info("Start TcpPing " + target + "..")
	stat := g.PingSt{}
	stat.MinDelay = -1
	lossPK := 0
	host, port := nettools.ParseTcpTarget(target)
	ipaddr, err := net.ResolveIPAddr("ip", host)
	if err == nil {
		for i := 0; i < 20; i++ {
			starttime := time.Now().UnixNano()
			delay, err := nettools.RunTcpPing(ipaddr.IP.String(), port, 3*time.Second)
			if err == nil {
				stat.AvgDelay = stat.AvgDelay + delay
				if stat.MaxDelay < delay {
					stat.MaxDelay = delay
				}
				if stat.MinDelay == -1 || stat.MinDelay > delay {
					stat.MinDelay = delay
				}
				stat.RevcPk = stat.RevcPk + 1
				seelog.Debug("[func:StartTcpPing] ID:", i, " Target:", target)
			} else {
				seelog.Debug("[func:StartTcpPing] ID:", i, " Target:", target, "| err:", err)
				lossPK = lossPK + 1
			}
			stat.SendPk = stat.SendPk + 1
			stat.LossPk = int((float64(lossPK) / float64(stat.SendPk)) * 100)
			duringtime := time.Now().UnixNano() - starttime
			time.Sleep(time.Duration(3000*1000000-duringtime) * time.Nanosecond)
		}
		if stat.RevcPk > 0 {
			stat.AvgDelay = stat.AvgDelay / float64(stat.RevcPk)
		} else {
			stat.AvgDelay = 0.0
		}
		seelog.Debug("[func:TcpPing] Finish Target:", target, " MaxDelay:", stat.MaxDelay, " MinDelay:", stat.MinDelay, " AvgDelay:", stat.AvgDelay, " Revc:", stat.RevcPk, " LossPK:", stat.LossPk)
	} else {
		stat.AvgDelay = 0.00
		stat.MinDelay = 0.00
		stat.MaxDelay = 0.00
		stat.SendPk = 0
		stat.RevcPk = 0
		stat.LossPk = 100
		seelog.Debug("[func:TcpPing] Finish Target:", target, " Unable to resolve destination host")
	}
	TcpPingStorage(stat, target)
	wg.Done()
	seelog.Info("Finish TcpPing " + target + "..")
}

// TcpPingStorage stores a single TCP-ping probe result into the tcppinglog
// table. The "target" column keeps the original "host:port" string so multiple
// ports on the same host can be tracked separately.
func TcpPingStorage(pingres g.PingSt, target string) {
	logtime := time.Now().Format("2006-01-02 15:04")
	seelog.Info("[func:StartTcpPing] ", "(", logtime, ")Starting TcpPingStorage ", target)
	query := "INSERT INTO [tcppinglog] (logtime, target, maxdelay, mindelay, avgdelay, sendpk, revcpk, losspk) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	g.DLock.Lock()
	_, err := g.Db.Exec(query,
		logtime,
		target,
		strconv.FormatFloat(pingres.MaxDelay, 'f', 2, 64),
		strconv.FormatFloat(pingres.MinDelay, 'f', 2, 64),
		strconv.FormatFloat(pingres.AvgDelay, 'f', 2, 64),
		strconv.Itoa(pingres.SendPk),
		strconv.Itoa(pingres.RevcPk),
		strconv.Itoa(pingres.LossPk),
	)
	if err != nil {
		seelog.Error("[func:StartTcpPing] Sql Error ", err)
	}
	g.DLock.Unlock()
	seelog.Info("[func:StartTcpPing] ", "(", logtime, ") Finish TcpPingStorage  ", target)
}
