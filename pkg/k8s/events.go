package k8s

const (
	// Gateway events
	GatewayEventReasonFailedAddFinalizer = "FailedAddFinalizer"
	GatewayEventReasonFailedBuildModel   = "FailedBuildModel"
	GatewayEventReasonFailedDeployModel  = "FailedDeployModel"

	// Route events
	RouteEventReasonReconcile          = "Reconcile"
	RouteEventReasonDeploySucceed      = "DeploySucceed"
	RouteEventReasonFailedAddFinalizer = "FailedAddFinalizer"
	RouteEventReasonFailedBuildModel   = "FailedBuildModel"
	RouteEventReasonFailedDeployModel  = "FailedDeployModel"
	RouteEventReasonRetryReconcile     = "Retry-Reconcile"

	// Service events
	ServiceEventReasonFailedAddFinalizer = "FailedAddFinalizer"
	ServiceEventReasonFailedBuildModel   = "FailedBuildModel"
	ServiceEventReasonFailedDeployModel  = "FailedDeployModel"

	// ServiceExport events
	ServiceExportEventReasonFailedAddFinalizer = "FailedAddFinalizer"
	ServiceExportEventReasonFailedBuildModel   = "FailedBuildModel"
	ServiceExportEventReasonFailedDeployModel  = "FailedDeployModel"

	// ServiceImport events
	ServiceImportEventReasonFailedAddFinalizer = "FailedAddFinalizer"
	ServiceImportEventReasonFailedBuildModel   = "FailedBuildModel"
	ServiceImportEventReasonFailedDeployModel  = "FailedDeployModel"

	// AccessLogPolicy events
	AccessLogPolicyEventReasonFailedAddFinalizer = "FailedAddFinalizer"
	AccessLogPolicyEventReasonFailedBuildModel   = "FailedBuildModel"
)
