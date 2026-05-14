package services

// CRMSyncCallback is set by the workers package to avoid import cycle
var CRMSyncCallback func(event, entityType, entityID string, data map[string]interface{})

// QueueCRMEvent enqueues an external CRM sync event in a fail-open way.
func QueueCRMEvent(event, entityType, entityID string, data map[string]interface{}) {
	if CRMSyncCallback == nil {
		return
	}

	CRMSyncCallback(event, entityType, entityID, data)
}

// SubmitJobCallback is set by main to submit jobs to worker pool (avoids import cycle)
var SubmitJobCallback func(jobType string, payload interface{}) error
