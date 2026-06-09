package hevlayer

type JSONValue = interface{}

type CreatePipelineRequest struct {
	ID string `json:"id"`
	TargetNamespace string `json:"target_namespace"`
	DistanceMetric string `json:"distance_metric,omitempty"`
}

type Pipeline struct {
	ID string `json:"id"`
	TargetNamespace string `json:"target_namespace"`
	DistanceMetric string `json:"distance_metric"`
	CreatedAt string `json:"created_at"`
}

type PipelineList struct {
	Pipelines []Pipeline `json:"pipelines"`
}

type PipelineStatus struct {
	PipelineID string `json:"pipeline_id"`
	Counts map[string]int64 `json:"counts"`
	PendingCount int64 `json:"pending_count"`
	ProcessingCount int64 `json:"processing_count"`
	FailedCount int64 `json:"failed_count"`
	IndexedRatePerMin float64 `json:"indexed_rate_per_min"`
	RateWindowSeconds int64 `json:"rate_window_seconds"`
}

type ClaimDocumentsRequest struct {
	Stage string `json:"stage,omitempty"`
	ClaimStage string `json:"claim_stage,omitempty"`
	Limit int64 `json:"limit,omitempty"`
	WorkerID string `json:"worker_id"`
	LeaseSeconds int64 `json:"lease_seconds,omitempty"`
	DocumentIdPrefix string `json:"document_id_prefix,omitempty"`
}

type ClaimDocumentsResponse struct {
	PipelineID string `json:"pipeline_id"`
	Stage string `json:"stage"`
	ClaimStage string `json:"claim_stage"`
	WorkerID string `json:"worker_id"`
	Documents []string `json:"documents"`
}

type HeartbeatDocumentsRequest struct {
	DocumentIds []string `json:"document_ids"`
	Stage string `json:"stage,omitempty"`
	WorkerID string `json:"worker_id"`
}

type SetDocumentsStageRequest struct {
	DocumentIds []string `json:"document_ids"`
	Stage string `json:"stage"`
	FromStage string `json:"from_stage,omitempty"`
	WorkerID string `json:"worker_id,omitempty"`
	CreateMissing bool `json:"create_missing,omitempty"`
}

type DocumentsStageResponse struct {
	PipelineID string `json:"pipeline_id"`
	Stage string `json:"stage"`
	Updated int64 `json:"updated"`
}

type StageDocumentResponse struct {
	PipelineID string `json:"pipeline_id"`
	DocumentID string `json:"document_id"`
	Stage string `json:"stage"`
	ChunkCount int64 `json:"chunk_count"`
	ChunkIds []string `json:"chunk_ids"`
}

type Chunk struct {
	ID string `json:"id"`
	Text string `json:"text,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type PutChunksRequest struct {
	Chunks []Chunk `json:"chunks"`
}

type GetChunksResponse []Chunk

type VectorEntry struct {
	ID string `json:"id"`
	Vector []float64 `json:"vector"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type PutVectorsRequest struct {
	Vectors []VectorEntry `json:"vectors"`
}

type CreateUdfRequest struct {
	ID string `json:"id"`
	Spec UdfSpec `json:"spec"`
}

type Udf struct {
	ID string `json:"id"`
	Spec UdfSpec `json:"spec"`
	Paused bool `json:"paused"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type UdfList struct {
	Udfs []Udf `json:"udfs"`
}

type GetUdfResponse struct {
	Udf Udf `json:"udf"`
	Status UdfStatus `json:"status"`
}

type UdfStatus struct {
	UdfID string `json:"udf_id"`
	Paused bool `json:"paused"`
	ActiveNamespaces []string `json:"active_namespaces"`
	Discovery UdfDiscoveryStatus `json:"discovery"`
	Counts map[string]int64 `json:"counts"`
	PendingCount int64 `json:"pending_count"`
	ProcessingCount int64 `json:"processing_count"`
	FailedCount int64 `json:"failed_count"`
	IndexedRatePerMin float64 `json:"indexed_rate_per_min"`
	RateWindowSeconds int64 `json:"rate_window_seconds"`
}

type UdfDiscoveryStatus struct {
	SweepsCompleted int64 `json:"sweeps_completed"`
	LastCompletedAt *string `json:"last_completed_at"`
}

type UdfSpec struct {
	IndexSelector interface{} `json:"index_selector,omitempty"`
	TargetNamespaces []string `json:"target_namespaces,omitempty"`
	Inputs []string `json:"inputs,omitempty"`
	Version string `json:"version,omitempty"`
	Filter interface{} `json:"filter,omitempty"`
	Worker UdfWorkerSpec `json:"worker"`
	Schedule UdfScheduleSpec `json:"schedule,omitempty"`
	Retry UdfRetrySpec `json:"retry,omitempty"`
	Triggers []UdfTrigger `json:"triggers,omitempty"`
	Invalidates []string `json:"invalidates,omitempty"`
}

type UdfTrigger string

type UdfWorkerSpec struct {
	Image string `json:"image,omitempty"`
	Url string `json:"url,omitempty"`
	Port int64 `json:"port,omitempty"`
	BatchSize int64 `json:"batch_size,omitempty"`
	TimeoutSeconds int64 `json:"timeout_seconds,omitempty"`
	PodSpec interface{} `json:"pod_spec,omitempty"`
}

type UdfScheduleSpec struct {
	DiscoveryIntervalSeconds int64 `json:"discovery_interval_seconds,omitempty"`
	LeaseSeconds int64 `json:"lease_seconds,omitempty"`
	MaxInFlightBatches int64 `json:"max_in_flight_batches,omitempty"`
	MaxConcurrentScans int64 `json:"max_concurrent_scans,omitempty"`
}

type UdfRetrySpec struct {
	MaxAttempts int64 `json:"max_attempts,omitempty"`
	InitialBackoffSeconds int64 `json:"initial_backoff_seconds,omitempty"`
	MaxBackoffSeconds int64 `json:"max_backoff_seconds,omitempty"`
}

type UdfDiscoverRequest struct {
	Namespaces []string `json:"namespaces,omitempty"`
	PageSize int64 `json:"page_size,omitempty"`
}

type UdfDiscoverResponse struct {
	UdfID string `json:"udf_id"`
	Enqueued int64 `json:"enqueued"`
	Namespaces []string `json:"namespaces"`
}

type UdfClaimRequest struct {
	WorkerID string `json:"worker_id"`
	Limit int64 `json:"limit,omitempty"`
	LeaseSeconds int64 `json:"lease_seconds,omitempty"`
}

type UdfClaimedItem struct {
	Namespace string `json:"namespace"`
	ID string `json:"id"`
	Input map[string]interface{} `json:"input"`
}

type UdfClaimResponse struct {
	UdfID string `json:"udf_id"`
	WorkerID string `json:"worker_id"`
	Items []UdfClaimedItem `json:"items"`
}

type UdfItemRef struct {
	Namespace string `json:"namespace"`
	ID string `json:"id"`
}

type UdfHeartbeatRequest struct {
	WorkerID string `json:"worker_id"`
	Items []UdfItemRef `json:"items"`
}

type UdfCompleteRequest struct {
	WorkerID string `json:"worker_id"`
	Items []UdfCompleteItem `json:"items"`
}

type UdfCompleteItem struct {
	Namespace string `json:"namespace"`
	ID string `json:"id"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type UdfErrorKind string

type UdfFailRequest struct {
	WorkerID string `json:"worker_id"`
	Items []UdfFailItem `json:"items"`
}

type UdfFailItem struct {
	Namespace string `json:"namespace"`
	ID string `json:"id"`
	Kind UdfErrorKind `json:"kind"`
	Message string `json:"message,omitempty"`
}

type UdfItemsResponse struct {
	UdfID string `json:"udf_id"`
	Updated int64 `json:"updated"`
}

type Document struct {
	ID string `json:"id"`
	Attributes map[string]interface{} `json:"attributes"`
}

type FetchDocumentsRequest struct {
	Ids []string `json:"ids"`
	IncludeAttributes []string `json:"include_attributes,omitempty"`
}

type FetchDocumentsResponse struct {
	Documents []Document `json:"documents"`
	Missing []string `json:"missing"`
}

type StatusResponse struct {
	Status string `json:"status"`
	Message string `json:"message,omitempty"`
	RowsAffected int64 `json:"rows_affected,omitempty"`
	RowsUpserted int64 `json:"rows_upserted,omitempty"`
	RowsPatched int64 `json:"rows_patched,omitempty"`
	RowsDeleted int64 `json:"rows_deleted,omitempty"`
	Billing map[string]interface{} `json:"billing,omitempty"`
}

type TurbopufferNamespaceSummary struct {
	ID string `json:"id"`
}

type TurbopufferNamespaceList struct {
	Namespaces []TurbopufferNamespaceSummary `json:"namespaces"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type TurbopufferSchema map[string]interface{}

type TurbopufferMetadataPatch struct {
	Pinning interface{} `json:"pinning,omitempty"`
}

type TurbopufferWriteRequest map[string]interface{}

type TurbopufferBranchFromRequest struct {
	BranchFromNamespace map[string]interface{} `json:"branch_from_namespace"`
}

type TurbopufferCopyFromRequest struct {
	CopyFromNamespace interface{} `json:"copy_from_namespace"`
}

type TurbopufferWriteResponse struct {
	Status string `json:"status"`
	Message string `json:"message"`
	RowsAffected int64 `json:"rows_affected"`
	RowsUpserted int64 `json:"rows_upserted,omitempty"`
	RowsPatched int64 `json:"rows_patched,omitempty"`
	RowsDeleted int64 `json:"rows_deleted,omitempty"`
	RowsRemaining bool `json:"rows_remaining,omitempty"`
	UpsertedIds []interface{} `json:"upserted_ids,omitempty"`
	PatchedIds []interface{} `json:"patched_ids,omitempty"`
	DeletedIds []interface{} `json:"deleted_ids,omitempty"`
	Billing map[string]interface{} `json:"billing"`
	Performance map[string]interface{} `json:"performance,omitempty"`
}

type TurbopufferQueryRequest map[string]interface{}

type TurbopufferQueryResponse struct {
	Rows []map[string]interface{} `json:"rows,omitempty"`
	Aggregations map[string]interface{} `json:"aggregations,omitempty"`
	AggregationGroups []map[string]interface{} `json:"aggregation_groups,omitempty"`
	Billing map[string]interface{} `json:"billing,omitempty"`
	Performance map[string]interface{} `json:"performance,omitempty"`
}

type TurbopufferMultiQueryRequest struct {
	Queries []TurbopufferQueryRequest `json:"queries"`
	Consistency map[string]interface{} `json:"consistency,omitempty"`
	VectorEncoding string `json:"vector_encoding,omitempty"`
}

type TurbopufferMultiQueryResponse struct {
	Results []TurbopufferQueryResponse `json:"results"`
	Billing map[string]interface{} `json:"billing,omitempty"`
	Performance map[string]interface{} `json:"performance,omitempty"`
	StableAsOf int64 `json:"stable_as_of,omitempty"`
}

type TurbopufferExplainQueryResponse struct {
	PlanText string `json:"plan_text,omitempty"`
}

type TurbopufferRecallRequest struct {
	Num int64 `json:"num,omitempty"`
	TopK int64 `json:"top_k,omitempty"`
	Filters interface{} `json:"filters,omitempty"`
	RankBy interface{} `json:"rank_by,omitempty"`
	IncludeGroundTruth bool `json:"include_ground_truth,omitempty"`
}

type TurbopufferRecallResponse struct {
	AvgRecall float64 `json:"avg_recall"`
	AvgExhaustiveCount float64 `json:"avg_exhaustive_count"`
	AvgAnnCount float64 `json:"avg_ann_count"`
	GroundTruth []map[string]interface{} `json:"ground_truth,omitempty"`
}

type HintCacheWarmResponse struct {
	Status string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

type JobStatus string

type SnapshotSource string

type ScanSource string

type ScanCountSource string

type ScanMode string

type ScanCountServedBy string

type CreateSnapshotRequest struct {
	Field string `json:"field"`
	Source SnapshotSource `json:"source,omitempty"`
	Filters interface{} `json:"filters,omitempty"`
	PageSize int64 `json:"page_size,omitempty"`
}

type CreateScanRequest struct {
	Source ScanCountSource `json:"source,omitempty"`
	Filters interface{} `json:"filters,omitempty"`
	Fts FtsScan `json:"fts,omitempty"`
	Ann AnnScan `json:"ann,omitempty"`
	Mode ScanMode `json:"mode,omitempty"`
	Exhaustive bool `json:"exhaustive,omitempty"`
	Threads int64 `json:"threads,omitempty"`
	PageSize int64 `json:"page_size,omitempty"`
	TimeoutSeconds int64 `json:"timeout_seconds,omitempty"`
}

type FtsScan struct {
	Field string `json:"field"`
	Query string `json:"query"`
}

type AnnScan struct {
	Vector []float64 `json:"vector"`
	Field string `json:"field,omitempty"`
	Radius float64 `json:"radius"`
}

type WarmStepStatus string

type WarmStepResponse struct {
	Enabled bool `json:"enabled"`
	Status WarmStepStatus `json:"status"`
}

type WarmDocumentsResponse struct {
	Enabled bool `json:"enabled"`
	Status WarmStepStatus `json:"status"`
	Job WarmJob `json:"job,omitempty"`
}

type WarmSnapshotsResponse struct {
	Enabled bool `json:"enabled"`
	Status WarmStepStatus `json:"status"`
	Key string `json:"key,omitempty"`
	WatermarkMs int64 `json:"watermark_ms,omitempty"`
	Sha string `json:"sha,omitempty"`
}

type WarmCacheResponse struct {
	Namespace string `json:"namespace"`
	Turbopuffer WarmStepResponse `json:"turbopuffer"`
	Documents WarmDocumentsResponse `json:"documents"`
	Snapshots WarmSnapshotsResponse `json:"snapshots"`
}

type JobBase struct {
	ID string `json:"id"`
	Namespace string `json:"namespace"`
	Status JobStatus `json:"status"`
	Progress float64 `json:"progress"`
	DocumentsScanned int64 `json:"documents_scanned"`
	StableAsOf int64 `json:"stable_as_of,omitempty"`
	CreatedAt string `json:"created_at"`
	CompletedAt string `json:"completed_at,omitempty"`
	Error string `json:"error,omitempty"`
}

type SnapshotJob struct {
	ID string `json:"id"`
	Namespace string `json:"namespace"`
	Status JobStatus `json:"status"`
	Progress float64 `json:"progress"`
	DocumentsScanned int64 `json:"documents_scanned"`
	StableAsOf int64 `json:"stable_as_of,omitempty"`
	CreatedAt string `json:"created_at"`
	CompletedAt string `json:"completed_at,omitempty"`
	Error string `json:"error,omitempty"`
	Field string `json:"field"`
	Source SnapshotSource `json:"source"`
	EffectiveSource SnapshotSource `json:"effective_source,omitempty"`
	Sha string `json:"sha,omitempty"`
}

type SnapshotJobList struct {
	SnapshotJobs []SnapshotJob `json:"snapshot_jobs"`
}

type WarmJob struct {
	ID string `json:"id"`
	Namespace string `json:"namespace"`
	Status JobStatus `json:"status"`
	Progress float64 `json:"progress"`
	DocumentsScanned int64 `json:"documents_scanned"`
	StableAsOf int64 `json:"stable_as_of,omitempty"`
	CreatedAt string `json:"created_at"`
	CompletedAt string `json:"completed_at,omitempty"`
	Error string `json:"error,omitempty"`
}

type WarmJobList struct {
	WarmJobs []WarmJob `json:"warm_jobs"`
}

type ScanJob struct {
	ID string `json:"id"`
	Namespace string `json:"namespace"`
	Status JobStatus `json:"status"`
	Progress float64 `json:"progress"`
	DocumentsScanned int64 `json:"documents_scanned"`
	StableAsOf int64 `json:"stable_as_of,omitempty"`
	CreatedAt string `json:"created_at"`
	CompletedAt string `json:"completed_at,omitempty"`
	Error string `json:"error,omitempty"`
	Source ScanSource `json:"source"`
	EffectiveSource ScanSource `json:"effective_source,omitempty"`
	Threads int64 `json:"threads,omitempty"`
}

type ScanJobList struct {
	Scans []ScanJob `json:"scans"`
}

type FieldValueResult struct {
	Value string `json:"value"`
	DocCount int64 `json:"doc_count"`
}

type ScanIdsResponse struct {
	Ids []string `json:"ids"`
	Total int64 `json:"total"`
}

type ScanCountResponse struct {
	Count int64 `json:"count"`
	ServedBy ScanCountServedBy `json:"served_by"`
	SnapshotSha string `json:"snapshot_sha,omitempty"`
	WatermarkMs int64 `json:"watermark_ms,omitempty"`
	Bounded bool `json:"bounded,omitempty"`
	TimedOut bool `json:"timed_out,omitempty"`
	ShardsSaturated int64 `json:"shards_saturated,omitempty"`
	ShardsTotal int64 `json:"shards_total,omitempty"`
	Approximate bool `json:"approximate,omitempty"`
	Threads int64 `json:"threads,omitempty"`
	ElapsedMs int64 `json:"elapsed_ms"`
}

type NamespaceList struct {
	Namespaces []NamespaceListEntry `json:"namespaces"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type NamespaceListEntry struct {
	Name string `json:"name"`
	RowCount int64 `json:"row_count,omitempty"`
	SizeBytes int64 `json:"size_bytes,omitempty"`
	StableAsOfMs int64 `json:"stable_as_of_ms,omitempty"`
	IsStable bool `json:"is_stable,omitempty"`
	SchemaSummary NamespaceSchemaSummary `json:"schema_summary,omitempty"`
	Index IndexState `json:"index,omitempty"`
	CacheState NamespaceCacheState `json:"cache_state,omitempty"`
	LastWriteMs int64 `json:"last_write_ms,omitempty"`
	Shadow bool `json:"shadow,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
	MetadataError string `json:"metadata_error,omitempty"`
}

type NamespaceSchemaSummary struct {
	VectorDim int64 `json:"vector_dim,omitempty"`
	Fields []string `json:"fields,omitempty"`
}

type IndexState struct {
	Status string `json:"status,omitempty"`
	UnindexedBytes int64 `json:"unindexed_bytes,omitempty"`
}

type NamespaceCacheState struct {
	State string `json:"state"`
	WarmedThroughMs int64 `json:"warmed_through_ms,omitempty"`
	WarmInflight bool `json:"warm_inflight"`
}

type NamespaceMetadata struct {
	ID string `json:"id"`
	Schema map[string]interface{} `json:"schema"`
	ApproxLogicalBytes int64 `json:"approx_logical_bytes"`
	ApproxRowCount int64 `json:"approx_row_count"`
	CreatedAt string `json:"created_at"`
	LastWriteAt string `json:"last_write_at,omitempty"`
	UpdatedAt string `json:"updated_at"`
	Config map[string]interface{} `json:"config,omitempty"`
}

type QueryRequest struct {
	Vector []float64 `json:"vector,omitempty"`
	NearestToID []string `json:"nearest_to_id,omitempty"`
	TopK int64 `json:"top_k,omitempty"`
	Filters interface{} `json:"filters,omitempty"`
	IncludeAttributes interface{} `json:"include_attributes,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type QueryResponse struct {
	Rows []map[string]interface{} `json:"rows"`
	Aggregations map[string]interface{} `json:"aggregations,omitempty"`
	AggregationGroups []map[string]interface{} `json:"aggregation_groups,omitempty"`
	Billing map[string]interface{} `json:"billing,omitempty"`
	Performance map[string]interface{} `json:"performance,omitempty"`
	StableAsOf int64 `json:"stable_as_of,omitempty"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type Error struct {
	Error string `json:"error"`
	Message string `json:"message"`
}

type SnapshotHistoryEntry struct {
	WatermarkMs int64 `json:"watermark_ms"`
	Sha string `json:"sha"`
}

type SnapshotBody struct {
	Namespace string `json:"namespace"`
	WatermarkMs int64 `json:"watermark_ms"`
	Sha string `json:"sha"`
	Fields []SnapshotField `json:"fields"`
	FieldsSkipped []SnapshotFieldSkipped `json:"fields_skipped"`
}

type SnapshotField struct {
	Name string `json:"name"`
	Values []SnapshotValueCount `json:"values"`
}

type SnapshotSkipReason string

type SnapshotFieldSkipped struct {
	Name string `json:"name"`
	Reason SnapshotSkipReason `json:"reason"`
	DistinctObserved int64 `json:"distinct_observed"`
	Cap int64 `json:"cap"`
}

type SnapshotValueCount struct {
	V string `json:"v"`
	N int64 `json:"n"`
}

type SnapshotActivityEvent struct {
	TsMs int64 `json:"ts_ms"`
	Namespace string `json:"namespace"`
	Sha string `json:"sha"`
}

type SnapshotActivityList struct {
	Events []SnapshotActivityEvent `json:"events"`
	NextCursor string `json:"next_cursor,omitempty"`
	Truncated bool `json:"truncated,omitempty"`
}

type MetricKind string

type MetricFamily string

type MetricAlert struct {
	Summary string `json:"summary"`
	Expr string `json:"expr"`
	For string `json:"for"`
}

type MetricCatalogEntry struct {
	Name string `json:"name"`
	Kind MetricKind `json:"kind"`
	Family MetricFamily `json:"family"`
	Labels []string `json:"labels"`
	Description string `json:"description"`
	ExamplePromql string `json:"example_promql"`
	Alert MetricAlert `json:"alert,omitempty"`
}

type MetricCatalog struct {
	Version string `json:"version"`
	Entries []MetricCatalogEntry `json:"entries"`
}

type PrometheusResponse map[string]interface{}

type SearchHistoryEntry struct {
	Timestamp string `json:"timestamp"`
	TimestampNanos int64 `json:"timestamp_nanos"`
	Namespace string `json:"namespace"`
	TraceID string `json:"trace_id,omitempty"`
	RawQuery string `json:"raw_query,omitempty"`
	StableAsOf int64 `json:"stable_as_of,omitempty"`
	Query map[string]interface{} `json:"query"`
	TopResultIds []string `json:"top_result_ids"`
	Tags []string `json:"tags"`
}

type SearchHistoryListResponse struct {
	Entries []SearchHistoryEntry `json:"entries"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type ClickstreamEvent struct {
	Timestamp string `json:"timestamp"`
	TimestampNanos int64 `json:"timestamp_nanos"`
	TraceID string `json:"trace_id"`
	Namespace string `json:"namespace"`
	DocID string `json:"doc_id"`
	Tags []string `json:"tags"`
	Source string `json:"source"`
	ServedFrom string `json:"served_from"`
}

type ClickstreamListResponse struct {
	Events []ClickstreamEvent `json:"events"`
	NextCursor string `json:"next_cursor,omitempty"`
}
