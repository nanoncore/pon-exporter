package target

import (
	"context"
	"log/slog"
	"time"

	southbound "github.com/nanoncore/nano-southbound"
	"github.com/nanoncore/nano-southbound/types"
	"github.com/nanoncore/pon-exporter/internal/config"
	"github.com/nanoncore/pon-exporter/internal/poller"
)

// Poll performs a single poll cycle for a target. It creates a driver,
// connects, gathers data, disconnects, and returns a snapshot.
func Poll(ctx context.Context, cfg config.TargetConfig, logger *slog.Logger) *poller.TargetSnapshot {
	start := time.Now()
	snap := &poller.TargetSnapshot{
		Target:   cfg.Name,
		Labels:   cfg.Labels,
		PollTime: start,
	}

	// Build equipment config
	eqCfg := &types.EquipmentConfig{
		Name:     cfg.Name,
		Type:     types.EquipmentTypeOLT,
		Vendor:   types.Vendor(cfg.Vendor),
		Address:  cfg.Address,
		Port:     cfg.Port,
		Protocol: types.Protocol(cfg.Protocol),
		Username: cfg.Username,
		Password: cfg.Password,
		Timeout:  time.Duration(cfg.Timeout),
		Metadata: make(map[string]string),
	}

	if cfg.SNMPCommunity != "" {
		eqCfg.Metadata["snmp_community"] = cfg.SNMPCommunity
	}
	if cfg.SNMPVersion != "" {
		eqCfg.Metadata["snmp_version"] = cfg.SNMPVersion
	}
	// Pass CLI credentials for SNMP targets that may need CLI fallback
	if cfg.Protocol == "snmp" && cfg.Username != "" {
		eqCfg.Metadata["cli_host"] = cfg.Address
	}

	// Create driver
	driver, err := southbound.NewDriver(eqCfg.Vendor, eqCfg.Protocol, eqCfg)
	if err != nil {
		logger.Error("failed to create driver", "target", cfg.Name, "err", err)
		snap.ErrorCount++
		snap.Duration = time.Since(start)
		return snap
	}

	// Connect
	connectCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Timeout))
	defer cancel()

	if err := driver.Connect(connectCtx, eqCfg); err != nil {
		logger.Error("failed to connect", "target", cfg.Name, "err", err)
		snap.ErrorCount++
		snap.Duration = time.Since(start)
		return snap
	}
	defer func() { _ = driver.Disconnect(ctx) }()

	snap.Up = true

	// Check DriverV2 support
	driverV2, ok := driver.(types.DriverV2)
	if !ok {
		logger.Warn("driver does not support DriverV2", "target", cfg.Name)
		snap.Duration = time.Since(start)
		return snap
	}
	snap.DriverV2 = true

	// GetOLTStatus — independent failure
	oltStatus, err := driverV2.GetOLTStatus(ctx)
	if err != nil {
		logger.Warn("GetOLTStatus failed", "target", cfg.Name, "err", err)
		snap.ErrorCount++
	} else if oltStatus != nil {
		snap.OLT = &poller.OLTStatusSnapshot{
			Vendor:        oltStatus.Vendor,
			Model:         oltStatus.Model,
			Firmware:      oltStatus.Firmware,
			SerialNumber:  oltStatus.SerialNumber,
			CPUPercent:    oltStatus.CPUPercent,
			MemoryPercent: oltStatus.MemoryPercent,
			Temperature:   oltStatus.Temperature,
			ActiveONUs:    oltStatus.ActiveONUs,
			TotalONUs:     oltStatus.TotalONUs,
			UptimeSeconds: oltStatus.UptimeSeconds,
		}
		for _, pp := range oltStatus.PONPorts {
			snap.PONPorts = append(snap.PONPorts, poller.PONPortSnapshot{
				Port:       pp.Port,
				ONUCount:   pp.ONUCount,
				RxPowerDBm: pp.RxPowerDBm,
				TxPowerDBm: pp.TxPowerDBm,
			})
		}
	}

	// GetONUList — independent failure
	onus, err := driverV2.GetONUList(ctx, nil)
	if err != nil {
		logger.Warn("GetONUList failed", "target", cfg.Name, "err", err)
		snap.ErrorCount++
	} else {
		for _, onu := range onus {
			withinSpec := types.IsPowerWithinSpec(onu.RxPowerDBm, onu.TxPowerDBm)
			snap.ONUs = append(snap.ONUs, poller.ONUSnapshot{
				PONPort:      onu.PONPort,
				ONUID:        onu.ONUID,
				Serial:       onu.Serial,
				Vendor:       onu.Vendor,
				Model:        onu.Model,
				AdminState:   onu.AdminState,
				OperState:    onu.OperState,
				IsOnline:     onu.IsOnline,
				RxPowerDBm:   onu.RxPowerDBm,
				TxPowerDBm:   onu.TxPowerDBm,
				DistanceM:    onu.DistanceM,
				Temperature:  onu.Temperature,
				IsWithinSpec: withinSpec,
				BytesUp:      onu.BytesUp,
				BytesDown:    onu.BytesDown,
			})
		}
	}

	// GetAlarms — independent failure
	alarms, err := driverV2.GetAlarms(ctx)
	if err != nil {
		logger.Warn("GetAlarms failed", "target", cfg.Name, "err", err)
		snap.ErrorCount++
	} else {
		for _, a := range alarms {
			snap.Alarms = append(snap.Alarms, poller.AlarmSnapshot{
				Severity: a.Severity,
			})
		}
	}

	snap.Duration = time.Since(start)
	return snap
}
