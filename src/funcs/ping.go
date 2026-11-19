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

func Ping() {
	var wg sync.WaitGroup
	for target := range g.Cfg.PingTargets {
		wg.Add(1)
		go PingTask(target, &wg)
	}
	wg.Wait()
	go StartAlert()
}

//ping main function
func PingTask(addr string, wg *sync.WaitGroup) {
	seelog.Info("Start Ping " + addr + "..")
	stat := g.PingSt{}
	stat.MinDelay = -1
	lossPK := 0
	ipaddr, err := net.ResolveIPAddr("ip", addr)
	// raw ICMP socket 不可用（macOS/Windows 普通用户）时回退到系统 ping 命令
	useCmdPing := err == nil && !nettools.IcmpAvailable()
	if useCmdPing {
		seelog.Warn("Raw ICMP socket unavailable, fallback to system ping command for " + addr)
	}
	if err == nil {
		for i := 0; i < 20; i++ {
			starttime := time.Now().UnixNano()
			var delay float64
			var perr error
			if useCmdPing {
				delay, perr = nettools.RunCmdPing(addr, 3*time.Second)
			} else {
				delay, perr = nettools.RunPing(ipaddr, 3*time.Second, 64, i)
			}
			if perr == nil && delay > 0 {
				stat.AvgDelay = stat.AvgDelay + delay
				if stat.MaxDelay < delay {
					stat.MaxDelay = delay
				}
				if stat.MinDelay == -1 || stat.MinDelay > delay {
					stat.MinDelay = delay
				}
				stat.RevcPk = stat.RevcPk + 1
				seelog.Debug("[func:StartPing IcmpPing] ID:", i, " IP:", addr, " delay:", delay)
			} else {
				if perr != nil {
					seelog.Debug("[func:StartPing IcmpPing] ID:", i, " IP:", addr, "| err:", perr)
				} else {
					seelog.Debug("[func:StartPing IcmpPing] ID:", i, " IP:", addr, "| zero delay treated as fail")
				}
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
		seelog.Debug("[func:IcmpPing] Finish Addr:", addr, " MaxDelay:", stat.MaxDelay, " MinDelay:", stat.MinDelay, " AvgDelay:", stat.AvgDelay, " Revc:", stat.RevcPk, " LossPK:", stat.LossPk)
	} else {
		stat.AvgDelay = 0.00
		stat.MinDelay = 0.00
		stat.MaxDelay = 0.00
		stat.SendPk = 0
		stat.RevcPk = 0
		stat.LossPk = 100
		seelog.Debug("[func:IcmpPing] Finish Addr:", addr, " Unable to resolve destination host")
	}
	PingStorage(stat, addr)
	wg.Done()
	seelog.Info("Finish Ping " + addr + "..")
}

//storage ping data
func PingStorage(pingres g.PingSt, Addr string) {
	logtime := time.Now().Format("2006-01-02 15:04")
	seelog.Info("[func:StartPing] ", "(", logtime, ")Starting PingStorage ", Addr)
	query := "INSERT INTO [pinglog] (logtime, target, maxdelay, mindelay, avgdelay, sendpk, revcpk, losspk) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	g.DLock.Lock()
	_, err := g.Db.Exec(query,
		logtime,
		Addr,
		strconv.FormatFloat(pingres.MaxDelay, 'f', 2, 64),
		strconv.FormatFloat(pingres.MinDelay, 'f', 2, 64),
		strconv.FormatFloat(pingres.AvgDelay, 'f', 2, 64),
		strconv.Itoa(pingres.SendPk),
		strconv.Itoa(pingres.RevcPk),
		strconv.Itoa(pingres.LossPk),
	)
	if err != nil {
		seelog.Error("[func:StartPing] Sql Error ", err)
	}
	g.DLock.Unlock()
	seelog.Info("[func:StartPing] ", "(", logtime, ") Finish PingStorage  ", Addr)
}
