package probe

import (
	"context"
	"fmt"
	"github.com/fatih/structs"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"net"
	"os"
	"strings"
	"net/http"
	"github.com/shirou/gopsutil/mem"
)

func init() {
	single.Register("env", EnvFunc)
	single.Register("host-info", HostInfoFunc)
	single.Register("cpu-info", CpuInfoFunc)
	single.Register("load-avg", LoadAvgFunc)
	single.Register("network-info", NetworkInfoFunc)
	single.Register("request-info", RequestInfoFunc)
	single.Register("memory-info", MemoryInfoFunc)
	single.Register("status", StatusFunc)
}

func StatusFunc(_ context.Context) (*Result, error) {
	result := NewResult("status")
	result.Data["status"] = "ok"
	return result, nil
}

func EnvFunc(_ context.Context) (*Result, error) {
	result := NewResult("env")
	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		if len(pair) >= 2 {
			result.Data[pair[0]] = pair[1]
		}
	}
	return result, nil
}


func HostInfoFunc(_ context.Context) (*Result, error) {
	result := NewResult("host-info")
	info, err := host.Info()
	if err != nil {
		return nil, err
	}
	s := structs.New(info)
	for k, v := range s.Map() {
		result.Data[k] = fmt.Sprintf("%v", v)
	}
	return result, nil
}

func CpuInfoFunc(_ context.Context) (*Result, error) {
	result := NewResult("cpu-info")
	infos, err := cpu.Info()
	if err != nil {
		return nil, err
	}
	for _, info := range infos {
		s := structs.New(info)
		for k, v := range s.Map() {
			result.Data[k] = fmt.Sprintf("%v", v)
		}
	}
	return result, nil
}

func MemoryInfoFunc(_ context.Context) (*Result, error) {
	result := NewResult("memory-info")
	info, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	result.Summary = fmt.Sprintf("Total: %v, Free:%v, UsedPercent:%f%%", info.Total, info.Free, info.UsedPercent)
	s := structs.New(info)
	for k, v := range s.Map() {
		result.Data[k] = fmt.Sprintf("%v", v)
	}
	return result, nil
}

func LoadAvgFunc(_ context.Context) (*Result, error) {
	result := NewResult("load-avg")
	info, err := load.Avg()
	if err != nil {
		return nil, err
	}
	s := structs.New(info)
	for k, v := range s.Map() {
		result.Data[k] = fmt.Sprintf("%v", v)
	}
	return result, nil
}

func NetworkInfoFunc(_ context.Context) (*Result, error) {
	result := NewResult("network-info")
	faces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, f := range faces {
		addrs, _ := f.Addrs()
		val := fmt.Sprintf("Index:%d Flags:%v HardwareAddr:%s Addrs:%v", f.Index, f.Flags, f.HardwareAddr.String(), addrs)
		result.Data[f.Name] = val
	}
	return result, nil
}

func RequestInfoFunc(ctx context.Context) (*Result, error) {
	result := NewResult("request-info")
	request := ctx.Value("request")
	if httpRequest, ok := request.(*http.Request); ok {
		result.Data["RemoteAddr"] = httpRequest.RemoteAddr
		for key, vals := range httpRequest.Header {
			var value string
			if len(vals) == 1 {
				value = vals[0]
			} else {
				value = fmt.Sprintf("%v", vals)
			}
			result.Data["Header"+key] = value
		}
	}
	return result, nil
}
