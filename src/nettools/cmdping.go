package nettools

import (
	"fmt"
	"github.com/cihub/seelog"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// IcmpAvailable 检测是否使用 Go 原生 raw ICMP。
// 设计：Linux/容器（root + CAP_NET_RAW）走原生 ICMP（更快、无子进程、无本地化问题）；
// Windows 强制走系统 ping 命令——实测 Windows raw ICMP socket 对回环/局域网
// 目标存在丢包兼容性问题（回包偶发收不到），而系统 ping 稳定 0% 丢包。
var icmpOnce sync.Once
var icmpOK bool

func IcmpAvailable() bool {
	icmpOnce.Do(func() {
		if runtime.GOOS == "windows" {
			icmpOK = false
			return
		}
		conn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
		if err != nil {
			seelog.Warn("[IcmpAvailable] raw ICMP socket unavailable, fallback to system ping: ", err)
			icmpOK = false
			return
		}
		conn.Close()
		icmpOK = true
	})
	return icmpOK
}

// pingTimeRe 匹配各平台 ping 输出中的往返时间：

// pingTimeRe 匹配 ping 输出中的往返时间数值(ms)。
// 仅匹配 ASCII 数字 + "ms"，不依赖 "time/时间" 等本地化前缀，
// 以规避中文 Windows 下 ping 输出为 GBK 编码导致 UTF-8 正则无法匹配、
// 进而把成功的探测误判为失败(0 延迟/100% 丢包)的问题。
var pingTimeRe = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*ms`)

// RunCmdPing 调用系统 ping 命令执行一次 ICMP 探测，返回毫秒延迟。
// 用于 raw ICMP socket 不可用（普通用户）时的回退方案。
func RunCmdPing(host string, timeout time.Duration) (float64, error) {
	args := cmdPingArgs(host, timeout)
	cmd := exec.Command("ping", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("ping %s failed: %v", host, err)
	}
	delay, perr := parsePingTime(string(out))
	if perr != nil {
		return 0, perr
	}
	return delay, nil
}

// cmdPingArgs 构造对应平台的一次性 ping 参数。
func cmdPingArgs(host string, timeout time.Duration) []string {
	switch runtime.GOOS {
	case "windows":
		// -w 单位: 毫秒
		return []string{"-n", "1", "-w", strconv.Itoa(int(timeout / time.Millisecond)), host}
	case "darwin":
		// -t 单位: 秒
		return []string{"-c", "1", "-t", strconv.Itoa(int(timeout.Seconds())), host}
	default:
		// linux 等: -W 单位: 秒
		return []string{"-c", "1", "-W", strconv.Itoa(int(timeout.Seconds())), host}
	}
}

func parsePingTime(out string) (float64, error) {
	m := pingTimeRe.FindStringSubmatch(out)
	if len(m) != 2 {
		return 0, fmt.Errorf("cannot parse ping output: %q", out)
	}
	t, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid ping delay: %v", err)
	}
	return t, nil
}
