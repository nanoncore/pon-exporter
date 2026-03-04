package collector

import "github.com/prometheus/client_golang/prometheus"

const namespace = "gpon"

var (
	descUp = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Whether the OLT target is reachable (1=up, 0=down).",
		[]string{"target"}, nil,
	)
	descDriverV2Supported = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "driver_v2_supported"),
		"Whether the driver supports DriverV2 interface (1=yes, 0=no).",
		[]string{"target"}, nil,
	)
	descScrapeDuration = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "scrape_duration_seconds"),
		"Duration of the last poll in seconds.",
		[]string{"target"}, nil,
	)
	descScrapeErrors = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "scrape_errors_total"),
		"Total number of poll errors for a target.",
		[]string{"target"}, nil,
	)
	descOLTInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "olt", "info"),
		"Static OLT information. Value is always 1.",
		[]string{"target", "vendor", "model", "firmware", "serial_number"}, nil,
	)
	descOLTCPU = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "olt", "cpu_percent"),
		"OLT CPU utilization percentage.",
		[]string{"target"}, nil,
	)
	descOLTMemory = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "olt", "memory_percent"),
		"OLT memory utilization percentage.",
		[]string{"target"}, nil,
	)
	descOLTTemperature = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "olt", "temperature_celsius"),
		"OLT system temperature in Celsius.",
		[]string{"target"}, nil,
	)
	descOLTActiveONUs = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "olt", "active_onus"),
		"Number of online ONUs.",
		[]string{"target"}, nil,
	)
	descOLTTotalONUs = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "olt", "total_onus"),
		"Total number of provisioned ONUs.",
		[]string{"target"}, nil,
	)
	descOLTUptime = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "olt", "uptime_seconds"),
		"OLT uptime in seconds.",
		[]string{"target"}, nil,
	)
	descOLTActiveAlarms = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "olt", "active_alarms"),
		"Total number of active alarms.",
		[]string{"target"}, nil,
	)
	descOLTAlarmsBySeverity = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "olt", "alarms_by_severity"),
		"Number of active alarms grouped by severity.",
		[]string{"target", "severity"}, nil,
	)

	// PON port metrics
	descPONPortONUCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "pon_port", "onu_count"),
		"Number of ONUs on a PON port.",
		[]string{"target", "pon_port"}, nil,
	)
	descPONPortRxPower = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "pon_port", "rx_power_dbm"),
		"PON port receive power in dBm.",
		[]string{"target", "pon_port"}, nil,
	)
	descPONPortTxPower = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "pon_port", "tx_power_dbm"),
		"PON port transmit power in dBm.",
		[]string{"target", "pon_port"}, nil,
	)

	// ONU metrics
	onuLabels = []string{"target", "pon_port", "onu_id", "serial"}

	descONUInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "onu", "info"),
		"Static ONU information. Value is always 1.",
		[]string{"target", "pon_port", "onu_id", "serial", "oper_state", "admin_state", "vendor", "model"}, nil,
	)
	descONUOnline = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "onu", "online"),
		"Whether the ONU is online (1=online, 0=offline).",
		onuLabels, nil,
	)
	descONURxPower = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "onu", "rx_power_dbm"),
		"ONU receive power in dBm.",
		onuLabels, nil,
	)
	descONUTxPower = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "onu", "tx_power_dbm"),
		"ONU transmit power in dBm.",
		onuLabels, nil,
	)
	descONUOLTRxPower = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "onu", "olt_rx_power_dbm"),
		"OLT receive power from this ONU in dBm.",
		onuLabels, nil,
	)
	descONUDistance = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "onu", "distance_meters"),
		"Estimated fiber distance to ONU in meters.",
		onuLabels, nil,
	)
	descONUTemperature = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "onu", "temperature_celsius"),
		"ONU temperature in Celsius.",
		onuLabels, nil,
	)
	descONUSignalWithinSpec = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "onu", "signal_within_spec"),
		"Whether the ONU signal is within GPON spec (1=yes, 0=no).",
		onuLabels, nil,
	)
	descONUBytesUp = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "onu", "bytes_up_total"),
		"Total bytes transmitted upstream by ONU.",
		onuLabels, nil,
	)
	descONUBytesDown = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "onu", "bytes_down_total"),
		"Total bytes received downstream by ONU.",
		onuLabels, nil,
	)
)
