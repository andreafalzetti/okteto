package analytics

import (
	"github.com/okteto/okteto/pkg/model"
)

const (
	upEvent        = "Up"
	upErrorEvent   = "Up Error"
	syncErrorEvent = "Sync Error"
	reconnectEvent = "Reconnect"
)

const (
	// ReconnectCauseDefault is the default cause for a reconnection
	ReconnectCauseDefault = "unrecognised"

	// ReconnectCauseDevPodRecreated is cause when pods UID change between retrys
	ReconnectCauseDevPodRecreated = "dev-pod-recreated"
)

// TrackUpMetadata defines the properties an up can have
type TrackUpMetadata struct {
	IsV2                   bool
	ManifestType           model.Archetype
	IsInteractive          bool
	IsOktetoRepository     bool
	HasDependenciesSection bool
	HasBuildSection        bool
	HasDeploySection       bool
	Success                bool
	HasReverse             bool
	IsHybridDev            bool
	Mode                   string
}

// TrackUp sends a tracking event to mixpanel when the user activates a development container
func (a *AnalyticsTracker) TrackUp(m TrackUpMetadata) {
	props := map[string]interface{}{
		"isInteractive":          m.IsInteractive,
		"isV2":                   m.IsV2,
		"manifestType":           m.ManifestType,
		"isOktetoRepository":     m.IsOktetoRepository,
		"hasDependenciesSection": m.HasDependenciesSection,
		"hasBuildSection":        m.HasBuildSection,
		"hasDeploySection":       m.HasDeploySection,
		"hasReverse":             m.HasReverse,
		"mode":                   m.Mode,
	}
	a.TrackFn(upEvent, m.Success, props)
}

// TrackUpError sends a tracking event to mixpanel when the okteto up command fails
func (a *AnalyticsTracker) TrackUpError(success bool) {
	a.TrackFn(upErrorEvent, success, nil)
}

const eventActivateUp = "Up Duration Time"

// TrackSecondsActivateUp sends a eventActivateUp to mixpanel
// measures the duration for command up to be active, from start until first exec is done
func (a *AnalyticsTracker) TrackSecondsActivateUp(seconds float64) {
	props := map[string]interface{}{
		"seconds": seconds,
	}
	a.TrackFn(eventActivateUp, true, props)
}

// TrackReconnect sends a tracking event to mixpanel when the development container reconnect
func (a *AnalyticsTracker) TrackReconnect(success bool, cause string) {
	props := map[string]interface{}{
		"cause": cause,
	}
	a.TrackFn(reconnectEvent, success, props)
}

// TrackSyncError sends a tracking event to mixpanel when the init sync fails
func (a *AnalyticsTracker) TrackSyncError() {
	a.TrackFn(syncErrorEvent, false, nil)
}

const eventSecondsToScanLocalFolders = "Up Scan Local Folders Duration"

// TrackSecondsToScanLocalFolders sends eventSecondsToScanLocalFolders to mixpanel with duration as seconds
func (a *AnalyticsTracker) TrackSecondsToScanLocalFolders(seconds float64) {
	props := map[string]interface{}{
		"seconds": seconds,
	}
	a.TrackFn(eventSecondsToScanLocalFolders, true, props)
}

const eventSecondsToSyncContext = "Up Sync Context Duration"

// TrackSecondsToScanLocalFolders sends eventSecondsToScanLocalFolders to mixpanel with duration as seconds
func (a *AnalyticsTracker) TrackSecondsToSyncContext(seconds float64) {
	props := map[string]interface{}{
		"seconds": seconds,
	}
	a.TrackFn(eventSecondsToSyncContext, true, props)
}

// eventSecondsUpCommandExecution measures the seconds a command is running during up
const eventSecondsUpCommandExecution = "Up Command Execution Duration"

// TrackUpTotalCommandExecution sends eventSecondsUpCommandExecution to mixpanel with duration as seconds
func (a *AnalyticsTracker) TrackSecondsUpCommandExecution(seconds float64) {
	props := map[string]interface{}{
		"seconds": seconds,
	}
	a.TrackFn(eventSecondsUpCommandExecution, true, props)
}

// eventSecondsUpOktetoContextConfig measures the seconds it takes to configure okteto context under an up command
const eventSecondsUpOktetoContextConfig = "Up Okteto Context Config Duration"

// TrackUpTotalCommandExecution sends eventSecondsUpCommandExecution to mixpanel with duration as seconds
func (a *AnalyticsTracker) TrackSecondsUpOktetoContextConfig(seconds float64) {
	props := map[string]interface{}{
		"seconds": seconds,
	}
	a.TrackFn(eventSecondsUpOktetoContextConfig, true, props)
}
