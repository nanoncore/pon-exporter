package collector

import (
	"fmt"
	"sort"

	"github.com/nanoncore/pon-exporter/internal/poller"
	"github.com/prometheus/client_golang/prometheus"
)

// GPONCollector implements prometheus.Collector using cached snapshots.
type GPONCollector struct {
	store *poller.SnapshotStore
}

// New creates a GPONCollector that reads from the given snapshot store.
func New(store *poller.SnapshotStore) *GPONCollector {
	return &GPONCollector{store: store}
}

// Describe sends metric descriptors to the channel.
// gpon_target_info is omitted because its label set is dynamic (user-defined).
func (c *GPONCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descUp
	ch <- descDriverV2Supported
	ch <- descScrapeDuration
	ch <- descScrapeErrors
	ch <- descOLTInfo
	ch <- descOLTCPU
	ch <- descOLTMemory
	ch <- descOLTTemperature
	ch <- descOLTActiveONUs
	ch <- descOLTTotalONUs
	ch <- descOLTUptime
	ch <- descOLTActiveAlarms
	ch <- descOLTAlarmsBySeverity
	ch <- descPONPortONUCount
	ch <- descPONPortRxPower
	ch <- descPONPortTxPower
	ch <- descONUInfo
	ch <- descONUOnline
	ch <- descONURxPower
	ch <- descONUTxPower
	ch <- descONUOLTRxPower
	ch <- descONUDistance
	ch <- descONUTemperature
	ch <- descONUSignalWithinSpec
	ch <- descONUBytesUp
	ch <- descONUBytesDown
}

// Collect emits metrics from the cached snapshots.
func (c *GPONCollector) Collect(ch chan<- prometheus.Metric) {
	for _, snap := range c.store.GetAll() {
		c.collectTarget(ch, snap)
	}
}

func (c *GPONCollector) collectTarget(ch chan<- prometheus.Metric, snap *poller.TargetSnapshot) {
	target := snap.Target

	// gpon_up
	up := 0.0
	if snap.Up {
		up = 1.0
	}
	ch <- prometheus.MustNewConstMetric(descUp, prometheus.GaugeValue, up, target)

	// gpon_driver_v2_supported
	v2 := 0.0
	if snap.DriverV2 {
		v2 = 1.0
	}
	ch <- prometheus.MustNewConstMetric(descDriverV2Supported, prometheus.GaugeValue, v2, target)

	// gpon_scrape_duration_seconds
	ch <- prometheus.MustNewConstMetric(descScrapeDuration, prometheus.GaugeValue, snap.Duration.Seconds(), target)

	// gpon_scrape_errors_total
	ch <- prometheus.MustNewConstMetric(descScrapeErrors, prometheus.CounterValue, float64(snap.ErrorCount), target)

	// gpon_target_info — dynamic label set
	if len(snap.Labels) > 0 {
		keys := sortedKeys(snap.Labels)
		labelNames := append([]string{"target"}, keys...)
		labelValues := make([]string, 0, len(keys)+1)
		labelValues = append(labelValues, target)
		for _, k := range keys {
			labelValues = append(labelValues, snap.Labels[k])
		}
		desc := prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "target_info"),
			"Static target information labels. Value is always 1.",
			labelNames, nil,
		)
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, 1, labelValues...)
	}

	if !snap.Up || !snap.DriverV2 {
		return
	}

	// OLT metrics
	if olt := snap.OLT; olt != nil {
		ch <- prometheus.MustNewConstMetric(descOLTInfo, prometheus.GaugeValue, 1,
			target, olt.Vendor, olt.Model, olt.Firmware, olt.SerialNumber)
		ch <- prometheus.MustNewConstMetric(descOLTCPU, prometheus.GaugeValue, olt.CPUPercent, target)
		ch <- prometheus.MustNewConstMetric(descOLTMemory, prometheus.GaugeValue, olt.MemoryPercent, target)
		ch <- prometheus.MustNewConstMetric(descOLTTemperature, prometheus.GaugeValue, olt.Temperature, target)
		ch <- prometheus.MustNewConstMetric(descOLTActiveONUs, prometheus.GaugeValue, float64(olt.ActiveONUs), target)
		ch <- prometheus.MustNewConstMetric(descOLTTotalONUs, prometheus.GaugeValue, float64(olt.TotalONUs), target)
		ch <- prometheus.MustNewConstMetric(descOLTUptime, prometheus.GaugeValue, float64(olt.UptimeSeconds), target)
	}

	// Alarm metrics
	ch <- prometheus.MustNewConstMetric(descOLTActiveAlarms, prometheus.GaugeValue, float64(len(snap.Alarms)), target)
	severityCounts := make(map[string]int)
	for _, a := range snap.Alarms {
		severityCounts[a.Severity]++
	}
	for sev, count := range severityCounts {
		ch <- prometheus.MustNewConstMetric(descOLTAlarmsBySeverity, prometheus.GaugeValue, float64(count), target, sev)
	}

	// PON port metrics
	for _, p := range snap.PONPorts {
		ch <- prometheus.MustNewConstMetric(descPONPortONUCount, prometheus.GaugeValue, float64(p.ONUCount), target, p.Port)
		ch <- prometheus.MustNewConstMetric(descPONPortRxPower, prometheus.GaugeValue, p.RxPowerDBm, target, p.Port)
		ch <- prometheus.MustNewConstMetric(descPONPortTxPower, prometheus.GaugeValue, p.TxPowerDBm, target, p.Port)
	}

	// ONU metrics
	for _, onu := range snap.ONUs {
		onuID := fmt.Sprintf("%d", onu.ONUID)
		labels := []string{target, onu.PONPort, onuID, onu.Serial}

		ch <- prometheus.MustNewConstMetric(descONUInfo, prometheus.GaugeValue, 1,
			target, onu.PONPort, onuID, onu.Serial, onu.OperState, onu.AdminState, onu.Vendor, onu.Model)

		online := 0.0
		if onu.IsOnline {
			online = 1.0
		}
		ch <- prometheus.MustNewConstMetric(descONUOnline, prometheus.GaugeValue, online, labels...)
		ch <- prometheus.MustNewConstMetric(descONURxPower, prometheus.GaugeValue, onu.RxPowerDBm, labels...)
		ch <- prometheus.MustNewConstMetric(descONUTxPower, prometheus.GaugeValue, onu.TxPowerDBm, labels...)
		ch <- prometheus.MustNewConstMetric(descONUOLTRxPower, prometheus.GaugeValue, onu.OLTRxDBm, labels...)
		ch <- prometheus.MustNewConstMetric(descONUDistance, prometheus.GaugeValue, float64(onu.DistanceM), labels...)
		ch <- prometheus.MustNewConstMetric(descONUTemperature, prometheus.GaugeValue, onu.Temperature, labels...)

		withinSpec := 0.0
		if onu.IsWithinSpec {
			withinSpec = 1.0
		}
		ch <- prometheus.MustNewConstMetric(descONUSignalWithinSpec, prometheus.GaugeValue, withinSpec, labels...)
		ch <- prometheus.MustNewConstMetric(descONUBytesUp, prometheus.CounterValue, float64(onu.BytesUp), labels...)
		ch <- prometheus.MustNewConstMetric(descONUBytesDown, prometheus.CounterValue, float64(onu.BytesDown), labels...)
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
