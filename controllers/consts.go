package controllers

const (
	// SuccessSynced is used as part of the Event 'reason' when a TrafficSplit is synced
	SuccessSynced = "Synced"

	// ErrSyncingConfig is used as part of the Event 'reason' when a TrafficSplit config can not be synced
	ErrSyncingConfig = "ErrSyncingConfig"

	// ErrResourceExists is used as part of the Event 'reason' when a TrafficSplit fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"

	// MessageResourceSyncFailed is the message used for an Event fired when a TrafficSplit
	// is not synced successfully
	MessageResourceSyncFailed = "%s/%s synced failed: %s"

	// MessageResourceSynced is the message used for an Event fired when a TrafficSplit
	// is synced successfully
	MessageResourceSynced = "%s/%s synced successfully"
)
