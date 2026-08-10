package api

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type serverConfigResponse struct {
	PublicKey  string `json:"public_key"`
	Host       string `json:"host"`
	Port       string `json:"port"`
	AllowedIPs string `json:"allowed_ips"`
	DNSServer  string `json:"dns_server"`
}

// handleGetConfig returns the server's public WireGuard config values
// (GET /api/config).
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, serverConfigResponse{
		PublicKey:  s.WGPublicKey,
		Host:       s.Config.WGHost,
		Port:       s.Config.WGPort,
		AllowedIPs: s.Config.WGAllowedIPs,
		DNSServer:  s.Config.WGDNSServer,
	})
}

type serverInfoResponse struct {
	Uptime string `json:"uptime"`
	Load   string `json:"load"`
}

// handleServerInfo reports host uptime and load average
// (GET /api/serverinfo).
func (s *Server) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	uptime, err := readUptime()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	load, err := readLoadAvg()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, serverInfoResponse{
		Uptime: formatUptime(uptime),
		Load:   load,
	})
}

// readUptime reads system uptime from /proc/uptime.
func readUptime() (time.Duration, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, fmt.Errorf("read /proc/uptime: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, fmt.Errorf("read /proc/uptime: unexpected format")
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("parse /proc/uptime: %w", err)
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

// formatUptime renders total elapsed time as H:MM:SS, with H unbounded past
// 24 -- unlike the original app's strftime("%H:%M:%S", gmtime(uptime)),
// which silently wrapped every 24h since gmtime interprets its argument as
// an absolute epoch timestamp. That was a display bug, not a contract
// anything depends on, so it's fixed here rather than reproduced.
func formatUptime(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	sec := d / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", h, m, sec)
}

// readLoadAvg reads the 1/5/15-minute load averages from /proc/loadavg.
func readLoadAvg() (string, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "", fmt.Errorf("read /proc/loadavg: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return "", fmt.Errorf("read /proc/loadavg: unexpected format")
	}
	// /proc/loadavg's first three fields are already formatted to 2 decimal
	// places by the kernel.
	return fmt.Sprintf("%s %s %s", fields[0], fields[1], fields[2]), nil
}
