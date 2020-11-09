package main

import (
	"github.com/fluent/fluent-bit-go/output"
	"github.com/microsoft/ApplicationInsights-Go/appinsights"
)
import (
	"C"
	"os"
	"strings"
	"unsafe"
)

var (
	EnableKubeAudit string
	EnablePerf      string
	EnableFlbPLugin string
)

//export FLBPluginRegister
func FLBPluginRegister(ctx unsafe.Pointer) int {
	return output.FLBPluginRegister(ctx, "oms", "OMS GO!")
}

//export FLBPluginInit
// (fluentbit will call this)
// ctx (context) pointer to fluentbit context (state/ c code)
func FLBPluginInit(ctx unsafe.Pointer) int {
	Log("Initializing out_oms go plugin for fluentbit")
	var agentVersion string
	agentVersion = os.Getenv("AGENT_VERSION")

	osType := os.Getenv("OS_TYPE")
	if strings.Compare(strings.ToLower(osType), "windows") == 0 {
		Log("Using %s for plugin config \n", WindowsContainerLogPluginConfFilePath)
		InitializePlugin(WindowsContainerLogPluginConfFilePath, agentVersion)
	} else {
		if strings.Compare(strings.ToLower(os.Getenv("CONTROLLER_TYPE")), "replicaset") == 0 {
			Log("Using %s for plugin config \n", ReplicaSetContainerLogPluginConfFilePath)
			InitializePlugin(ReplicaSetContainerLogPluginConfFilePath, agentVersion)
		} else {
			Log("Using %s for plugin config \n", DaemonSetContainerLogPluginConfFilePath)
			InitializePlugin(DaemonSetContainerLogPluginConfFilePath, agentVersion)
		}
	}

	EnableKubeAudit = output.FLBPluginConfigKey(ctx, "EnableKubeAudit")
	EnablePerf = output.FLBPluginConfigKey(ctx, "EnablePerf")
	EnableFlbPLugin = output.FLBPluginConfigKey(ctx, "EnableFlbPLugin")
	enableTelemetry := output.FLBPluginConfigKey(ctx, "EnableTelemetry")
	if strings.Compare(strings.ToLower(enableTelemetry), "true") == 0 {
		telemetryPushInterval := output.FLBPluginConfigKey(ctx, "TelemetryPushIntervalSeconds")
		go SendContainerLogPluginMetrics(telemetryPushInterval)
	} else {
		Log("Telemetry is not enabled for the plugin %s \n", output.FLBPluginConfigKey(ctx, "Name"))
		return output.FLB_OK
	}
	return output.FLB_OK
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {

	// Create Fluent Bit decoder
	incomingTag := strings.ToLower(C.GoString(tag))
	Log("EnableKubeAudit: %v", EnableKubeAudit)
	Log("EnableFlbPLugin: %v", EnableFlbPLugin)
	Log("EnablePerf: %v", EnablePerf)
	if strings.HasPrefix(incomingTag, "oms.container.kube.audit") {
		if strings.Compare(strings.ToLower(EnableKubeAudit), "true") == 0 {
			// kube audit collection is off, pass the parsing
			records := GetRecords(data, length)
			return PostDataHelper(records)
		}
	}

	if strings.HasPrefix(incomingTag, "oms.container.log.flbplugin") {
		if strings.Compare(strings.ToLower(EnableFlbPLugin), "true") == 0 {
			// This will also include populating cache to be sent as for config events
			records := GetRecords(data, length)
			return PushToAppInsightsTraces(records, appinsights.Information, incomingTag)
		}
	}

	if strings.HasPrefix(incomingTag, "oms.container.perf.telegraf") {
		if strings.Compare(strings.ToLower(EnablePerf), "true") == 0 {
			// This will also include populating cache to be sent as for config events
			records := GetRecords(data, length)
			return PostTelegrafMetricsToLA(records)
		}
	}
	return output.FLB_OK
}

// GetRecords
func GetRecords(data unsafe.Pointer, length C.int) []map[interface{}]interface{} {
	var ret int
	var record map[interface{}]interface{}
	var records []map[interface{}]interface{}

	dec := output.NewDecoder(data, int(length))
	// Iterate Records
	for {
		// Extract Record
		ret, _, record = output.GetRecord(dec)
		if ret != 0 {
			break
		}
		records = append(records, record)
	}

	return records
}

// FLBPluginExit exits the plugin
func FLBPluginExit() int {
	ContainerLogTelemetryTicker.Stop()
	ContainerImageNameRefreshTicker.Stop()
	return output.FLB_OK
}

func main() {
}
