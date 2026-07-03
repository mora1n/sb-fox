package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/mora1n/sb-fox/internal/api"
	"github.com/mora1n/sb-fox/internal/config"
	"github.com/mora1n/sb-fox/internal/manage"
)

const daemonControlTimeout = 800 * time.Millisecond

var daemonControlSocketPath = manage.DefaultSocketPath

type daemonControlRequest struct {
	Command string `json:"command"`
	RegMode string `json:"regMode,omitempty"`
}

type daemonControlStatus struct {
	PID                 int    `json:"pid"`
	Version             string `json:"version"`
	Addr                string `json:"addr"`
	DataDir             string `json:"dataDir"`
	RegistrationEnabled bool   `json:"registrationEnabled"`
}

type daemonControlResponse struct {
	OK     bool                `json:"ok"`
	Status daemonControlStatus `json:"status"`
	Error  string              `json:"error,omitempty"`
}

type daemonControlHandler func(daemonControlRequest) daemonControlResponse

type daemonRuntime struct {
	cfg *config.Config
	srv *api.Server
}

func (d daemonRuntime) handle(req daemonControlRequest) daemonControlResponse {
	switch req.Command {
	case "", "status":
		return daemonControlResponse{OK: true, Status: d.status()}
	case "set_registration":
		var enabled bool
		switch req.RegMode {
		case "on":
			enabled = true
		case "off":
			enabled = false
		default:
			return daemonControlResponse{Error: "regMode must be on or off"}
		}
		if err := d.srv.Store.SetSetting(api.SettingRegistrationEnabled, boolSetting(enabled)); err != nil {
			return daemonControlResponse{Error: err.Error()}
		}
		d.srv.SetRegistrationEnabled(enabled)
		return daemonControlResponse{OK: true, Status: d.status()}
	default:
		return daemonControlResponse{Error: "unknown command"}
	}
}

func (d daemonRuntime) status() daemonControlStatus {
	return daemonControlStatus{
		PID:                 os.Getpid(),
		Version:             version,
		Addr:                d.cfg.Addr,
		DataDir:             d.cfg.DataDir,
		RegistrationEnabled: d.srv.IsRegistrationEnabled(),
	}
}

func maybeUseDaemonControl(cfg *config.Config) (bool, error) {
	status, err := queryDaemonStatus(daemonControlSocketPath)
	if err != nil {
		if cfg.RegExplicit || (!cfg.AddrExplicit && daemonSocketExists(daemonControlSocketPath)) {
			return true, fmt.Errorf("daemon socket %s is not available: %w; use sudo or restart sb-fox.service", daemonControlSocketPath, err)
		}
		return false, nil
	}

	if cfg.RegExplicit {
		resp, err := queryDaemon(daemonControlSocketPath, daemonControlRequest{
			Command: "set_registration",
			RegMode: cfg.RegMode,
		})
		if err != nil {
			return true, err
		}
		printDaemonRegistration(resp.Status)
		return true, nil
	}

	if !cfg.AddrExplicit || cfg.Addr == status.Addr {
		printDaemonStatus(status)
		return true, nil
	}

	if !cfg.DataDirExplicit {
		cfg.SetDataDir(status.DataDir)
	}
	return false, nil
}

func queryDaemonStatus(path string) (daemonControlStatus, error) {
	resp, err := queryDaemon(path, daemonControlRequest{Command: "status"})
	if err != nil {
		return daemonControlStatus{}, err
	}
	return resp.Status, nil
}

func queryDaemon(path string, req daemonControlRequest) (daemonControlResponse, error) {
	conn, err := net.DialTimeout("unix", path, daemonControlTimeout)
	if err != nil {
		return daemonControlResponse{}, err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(daemonControlTimeout)); err != nil {
		return daemonControlResponse{}, err
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return daemonControlResponse{}, err
	}
	var resp daemonControlResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return daemonControlResponse{}, err
	}
	if !resp.OK {
		if resp.Error == "" {
			resp.Error = "daemon control failed"
		}
		return resp, errors.New(resp.Error)
	}
	return resp, nil
}

func daemonSocketExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode()&os.ModeSocket != 0
}

func printDaemonStatus(status daemonControlStatus) {
	fmt.Printf("sb-fox daemon is running\naddr: %s\ndata-dir: %s\nregistration: %s\npid: %d\n",
		status.Addr, status.DataDir, regStatus(status.RegistrationEnabled), status.PID)
}

func printDaemonRegistration(status daemonControlStatus) {
	fmt.Printf("sb-fox daemon registration: %s\naddr: %s\ndata-dir: %s\n",
		regStatus(status.RegistrationEnabled), status.Addr, status.DataDir)
}

func regStatus(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func boolSetting(enabled bool) string {
	if enabled {
		return "true"
	}
	return "false"
}
