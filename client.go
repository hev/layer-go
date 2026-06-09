package hevlayer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const DefaultBaseURL = "https://aws-us-east-1.hevlayer.com"
const DefaultTurbopufferBaseURL = "https://aws-us-east-1.turbopuffer.com"

const searchHistoryMaxTags = 32
const searchHistoryMaxTagLength = 128

type Client struct {
	baseURL string
	apiKey string
	httpClient *http.Client
	turbopufferBaseURL string
	turbopufferAPIKey string
	turbopufferHTTPClient *http.Client
	turbopufferHTTPClientExplicit bool
	fallbackToTurbopuffer bool
}

type Option func(*Client)

func NewClient(options ...Option) *Client {
	client := &Client{
		baseURL: DefaultBaseURL,
		turbopufferBaseURL: DefaultTurbopufferBaseURL,
		httpClient: http.DefaultClient,
		turbopufferHTTPClient: http.DefaultClient,
		turbopufferAPIKey: cleanToken(os.Getenv("TURBOPUFFER_API_KEY")),
		fallbackToTurbopuffer: true,
	}
	if baseURL := strings.TrimSpace(os.Getenv("TURBOPUFFER_API_URL")); baseURL != "" {
		client.turbopufferBaseURL = strings.TrimRight(baseURL, "/")
	}
	for _, option := range options {
		option(client)
	}
	return client
}

func WithBaseURL(baseURL string) Option {
	return func(client *Client) {
		if strings.TrimSpace(baseURL) != "" {
			client.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		}
	}
}

func WithAPIKey(apiKey string) Option {
	return func(client *Client) {
		client.apiKey = strings.TrimSpace(apiKey)
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

func WithTurbopufferAPIKey(apiKey string) Option {
	return func(client *Client) {
		client.turbopufferAPIKey = cleanToken(apiKey)
	}
}

func WithTurbopufferBaseURL(baseURL string) Option {
	return func(client *Client) {
		if strings.TrimSpace(baseURL) != "" {
			client.turbopufferBaseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		}
	}
}

func WithTurbopufferHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient != nil {
			client.turbopufferHTTPClient = httpClient
			client.turbopufferHTTPClientExplicit = true
		}
	}
}

func WithFallbackToTurbopuffer(enabled bool) Option {
	return func(client *Client) {
		client.fallbackToTurbopuffer = enabled
	}
}

type LayerPerf struct {
	LatencyMS float64 `json:"latency_ms"`
	CacheStatus string `json:"cache_status,omitempty"`
	Fallback string `json:"fallback,omitempty"`
}

type LayerResponse[T any] struct {
	Data T `json:"data"`
	Perf LayerPerf `json:"perf"`
}

type RequestOption func(*requestOptions)

func WithSearchQuery(rawQuery string) RequestOption {
	return func(options *requestOptions) {
		query := strings.TrimSpace(rawQuery)
		if query != "" {
			options.headers.Set("x-hevlayer-search-query", query)
		}
	}
}

func WithSearchTags(tags []string) RequestOption {
	return func(options *requestOptions) {
		cleaned, err := cleanHistoryTags(tags)
		if err != nil {
			options.err = err
			return
		}
		if len(cleaned) > 0 {
			options.headers.Set("x-hevlayer-tags", strings.Join(cleaned, ","))
		}
	}
}

type requestOptions struct {
	headers http.Header
	err error
}

type turbopufferFallback struct {
	method string
	path string
	transform string
}

type HevlayerError struct {
	StatusCode int
	Kind string
	Message string
	Body []byte
}

func (e *HevlayerError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("hevlayer: status %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("hevlayer: status %d", e.StatusCode)
}

type FetchDocumentParams struct {
	IncludeAttributes []string `json:"include_attributes,omitempty"`
}

func (params *FetchDocumentParams) query() (url.Values, error) {
	query := url.Values{}
	if params == nil {
		return query, nil
	}
	if err := addQueryValue(query, "include_attributes", params.IncludeAttributes); err != nil {
		return nil, err
	}
	return query, nil
}

type GetScanResultsParams struct {
	Limit int64 `json:"limit,omitempty"`
	Offset int64 `json:"offset,omitempty"`
}

func (params *GetScanResultsParams) query() (url.Values, error) {
	query := url.Values{}
	if params == nil {
		return query, nil
	}
	if err := addQueryValue(query, "limit", params.Limit); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "offset", params.Offset); err != nil {
		return nil, err
	}
	return query, nil
}

type HintCacheWarmParams struct {
	Turbopuffer bool `json:"turbopuffer,omitempty"`
	Documents bool `json:"documents,omitempty"`
	Snapshots bool `json:"snapshots,omitempty"`
	PageSize int64 `json:"page_size,omitempty"`
}

func (params *HintCacheWarmParams) query() (url.Values, error) {
	query := url.Values{}
	if params == nil {
		return query, nil
	}
	if err := addQueryValue(query, "turbopuffer", params.Turbopuffer); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "documents", params.Documents); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "snapshots", params.Snapshots); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "page_size", params.PageSize); err != nil {
		return nil, err
	}
	return query, nil
}

type ListClickstreamParams struct {
	TraceID string `json:"trace_id,omitempty"`
	Tag []string `json:"tag,omitempty"`
	From string `json:"from,omitempty"`
	To string `json:"to,omitempty"`
	Before string `json:"before,omitempty"`
	Limit int64 `json:"limit,omitempty"`
}

func (params *ListClickstreamParams) query() (url.Values, error) {
	query := url.Values{}
	if params == nil {
		return query, nil
	}
	if err := addQueryValue(query, "trace_id", params.TraceID); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "tag", params.Tag); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "from", params.From); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "to", params.To); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "before", params.Before); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "limit", params.Limit); err != nil {
		return nil, err
	}
	return query, nil
}

type ListMetricsCatalogParams struct {
	Family MetricFamily `json:"family,omitempty"`
}

func (params *ListMetricsCatalogParams) query() (url.Values, error) {
	query := url.Values{}
	if params == nil {
		return query, nil
	}
	if err := addQueryValue(query, "family", params.Family); err != nil {
		return nil, err
	}
	return query, nil
}

type ListNamespaceHistoryParams struct {
	Limit int64 `json:"limit,omitempty"`
	Before string `json:"before,omitempty"`
}

func (params *ListNamespaceHistoryParams) query() (url.Values, error) {
	query := url.Values{}
	if params == nil {
		return query, nil
	}
	if err := addQueryValue(query, "limit", params.Limit); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "before", params.Before); err != nil {
		return nil, err
	}
	return query, nil
}

type ListNamespacesParams struct {
	Prefix string `json:"prefix,omitempty"`
	Cursor string `json:"cursor,omitempty"`
	PageSize int64 `json:"page_size,omitempty"`
}

func (params *ListNamespacesParams) query() (url.Values, error) {
	query := url.Values{}
	if params == nil {
		return query, nil
	}
	if err := addQueryValue(query, "prefix", params.Prefix); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "cursor", params.Cursor); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "page_size", params.PageSize); err != nil {
		return nil, err
	}
	return query, nil
}

type ListSearchHistoryParams struct {
	Tag []string `json:"tag,omitempty"`
	From string `json:"from,omitempty"`
	To string `json:"to,omitempty"`
	Before string `json:"before,omitempty"`
	Limit int64 `json:"limit,omitempty"`
}

func (params *ListSearchHistoryParams) query() (url.Values, error) {
	query := url.Values{}
	if params == nil {
		return query, nil
	}
	if err := addQueryValue(query, "tag", params.Tag); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "from", params.From); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "to", params.To); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "before", params.Before); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "limit", params.Limit); err != nil {
		return nil, err
	}
	return query, nil
}

type ListSnapshotActivityParams struct {
	Since int64 `json:"since,omitempty"`
	Limit int64 `json:"limit,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

func (params *ListSnapshotActivityParams) query() (url.Values, error) {
	query := url.Values{}
	if params == nil {
		return query, nil
	}
	if err := addQueryValue(query, "since", params.Since); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "limit", params.Limit); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "namespace", params.Namespace); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "cursor", params.Cursor); err != nil {
		return nil, err
	}
	return query, nil
}

type ListTurbopufferNamespacesParams struct {
	Cursor string `json:"cursor,omitempty"`
	Prefix string `json:"prefix,omitempty"`
	PageSize int64 `json:"page_size,omitempty"`
}

func (params *ListTurbopufferNamespacesParams) query() (url.Values, error) {
	query := url.Values{}
	if params == nil {
		return query, nil
	}
	if err := addQueryValue(query, "cursor", params.Cursor); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "prefix", params.Prefix); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "page_size", params.PageSize); err != nil {
		return nil, err
	}
	return query, nil
}

type QueryMetricsParams struct {
	Query string `json:"query,omitempty"`
	Time string `json:"time,omitempty"`
	Timeout string `json:"timeout,omitempty"`
}

func (params *QueryMetricsParams) query() (url.Values, error) {
	query := url.Values{}
	if params == nil {
		return query, nil
	}
	if err := addQueryValue(query, "query", params.Query); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "time", params.Time); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "timeout", params.Timeout); err != nil {
		return nil, err
	}
	return query, nil
}

type QueryMetricsApiV1Params struct {
	Query string `json:"query,omitempty"`
	Time string `json:"time,omitempty"`
	Timeout string `json:"timeout,omitempty"`
}

func (params *QueryMetricsApiV1Params) query() (url.Values, error) {
	query := url.Values{}
	if params == nil {
		return query, nil
	}
	if err := addQueryValue(query, "query", params.Query); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "time", params.Time); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "timeout", params.Timeout); err != nil {
		return nil, err
	}
	return query, nil
}

type QueryMetricsRangeParams struct {
	Query string `json:"query,omitempty"`
	Start string `json:"start,omitempty"`
	End string `json:"end,omitempty"`
	Step string `json:"step,omitempty"`
	Timeout string `json:"timeout,omitempty"`
}

func (params *QueryMetricsRangeParams) query() (url.Values, error) {
	query := url.Values{}
	if params == nil {
		return query, nil
	}
	if err := addQueryValue(query, "query", params.Query); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "start", params.Start); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "end", params.End); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "step", params.Step); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "timeout", params.Timeout); err != nil {
		return nil, err
	}
	return query, nil
}

type QueryMetricsRangeApiV1Params struct {
	Query string `json:"query,omitempty"`
	Start string `json:"start,omitempty"`
	End string `json:"end,omitempty"`
	Step string `json:"step,omitempty"`
	Timeout string `json:"timeout,omitempty"`
}

func (params *QueryMetricsRangeApiV1Params) query() (url.Values, error) {
	query := url.Values{}
	if params == nil {
		return query, nil
	}
	if err := addQueryValue(query, "query", params.Query); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "start", params.Start); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "end", params.End); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "step", params.Step); err != nil {
		return nil, err
	}
	if err := addQueryValue(query, "timeout", params.Timeout); err != nil {
		return nil, err
	}
	return query, nil
}

type WarmCacheParams struct {
	PageSize int64 `json:"page_size,omitempty"`
}

func (params *WarmCacheParams) query() (url.Values, error) {
	query := url.Values{}
	if params == nil {
		return query, nil
	}
	if err := addQueryValue(query, "page_size", params.PageSize); err != nil {
		return nil, err
	}
	return query, nil
}

func (client *Client) BranchNamespace(ctx context.Context, namespace string, body *TurbopufferBranchFromRequest, options ...RequestOption) (*TurbopufferWriteResponse, error) {
	out := TurbopufferWriteResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s", url.PathEscape(namespace)), func() url.Values {
		query := url.Values{}
		query.Set("stainless_overload", "branchFrom")
		return query
	}(), body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v2/namespaces/%s", url.PathEscape(namespace))}, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) BranchNamespaceWithPerf(ctx context.Context, namespace string, body *TurbopufferBranchFromRequest, options ...RequestOption) (*LayerResponse[TurbopufferWriteResponse], error) {
	out := TurbopufferWriteResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s", url.PathEscape(namespace)), func() url.Values {
		query := url.Values{}
		query.Set("stainless_overload", "branchFrom")
		return query
	}(), body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v2/namespaces/%s", url.PathEscape(namespace))}, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[TurbopufferWriteResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) ClaimDocuments(ctx context.Context, pipelineID string, body *ClaimDocumentsRequest, options ...RequestOption) (*ClaimDocumentsResponse, error) {
	out := ClaimDocumentsResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/pipelines/%s/claim", url.PathEscape(pipelineID)), url.Values{}, body, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) ClaimDocumentsWithPerf(ctx context.Context, pipelineID string, body *ClaimDocumentsRequest, options ...RequestOption) (*LayerResponse[ClaimDocumentsResponse], error) {
	out := ClaimDocumentsResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/pipelines/%s/claim", url.PathEscape(pipelineID)), url.Values{}, body, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[ClaimDocumentsResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) ClaimUdfItems(ctx context.Context, udfID string, body *UdfClaimRequest, options ...RequestOption) (*UdfClaimResponse, error) {
	out := UdfClaimResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/udfs/%s/claim", url.PathEscape(udfID)), url.Values{}, body, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) ClaimUdfItemsWithPerf(ctx context.Context, udfID string, body *UdfClaimRequest, options ...RequestOption) (*LayerResponse[UdfClaimResponse], error) {
	out := UdfClaimResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/udfs/%s/claim", url.PathEscape(udfID)), url.Values{}, body, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[UdfClaimResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) CompleteUdfItems(ctx context.Context, udfID string, body *UdfCompleteRequest, options ...RequestOption) (*UdfItemsResponse, error) {
	out := UdfItemsResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/udfs/%s/items/complete", url.PathEscape(udfID)), url.Values{}, body, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) CompleteUdfItemsWithPerf(ctx context.Context, udfID string, body *UdfCompleteRequest, options ...RequestOption) (*LayerResponse[UdfItemsResponse], error) {
	out := UdfItemsResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/udfs/%s/items/complete", url.PathEscape(udfID)), url.Values{}, body, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[UdfItemsResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) CopyNamespace(ctx context.Context, namespace string, body *TurbopufferCopyFromRequest, options ...RequestOption) (*TurbopufferWriteResponse, error) {
	out := TurbopufferWriteResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s", url.PathEscape(namespace)), func() url.Values {
		query := url.Values{}
		query.Set("stainless_overload", "copyFrom")
		return query
	}(), body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v2/namespaces/%s", url.PathEscape(namespace))}, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) CopyNamespaceWithPerf(ctx context.Context, namespace string, body *TurbopufferCopyFromRequest, options ...RequestOption) (*LayerResponse[TurbopufferWriteResponse], error) {
	out := TurbopufferWriteResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s", url.PathEscape(namespace)), func() url.Values {
		query := url.Values{}
		query.Set("stainless_overload", "copyFrom")
		return query
	}(), body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v2/namespaces/%s", url.PathEscape(namespace))}, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[TurbopufferWriteResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) CreatePipeline(ctx context.Context, body *CreatePipelineRequest, options ...RequestOption) (*Pipeline, error) {
	out := Pipeline{}
	if _, err := client.request(ctx, "POST", "/v2/pipelines", url.Values{}, body, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) CreatePipelineWithPerf(ctx context.Context, body *CreatePipelineRequest, options ...RequestOption) (*LayerResponse[Pipeline], error) {
	out := Pipeline{}
	perf, err := client.request(ctx, "POST", "/v2/pipelines", url.Values{}, body, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[Pipeline]{Data: out, Perf: *perf}, nil
}


func (client *Client) CreateScan(ctx context.Context, namespace string, body *CreateScanRequest, options ...RequestOption) (interface{}, error) {
	var out interface{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s/scans", url.PathEscape(namespace)), url.Values{}, body, &out, nil, options...); err != nil {
		return nil, err
	}
	return out, nil
}

func (client *Client) CreateScanWithPerf(ctx context.Context, namespace string, body *CreateScanRequest, options ...RequestOption) (*LayerResponse[interface{}], error) {
	var out interface{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s/scans", url.PathEscape(namespace)), url.Values{}, body, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[interface{}]{Data: out, Perf: *perf}, nil
}


func (client *Client) CreateSnapshot(ctx context.Context, namespace string, body *CreateSnapshotRequest, options ...RequestOption) (*SnapshotJob, error) {
	out := SnapshotJob{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s/snapshots", url.PathEscape(namespace)), url.Values{}, body, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) CreateSnapshotWithPerf(ctx context.Context, namespace string, body *CreateSnapshotRequest, options ...RequestOption) (*LayerResponse[SnapshotJob], error) {
	out := SnapshotJob{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s/snapshots", url.PathEscape(namespace)), url.Values{}, body, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[SnapshotJob]{Data: out, Perf: *perf}, nil
}


func (client *Client) CreateUdf(ctx context.Context, body *CreateUdfRequest, options ...RequestOption) (*Udf, error) {
	out := Udf{}
	if _, err := client.request(ctx, "POST", "/v2/udfs", url.Values{}, body, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) CreateUdfWithPerf(ctx context.Context, body *CreateUdfRequest, options ...RequestOption) (*LayerResponse[Udf], error) {
	out := Udf{}
	perf, err := client.request(ctx, "POST", "/v2/udfs", url.Values{}, body, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[Udf]{Data: out, Perf: *perf}, nil
}


func (client *Client) DeleteNamespace(ctx context.Context, namespace string, options ...RequestOption) (*StatusResponse, error) {
	out := StatusResponse{}
	if _, err := client.request(ctx, "DELETE", fmt.Sprintf("/v2/namespaces/%s", url.PathEscape(namespace)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) DeleteNamespaceWithPerf(ctx context.Context, namespace string, options ...RequestOption) (*LayerResponse[StatusResponse], error) {
	out := StatusResponse{}
	perf, err := client.request(ctx, "DELETE", fmt.Sprintf("/v2/namespaces/%s", url.PathEscape(namespace)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[StatusResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) DeletePipeline(ctx context.Context, pipelineID string, options ...RequestOption) (*StatusResponse, error) {
	out := StatusResponse{}
	if _, err := client.request(ctx, "DELETE", fmt.Sprintf("/v2/pipelines/%s", url.PathEscape(pipelineID)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) DeletePipelineWithPerf(ctx context.Context, pipelineID string, options ...RequestOption) (*LayerResponse[StatusResponse], error) {
	out := StatusResponse{}
	perf, err := client.request(ctx, "DELETE", fmt.Sprintf("/v2/pipelines/%s", url.PathEscape(pipelineID)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[StatusResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) DeleteScan(ctx context.Context, namespace string, scanID string, options ...RequestOption) (*StatusResponse, error) {
	out := StatusResponse{}
	if _, err := client.request(ctx, "DELETE", fmt.Sprintf("/v2/namespaces/%s/scans/%s", url.PathEscape(namespace), url.PathEscape(scanID)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) DeleteScanWithPerf(ctx context.Context, namespace string, scanID string, options ...RequestOption) (*LayerResponse[StatusResponse], error) {
	out := StatusResponse{}
	perf, err := client.request(ctx, "DELETE", fmt.Sprintf("/v2/namespaces/%s/scans/%s", url.PathEscape(namespace), url.PathEscape(scanID)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[StatusResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) DeleteUdf(ctx context.Context, udfID string, options ...RequestOption) (*StatusResponse, error) {
	out := StatusResponse{}
	if _, err := client.request(ctx, "DELETE", fmt.Sprintf("/v2/udfs/%s", url.PathEscape(udfID)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) DeleteUdfWithPerf(ctx context.Context, udfID string, options ...RequestOption) (*LayerResponse[StatusResponse], error) {
	out := StatusResponse{}
	perf, err := client.request(ctx, "DELETE", fmt.Sprintf("/v2/udfs/%s", url.PathEscape(udfID)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[StatusResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) DiscoverUdf(ctx context.Context, udfID string, body *UdfDiscoverRequest, options ...RequestOption) (*UdfDiscoverResponse, error) {
	out := UdfDiscoverResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/udfs/%s/discover", url.PathEscape(udfID)), url.Values{}, body, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) DiscoverUdfWithPerf(ctx context.Context, udfID string, body *UdfDiscoverRequest, options ...RequestOption) (*LayerResponse[UdfDiscoverResponse], error) {
	out := UdfDiscoverResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/udfs/%s/discover", url.PathEscape(udfID)), url.Values{}, body, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[UdfDiscoverResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) EvaluateTurbopufferRecall(ctx context.Context, namespace string, body *TurbopufferRecallRequest, options ...RequestOption) (*TurbopufferRecallResponse, error) {
	out := TurbopufferRecallResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/_debug/recall", url.PathEscape(namespace)), url.Values{}, body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v1/namespaces/%s/_debug/recall", url.PathEscape(namespace))}, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) EvaluateTurbopufferRecallWithPerf(ctx context.Context, namespace string, body *TurbopufferRecallRequest, options ...RequestOption) (*LayerResponse[TurbopufferRecallResponse], error) {
	out := TurbopufferRecallResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/_debug/recall", url.PathEscape(namespace)), url.Values{}, body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v1/namespaces/%s/_debug/recall", url.PathEscape(namespace))}, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[TurbopufferRecallResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) ExplainTurbopufferQuery(ctx context.Context, namespace string, body TurbopufferQueryRequest, options ...RequestOption) (*TurbopufferExplainQueryResponse, error) {
	out := TurbopufferExplainQueryResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s/explain_query", url.PathEscape(namespace)), url.Values{}, body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v2/namespaces/%s/explain_query", url.PathEscape(namespace))}, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) ExplainTurbopufferQueryWithPerf(ctx context.Context, namespace string, body TurbopufferQueryRequest, options ...RequestOption) (*LayerResponse[TurbopufferExplainQueryResponse], error) {
	out := TurbopufferExplainQueryResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s/explain_query", url.PathEscape(namespace)), url.Values{}, body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v2/namespaces/%s/explain_query", url.PathEscape(namespace))}, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[TurbopufferExplainQueryResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) FailUdfItems(ctx context.Context, udfID string, body *UdfFailRequest, options ...RequestOption) (*UdfItemsResponse, error) {
	out := UdfItemsResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/udfs/%s/items/fail", url.PathEscape(udfID)), url.Values{}, body, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) FailUdfItemsWithPerf(ctx context.Context, udfID string, body *UdfFailRequest, options ...RequestOption) (*LayerResponse[UdfItemsResponse], error) {
	out := UdfItemsResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/udfs/%s/items/fail", url.PathEscape(udfID)), url.Values{}, body, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[UdfItemsResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) FetchDocument(ctx context.Context, namespace string, docID string, params *FetchDocumentParams, options ...RequestOption) (*Document, error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := Document{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/documents/%s", url.PathEscape(namespace), url.PathEscape(docID)), query, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) FetchDocumentWithPerf(ctx context.Context, namespace string, docID string, params *FetchDocumentParams, options ...RequestOption) (*LayerResponse[Document], error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := Document{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/documents/%s", url.PathEscape(namespace), url.PathEscape(docID)), query, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[Document]{Data: out, Perf: *perf}, nil
}


func (client *Client) FetchDocuments(ctx context.Context, namespace string, body *FetchDocumentsRequest, options ...RequestOption) (*FetchDocumentsResponse, error) {
	out := FetchDocumentsResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s/documents", url.PathEscape(namespace)), url.Values{}, body, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) FetchDocumentsWithPerf(ctx context.Context, namespace string, body *FetchDocumentsRequest, options ...RequestOption) (*LayerResponse[FetchDocumentsResponse], error) {
	out := FetchDocumentsResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s/documents", url.PathEscape(namespace)), url.Values{}, body, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[FetchDocumentsResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) GetMetricCatalogEntry(ctx context.Context, name string, options ...RequestOption) (*MetricCatalogEntry, error) {
	out := MetricCatalogEntry{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/metrics/catalog/%s", url.PathEscape(name)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) GetMetricCatalogEntryWithPerf(ctx context.Context, name string, options ...RequestOption) (*LayerResponse[MetricCatalogEntry], error) {
	out := MetricCatalogEntry{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/metrics/catalog/%s", url.PathEscape(name)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[MetricCatalogEntry]{Data: out, Perf: *perf}, nil
}


func (client *Client) GetNamespaceMetadata(ctx context.Context, namespace string, options ...RequestOption) (*NamespaceMetadata, error) {
	out := NamespaceMetadata{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/metadata", url.PathEscape(namespace)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) GetNamespaceMetadataWithPerf(ctx context.Context, namespace string, options ...RequestOption) (*LayerResponse[NamespaceMetadata], error) {
	out := NamespaceMetadata{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/metadata", url.PathEscape(namespace)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[NamespaceMetadata]{Data: out, Perf: *perf}, nil
}


func (client *Client) GetNamespaceSnapshot(ctx context.Context, namespace string, sha string, options ...RequestOption) (*SnapshotBody, error) {
	out := SnapshotBody{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/snapshots/%s", url.PathEscape(namespace), url.PathEscape(sha)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) GetNamespaceSnapshotWithPerf(ctx context.Context, namespace string, sha string, options ...RequestOption) (*LayerResponse[SnapshotBody], error) {
	out := SnapshotBody{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/snapshots/%s", url.PathEscape(namespace), url.PathEscape(sha)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[SnapshotBody]{Data: out, Perf: *perf}, nil
}


func (client *Client) GetPipelineDocumentChunks(ctx context.Context, pipelineID string, docID string, options ...RequestOption) (*GetChunksResponse, error) {
	out := GetChunksResponse{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/pipelines/%s/documents/%s/chunks", url.PathEscape(pipelineID), url.PathEscape(docID)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) GetPipelineDocumentChunksWithPerf(ctx context.Context, pipelineID string, docID string, options ...RequestOption) (*LayerResponse[GetChunksResponse], error) {
	out := GetChunksResponse{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/pipelines/%s/documents/%s/chunks", url.PathEscape(pipelineID), url.PathEscape(docID)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[GetChunksResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) GetPipelineStatus(ctx context.Context, pipelineID string, options ...RequestOption) (*PipelineStatus, error) {
	out := PipelineStatus{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/pipelines/%s/status", url.PathEscape(pipelineID)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) GetPipelineStatusWithPerf(ctx context.Context, pipelineID string, options ...RequestOption) (*LayerResponse[PipelineStatus], error) {
	out := PipelineStatus{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/pipelines/%s/status", url.PathEscape(pipelineID)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[PipelineStatus]{Data: out, Perf: *perf}, nil
}


func (client *Client) GetScan(ctx context.Context, namespace string, scanID string, options ...RequestOption) (*ScanJob, error) {
	out := ScanJob{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/scans/%s", url.PathEscape(namespace), url.PathEscape(scanID)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) GetScanWithPerf(ctx context.Context, namespace string, scanID string, options ...RequestOption) (*LayerResponse[ScanJob], error) {
	out := ScanJob{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/scans/%s", url.PathEscape(namespace), url.PathEscape(scanID)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[ScanJob]{Data: out, Perf: *perf}, nil
}


func (client *Client) GetScanResults(ctx context.Context, namespace string, scanID string, params *GetScanResultsParams, options ...RequestOption) (*ScanIdsResponse, error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := ScanIdsResponse{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/scans/%s/results", url.PathEscape(namespace), url.PathEscape(scanID)), query, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) GetScanResultsWithPerf(ctx context.Context, namespace string, scanID string, params *GetScanResultsParams, options ...RequestOption) (*LayerResponse[ScanIdsResponse], error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := ScanIdsResponse{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/scans/%s/results", url.PathEscape(namespace), url.PathEscape(scanID)), query, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[ScanIdsResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) GetSnapshotJob(ctx context.Context, namespace string, jobID string, options ...RequestOption) (*SnapshotJob, error) {
	out := SnapshotJob{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/snapshot-jobs/%s", url.PathEscape(namespace), url.PathEscape(jobID)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) GetSnapshotJobWithPerf(ctx context.Context, namespace string, jobID string, options ...RequestOption) (*LayerResponse[SnapshotJob], error) {
	out := SnapshotJob{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/snapshot-jobs/%s", url.PathEscape(namespace), url.PathEscape(jobID)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[SnapshotJob]{Data: out, Perf: *perf}, nil
}


func (client *Client) GetTurbopufferNamespaceSchema(ctx context.Context, namespace string, options ...RequestOption) (TurbopufferSchema, error) {
	out := TurbopufferSchema{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v1/namespaces/%s/schema", url.PathEscape(namespace)), url.Values{}, nil, &out, &turbopufferFallback{method: "GET", path: fmt.Sprintf("/v1/namespaces/%s/schema", url.PathEscape(namespace))}, options...); err != nil {
		return nil, err
	}
	return out, nil
}

func (client *Client) GetTurbopufferNamespaceSchemaWithPerf(ctx context.Context, namespace string, options ...RequestOption) (*LayerResponse[TurbopufferSchema], error) {
	out := TurbopufferSchema{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v1/namespaces/%s/schema", url.PathEscape(namespace)), url.Values{}, nil, &out, &turbopufferFallback{method: "GET", path: fmt.Sprintf("/v1/namespaces/%s/schema", url.PathEscape(namespace))}, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[TurbopufferSchema]{Data: out, Perf: *perf}, nil
}


func (client *Client) GetTurbopufferV1NamespaceMetadata(ctx context.Context, namespace string, options ...RequestOption) (*NamespaceMetadata, error) {
	out := NamespaceMetadata{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v1/namespaces/%s/metadata", url.PathEscape(namespace)), url.Values{}, nil, &out, &turbopufferFallback{method: "GET", path: fmt.Sprintf("/v1/namespaces/%s/metadata", url.PathEscape(namespace))}, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) GetTurbopufferV1NamespaceMetadataWithPerf(ctx context.Context, namespace string, options ...RequestOption) (*LayerResponse[NamespaceMetadata], error) {
	out := NamespaceMetadata{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v1/namespaces/%s/metadata", url.PathEscape(namespace)), url.Values{}, nil, &out, &turbopufferFallback{method: "GET", path: fmt.Sprintf("/v1/namespaces/%s/metadata", url.PathEscape(namespace))}, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[NamespaceMetadata]{Data: out, Perf: *perf}, nil
}


func (client *Client) GetUdf(ctx context.Context, udfID string, options ...RequestOption) (*GetUdfResponse, error) {
	out := GetUdfResponse{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/udfs/%s", url.PathEscape(udfID)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) GetUdfWithPerf(ctx context.Context, udfID string, options ...RequestOption) (*LayerResponse[GetUdfResponse], error) {
	out := GetUdfResponse{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/udfs/%s", url.PathEscape(udfID)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[GetUdfResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) GetUdfStatus(ctx context.Context, udfID string, options ...RequestOption) (*UdfStatus, error) {
	out := UdfStatus{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/udfs/%s/status", url.PathEscape(udfID)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) GetUdfStatusWithPerf(ctx context.Context, udfID string, options ...RequestOption) (*LayerResponse[UdfStatus], error) {
	out := UdfStatus{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/udfs/%s/status", url.PathEscape(udfID)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[UdfStatus]{Data: out, Perf: *perf}, nil
}


func (client *Client) GetWarmJob(ctx context.Context, namespace string, jobID string, options ...RequestOption) (*WarmJob, error) {
	out := WarmJob{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/warm-jobs/%s", url.PathEscape(namespace), url.PathEscape(jobID)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) GetWarmJobWithPerf(ctx context.Context, namespace string, jobID string, options ...RequestOption) (*LayerResponse[WarmJob], error) {
	out := WarmJob{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/warm-jobs/%s", url.PathEscape(namespace), url.PathEscape(jobID)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[WarmJob]{Data: out, Perf: *perf}, nil
}


func (client *Client) HeartbeatDocuments(ctx context.Context, pipelineID string, body *HeartbeatDocumentsRequest, options ...RequestOption) (*DocumentsStageResponse, error) {
	out := DocumentsStageResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/pipelines/%s/documents/heartbeat", url.PathEscape(pipelineID)), url.Values{}, body, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) HeartbeatDocumentsWithPerf(ctx context.Context, pipelineID string, body *HeartbeatDocumentsRequest, options ...RequestOption) (*LayerResponse[DocumentsStageResponse], error) {
	out := DocumentsStageResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/pipelines/%s/documents/heartbeat", url.PathEscape(pipelineID)), url.Values{}, body, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[DocumentsStageResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) HeartbeatUdfItems(ctx context.Context, udfID string, body *UdfHeartbeatRequest, options ...RequestOption) (*UdfItemsResponse, error) {
	out := UdfItemsResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/udfs/%s/items/heartbeat", url.PathEscape(udfID)), url.Values{}, body, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) HeartbeatUdfItemsWithPerf(ctx context.Context, udfID string, body *UdfHeartbeatRequest, options ...RequestOption) (*LayerResponse[UdfItemsResponse], error) {
	out := UdfItemsResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/udfs/%s/items/heartbeat", url.PathEscape(udfID)), url.Values{}, body, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[UdfItemsResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) HintCacheWarm(ctx context.Context, namespace string, params *HintCacheWarmParams, options ...RequestOption) (*HintCacheWarmResponse, error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := HintCacheWarmResponse{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v1/namespaces/%s/hint_cache_warm", url.PathEscape(namespace)), query, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) HintCacheWarmWithPerf(ctx context.Context, namespace string, params *HintCacheWarmParams, options ...RequestOption) (*LayerResponse[HintCacheWarmResponse], error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := HintCacheWarmResponse{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v1/namespaces/%s/hint_cache_warm", url.PathEscape(namespace)), query, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[HintCacheWarmResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) ListClickstream(ctx context.Context, namespace string, params *ListClickstreamParams, options ...RequestOption) (*ClickstreamListResponse, error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := ClickstreamListResponse{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/clickstream", url.PathEscape(namespace)), query, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) ListClickstreamWithPerf(ctx context.Context, namespace string, params *ListClickstreamParams, options ...RequestOption) (*LayerResponse[ClickstreamListResponse], error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := ClickstreamListResponse{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/clickstream", url.PathEscape(namespace)), query, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[ClickstreamListResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) ListMetricsCatalog(ctx context.Context, params *ListMetricsCatalogParams, options ...RequestOption) (*MetricCatalog, error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := MetricCatalog{}
	if _, err := client.request(ctx, "GET", "/v2/metrics/catalog", query, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) ListMetricsCatalogWithPerf(ctx context.Context, params *ListMetricsCatalogParams, options ...RequestOption) (*LayerResponse[MetricCatalog], error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := MetricCatalog{}
	perf, err := client.request(ctx, "GET", "/v2/metrics/catalog", query, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[MetricCatalog]{Data: out, Perf: *perf}, nil
}


func (client *Client) ListNamespaceHistory(ctx context.Context, namespace string, params *ListNamespaceHistoryParams, options ...RequestOption) ([]SnapshotHistoryEntry, error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := []SnapshotHistoryEntry{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/history", url.PathEscape(namespace)), query, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return out, nil
}

func (client *Client) ListNamespaceHistoryWithPerf(ctx context.Context, namespace string, params *ListNamespaceHistoryParams, options ...RequestOption) (*LayerResponse[[]SnapshotHistoryEntry], error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := []SnapshotHistoryEntry{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/history", url.PathEscape(namespace)), query, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[[]SnapshotHistoryEntry]{Data: out, Perf: *perf}, nil
}


func (client *Client) ListNamespaces(ctx context.Context, params *ListNamespacesParams, options ...RequestOption) (*NamespaceList, error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := NamespaceList{}
	if _, err := client.request(ctx, "GET", "/v2/namespaces", query, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) ListNamespacesWithPerf(ctx context.Context, params *ListNamespacesParams, options ...RequestOption) (*LayerResponse[NamespaceList], error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := NamespaceList{}
	perf, err := client.request(ctx, "GET", "/v2/namespaces", query, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[NamespaceList]{Data: out, Perf: *perf}, nil
}


func (client *Client) ListPipelines(ctx context.Context, options ...RequestOption) (*PipelineList, error) {
	out := PipelineList{}
	if _, err := client.request(ctx, "GET", "/v2/pipelines", url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) ListPipelinesWithPerf(ctx context.Context, options ...RequestOption) (*LayerResponse[PipelineList], error) {
	out := PipelineList{}
	perf, err := client.request(ctx, "GET", "/v2/pipelines", url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[PipelineList]{Data: out, Perf: *perf}, nil
}


func (client *Client) ListScans(ctx context.Context, namespace string, options ...RequestOption) (*ScanJobList, error) {
	out := ScanJobList{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/scans", url.PathEscape(namespace)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) ListScansWithPerf(ctx context.Context, namespace string, options ...RequestOption) (*LayerResponse[ScanJobList], error) {
	out := ScanJobList{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/scans", url.PathEscape(namespace)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[ScanJobList]{Data: out, Perf: *perf}, nil
}


func (client *Client) ListSearchHistory(ctx context.Context, namespace string, params *ListSearchHistoryParams, options ...RequestOption) (*SearchHistoryListResponse, error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := SearchHistoryListResponse{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/search-history", url.PathEscape(namespace)), query, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) ListSearchHistoryWithPerf(ctx context.Context, namespace string, params *ListSearchHistoryParams, options ...RequestOption) (*LayerResponse[SearchHistoryListResponse], error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := SearchHistoryListResponse{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/search-history", url.PathEscape(namespace)), query, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[SearchHistoryListResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) ListSnapshotActivity(ctx context.Context, params *ListSnapshotActivityParams, options ...RequestOption) (*SnapshotActivityList, error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := SnapshotActivityList{}
	if _, err := client.request(ctx, "GET", "/v2/activity/snapshots", query, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) ListSnapshotActivityWithPerf(ctx context.Context, params *ListSnapshotActivityParams, options ...RequestOption) (*LayerResponse[SnapshotActivityList], error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := SnapshotActivityList{}
	perf, err := client.request(ctx, "GET", "/v2/activity/snapshots", query, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[SnapshotActivityList]{Data: out, Perf: *perf}, nil
}


func (client *Client) ListSnapshotJobs(ctx context.Context, namespace string, options ...RequestOption) (*SnapshotJobList, error) {
	out := SnapshotJobList{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/snapshot-jobs", url.PathEscape(namespace)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) ListSnapshotJobsWithPerf(ctx context.Context, namespace string, options ...RequestOption) (*LayerResponse[SnapshotJobList], error) {
	out := SnapshotJobList{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/snapshot-jobs", url.PathEscape(namespace)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[SnapshotJobList]{Data: out, Perf: *perf}, nil
}


func (client *Client) ListTurbopufferNamespaces(ctx context.Context, params *ListTurbopufferNamespacesParams, options ...RequestOption) (*TurbopufferNamespaceList, error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := TurbopufferNamespaceList{}
	if _, err := client.request(ctx, "GET", "/v1/namespaces", query, nil, &out, &turbopufferFallback{method: "GET", path: "/v1/namespaces"}, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) ListTurbopufferNamespacesWithPerf(ctx context.Context, params *ListTurbopufferNamespacesParams, options ...RequestOption) (*LayerResponse[TurbopufferNamespaceList], error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := TurbopufferNamespaceList{}
	perf, err := client.request(ctx, "GET", "/v1/namespaces", query, nil, &out, &turbopufferFallback{method: "GET", path: "/v1/namespaces"}, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[TurbopufferNamespaceList]{Data: out, Perf: *perf}, nil
}


func (client *Client) ListUdfs(ctx context.Context, options ...RequestOption) (*UdfList, error) {
	out := UdfList{}
	if _, err := client.request(ctx, "GET", "/v2/udfs", url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) ListUdfsWithPerf(ctx context.Context, options ...RequestOption) (*LayerResponse[UdfList], error) {
	out := UdfList{}
	perf, err := client.request(ctx, "GET", "/v2/udfs", url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[UdfList]{Data: out, Perf: *perf}, nil
}


func (client *Client) ListWarmJobs(ctx context.Context, namespace string, options ...RequestOption) (*WarmJobList, error) {
	out := WarmJobList{}
	if _, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/warm-jobs", url.PathEscape(namespace)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) ListWarmJobsWithPerf(ctx context.Context, namespace string, options ...RequestOption) (*LayerResponse[WarmJobList], error) {
	out := WarmJobList{}
	perf, err := client.request(ctx, "GET", fmt.Sprintf("/v2/namespaces/%s/warm-jobs", url.PathEscape(namespace)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[WarmJobList]{Data: out, Perf: *perf}, nil
}


func (client *Client) MultiQueryTurbopufferNamespace(ctx context.Context, namespace string, body *TurbopufferMultiQueryRequest, options ...RequestOption) (*TurbopufferMultiQueryResponse, error) {
	out := TurbopufferMultiQueryResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s/query", url.PathEscape(namespace)), func() url.Values {
		query := url.Values{}
		query.Set("stainless_overload", "multiQuery")
		return query
	}(), body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v2/namespaces/%s/query", url.PathEscape(namespace))}, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) MultiQueryTurbopufferNamespaceWithPerf(ctx context.Context, namespace string, body *TurbopufferMultiQueryRequest, options ...RequestOption) (*LayerResponse[TurbopufferMultiQueryResponse], error) {
	out := TurbopufferMultiQueryResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s/query", url.PathEscape(namespace)), func() url.Values {
		query := url.Values{}
		query.Set("stainless_overload", "multiQuery")
		return query
	}(), body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v2/namespaces/%s/query", url.PathEscape(namespace))}, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[TurbopufferMultiQueryResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) PauseUdf(ctx context.Context, udfID string, options ...RequestOption) (*Udf, error) {
	out := Udf{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/udfs/%s/pause", url.PathEscape(udfID)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) PauseUdfWithPerf(ctx context.Context, udfID string, options ...RequestOption) (*LayerResponse[Udf], error) {
	out := Udf{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/udfs/%s/pause", url.PathEscape(udfID)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[Udf]{Data: out, Perf: *perf}, nil
}


func (client *Client) PutPipelineDocumentChunks(ctx context.Context, pipelineID string, docID string, body *PutChunksRequest, options ...RequestOption) (*StageDocumentResponse, error) {
	out := StageDocumentResponse{}
	if _, err := client.request(ctx, "PUT", fmt.Sprintf("/v2/pipelines/%s/documents/%s", url.PathEscape(pipelineID), url.PathEscape(docID)), url.Values{}, body, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) PutPipelineDocumentChunksWithPerf(ctx context.Context, pipelineID string, docID string, body *PutChunksRequest, options ...RequestOption) (*LayerResponse[StageDocumentResponse], error) {
	out := StageDocumentResponse{}
	perf, err := client.request(ctx, "PUT", fmt.Sprintf("/v2/pipelines/%s/documents/%s", url.PathEscape(pipelineID), url.PathEscape(docID)), url.Values{}, body, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[StageDocumentResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) PutPipelineDocumentVectors(ctx context.Context, pipelineID string, docID string, body *PutVectorsRequest, options ...RequestOption) (*StatusResponse, error) {
	out := StatusResponse{}
	if _, err := client.request(ctx, "PUT", fmt.Sprintf("/v2/pipelines/%s/documents/%s/vectors", url.PathEscape(pipelineID), url.PathEscape(docID)), url.Values{}, body, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) PutPipelineDocumentVectorsWithPerf(ctx context.Context, pipelineID string, docID string, body *PutVectorsRequest, options ...RequestOption) (*LayerResponse[StatusResponse], error) {
	out := StatusResponse{}
	perf, err := client.request(ctx, "PUT", fmt.Sprintf("/v2/pipelines/%s/documents/%s/vectors", url.PathEscape(pipelineID), url.PathEscape(docID)), url.Values{}, body, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[StatusResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) QueryMetrics(ctx context.Context, params *QueryMetricsParams, options ...RequestOption) (PrometheusResponse, error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := PrometheusResponse{}
	if _, err := client.request(ctx, "GET", "/v2/metrics/query", query, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return out, nil
}

func (client *Client) QueryMetricsWithPerf(ctx context.Context, params *QueryMetricsParams, options ...RequestOption) (*LayerResponse[PrometheusResponse], error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := PrometheusResponse{}
	perf, err := client.request(ctx, "GET", "/v2/metrics/query", query, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[PrometheusResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) QueryMetricsApiV1(ctx context.Context, params *QueryMetricsApiV1Params, options ...RequestOption) (PrometheusResponse, error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := PrometheusResponse{}
	if _, err := client.request(ctx, "GET", "/v2/metrics/api/v1/query", query, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return out, nil
}

func (client *Client) QueryMetricsApiV1WithPerf(ctx context.Context, params *QueryMetricsApiV1Params, options ...RequestOption) (*LayerResponse[PrometheusResponse], error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := PrometheusResponse{}
	perf, err := client.request(ctx, "GET", "/v2/metrics/api/v1/query", query, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[PrometheusResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) QueryMetricsRange(ctx context.Context, params *QueryMetricsRangeParams, options ...RequestOption) (PrometheusResponse, error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := PrometheusResponse{}
	if _, err := client.request(ctx, "GET", "/v2/metrics/query_range", query, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return out, nil
}

func (client *Client) QueryMetricsRangeWithPerf(ctx context.Context, params *QueryMetricsRangeParams, options ...RequestOption) (*LayerResponse[PrometheusResponse], error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := PrometheusResponse{}
	perf, err := client.request(ctx, "GET", "/v2/metrics/query_range", query, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[PrometheusResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) QueryMetricsRangeApiV1(ctx context.Context, params *QueryMetricsRangeApiV1Params, options ...RequestOption) (PrometheusResponse, error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := PrometheusResponse{}
	if _, err := client.request(ctx, "GET", "/v2/metrics/api/v1/query_range", query, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return out, nil
}

func (client *Client) QueryMetricsRangeApiV1WithPerf(ctx context.Context, params *QueryMetricsRangeApiV1Params, options ...RequestOption) (*LayerResponse[PrometheusResponse], error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := PrometheusResponse{}
	perf, err := client.request(ctx, "GET", "/v2/metrics/api/v1/query_range", query, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[PrometheusResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) QueryNamespace(ctx context.Context, namespace string, body *QueryRequest, options ...RequestOption) (*QueryResponse, error) {
	out := QueryResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s/query", url.PathEscape(namespace)), url.Values{}, body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v2/namespaces/%s/query", url.PathEscape(namespace)), transform: "query_namespace"}, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) QueryNamespaceWithPerf(ctx context.Context, namespace string, body *QueryRequest, options ...RequestOption) (*LayerResponse[QueryResponse], error) {
	out := QueryResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s/query", url.PathEscape(namespace)), url.Values{}, body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v2/namespaces/%s/query", url.PathEscape(namespace)), transform: "query_namespace"}, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[QueryResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) QueryTurbopufferNamespace(ctx context.Context, namespace string, body TurbopufferQueryRequest, options ...RequestOption) (*TurbopufferQueryResponse, error) {
	out := TurbopufferQueryResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s/query", url.PathEscape(namespace)), url.Values{}, body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v2/namespaces/%s/query", url.PathEscape(namespace))}, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) QueryTurbopufferNamespaceWithPerf(ctx context.Context, namespace string, body TurbopufferQueryRequest, options ...RequestOption) (*LayerResponse[TurbopufferQueryResponse], error) {
	out := TurbopufferQueryResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s/query", url.PathEscape(namespace)), url.Values{}, body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v2/namespaces/%s/query", url.PathEscape(namespace))}, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[TurbopufferQueryResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) ResetFailedUdf(ctx context.Context, udfID string, options ...RequestOption) (*UdfItemsResponse, error) {
	out := UdfItemsResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/udfs/%s/reset-failed", url.PathEscape(udfID)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) ResetFailedUdfWithPerf(ctx context.Context, udfID string, options ...RequestOption) (*LayerResponse[UdfItemsResponse], error) {
	out := UdfItemsResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/udfs/%s/reset-failed", url.PathEscape(udfID)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[UdfItemsResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) ResumeUdf(ctx context.Context, udfID string, options ...RequestOption) (*Udf, error) {
	out := Udf{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/udfs/%s/resume", url.PathEscape(udfID)), url.Values{}, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) ResumeUdfWithPerf(ctx context.Context, udfID string, options ...RequestOption) (*LayerResponse[Udf], error) {
	out := Udf{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/udfs/%s/resume", url.PathEscape(udfID)), url.Values{}, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[Udf]{Data: out, Perf: *perf}, nil
}


func (client *Client) SetDocumentsStage(ctx context.Context, pipelineID string, body *SetDocumentsStageRequest, options ...RequestOption) (*DocumentsStageResponse, error) {
	out := DocumentsStageResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/pipelines/%s/documents/stage", url.PathEscape(pipelineID)), url.Values{}, body, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) SetDocumentsStageWithPerf(ctx context.Context, pipelineID string, body *SetDocumentsStageRequest, options ...RequestOption) (*LayerResponse[DocumentsStageResponse], error) {
	out := DocumentsStageResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/pipelines/%s/documents/stage", url.PathEscape(pipelineID)), url.Values{}, body, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[DocumentsStageResponse]{Data: out, Perf: *perf}, nil
}


func (client *Client) UpdateTurbopufferNamespaceMetadata(ctx context.Context, namespace string, body *TurbopufferMetadataPatch, options ...RequestOption) (*NamespaceMetadata, error) {
	out := NamespaceMetadata{}
	if _, err := client.request(ctx, "PATCH", fmt.Sprintf("/v1/namespaces/%s/metadata", url.PathEscape(namespace)), url.Values{}, body, &out, &turbopufferFallback{method: "PATCH", path: fmt.Sprintf("/v1/namespaces/%s/metadata", url.PathEscape(namespace))}, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) UpdateTurbopufferNamespaceMetadataWithPerf(ctx context.Context, namespace string, body *TurbopufferMetadataPatch, options ...RequestOption) (*LayerResponse[NamespaceMetadata], error) {
	out := NamespaceMetadata{}
	perf, err := client.request(ctx, "PATCH", fmt.Sprintf("/v1/namespaces/%s/metadata", url.PathEscape(namespace)), url.Values{}, body, &out, &turbopufferFallback{method: "PATCH", path: fmt.Sprintf("/v1/namespaces/%s/metadata", url.PathEscape(namespace))}, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[NamespaceMetadata]{Data: out, Perf: *perf}, nil
}


func (client *Client) UpdateTurbopufferNamespaceSchema(ctx context.Context, namespace string, body TurbopufferSchema, options ...RequestOption) (TurbopufferSchema, error) {
	out := TurbopufferSchema{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/schema", url.PathEscape(namespace)), url.Values{}, body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v1/namespaces/%s/schema", url.PathEscape(namespace))}, options...); err != nil {
		return nil, err
	}
	return out, nil
}

func (client *Client) UpdateTurbopufferNamespaceSchemaWithPerf(ctx context.Context, namespace string, body TurbopufferSchema, options ...RequestOption) (*LayerResponse[TurbopufferSchema], error) {
	out := TurbopufferSchema{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v1/namespaces/%s/schema", url.PathEscape(namespace)), url.Values{}, body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v1/namespaces/%s/schema", url.PathEscape(namespace))}, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[TurbopufferSchema]{Data: out, Perf: *perf}, nil
}


func (client *Client) WarmCache(ctx context.Context, namespace string, params *WarmCacheParams, options ...RequestOption) (*WarmJob, error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := WarmJob{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s/warm", url.PathEscape(namespace)), query, nil, &out, nil, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) WarmCacheWithPerf(ctx context.Context, namespace string, params *WarmCacheParams, options ...RequestOption) (*LayerResponse[WarmJob], error) {
	query, err := params.query()
	if err != nil {
		return nil, err
	}
	out := WarmJob{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s/warm", url.PathEscape(namespace)), query, nil, &out, nil, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[WarmJob]{Data: out, Perf: *perf}, nil
}


func (client *Client) WriteNamespace(ctx context.Context, namespace string, body TurbopufferWriteRequest, options ...RequestOption) (*TurbopufferWriteResponse, error) {
	out := TurbopufferWriteResponse{}
	if _, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s", url.PathEscape(namespace)), url.Values{}, body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v2/namespaces/%s", url.PathEscape(namespace))}, options...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (client *Client) WriteNamespaceWithPerf(ctx context.Context, namespace string, body TurbopufferWriteRequest, options ...RequestOption) (*LayerResponse[TurbopufferWriteResponse], error) {
	out := TurbopufferWriteResponse{}
	perf, err := client.request(ctx, "POST", fmt.Sprintf("/v2/namespaces/%s", url.PathEscape(namespace)), url.Values{}, body, &out, &turbopufferFallback{method: "POST", path: fmt.Sprintf("/v2/namespaces/%s", url.PathEscape(namespace))}, options...)
	if err != nil {
		return nil, err
	}
	return &LayerResponse[TurbopufferWriteResponse]{Data: out, Perf: *perf}, nil
}


type DocumentStageOptions struct {
	FromStage string
	WorkerID string
}

type ScanWaitOptions struct {
	InitialDelay time.Duration
	MaxDelay time.Duration
	Timeout time.Duration
}

func (client *Client) EnsurePipeline(ctx context.Context, body *CreatePipelineRequest, options ...RequestOption) (*Pipeline, error) {
	pipeline, err := client.CreatePipeline(ctx, body, options...)
	if err == nil {
		return pipeline, nil
	}
	var hevlayerErr *HevlayerError
	if !errors.As(err, &hevlayerErr) || hevlayerErr.StatusCode != http.StatusConflict {
		return nil, err
	}
	if body == nil {
		return nil, err
	}
	pipelines, listErr := client.ListPipelines(ctx, options...)
	if listErr != nil {
		return nil, listErr
	}
	for _, existing := range pipelines.Pipelines {
		if existing.ID == body.ID {
			return &existing, nil
		}
	}
	return nil, err
}

func (client *Client) ReleaseDocuments(ctx context.Context, pipelineID string, documentIDs []string, stageOptions *DocumentStageOptions, options ...RequestOption) (*DocumentsStageResponse, error) {
	return client.setDocumentsStageHelper(ctx, pipelineID, documentIDs, "pending", stageOptions, options...)
}

func (client *Client) ReleaseDocumentsWithPerf(ctx context.Context, pipelineID string, documentIDs []string, stageOptions *DocumentStageOptions, options ...RequestOption) (*LayerResponse[DocumentsStageResponse], error) {
	return client.setDocumentsStageHelperWithPerf(ctx, pipelineID, documentIDs, "pending", stageOptions, options...)
}

func (client *Client) FailDocuments(ctx context.Context, pipelineID string, documentIDs []string, stageOptions *DocumentStageOptions, options ...RequestOption) (*DocumentsStageResponse, error) {
	return client.setDocumentsStageHelper(ctx, pipelineID, documentIDs, "failed", stageOptions, options...)
}

func (client *Client) FailDocumentsWithPerf(ctx context.Context, pipelineID string, documentIDs []string, stageOptions *DocumentStageOptions, options ...RequestOption) (*LayerResponse[DocumentsStageResponse], error) {
	return client.setDocumentsStageHelperWithPerf(ctx, pipelineID, documentIDs, "failed", stageOptions, options...)
}

func (client *Client) CompleteDocuments(ctx context.Context, pipelineID string, documentIDs []string, stageOptions *DocumentStageOptions, options ...RequestOption) (*DocumentsStageResponse, error) {
	return client.setDocumentsStageHelper(ctx, pipelineID, documentIDs, "indexed", stageOptions, options...)
}

func (client *Client) CompleteDocumentsWithPerf(ctx context.Context, pipelineID string, documentIDs []string, stageOptions *DocumentStageOptions, options ...RequestOption) (*LayerResponse[DocumentsStageResponse], error) {
	return client.setDocumentsStageHelperWithPerf(ctx, pipelineID, documentIDs, "indexed", stageOptions, options...)
}

func (client *Client) setDocumentsStageHelper(ctx context.Context, pipelineID string, documentIDs []string, stage string, stageOptions *DocumentStageOptions, options ...RequestOption) (*DocumentsStageResponse, error) {
	body := documentsStageRequest(documentIDs, stage, stageOptions)
	return client.SetDocumentsStage(ctx, pipelineID, body, options...)
}

func (client *Client) setDocumentsStageHelperWithPerf(ctx context.Context, pipelineID string, documentIDs []string, stage string, stageOptions *DocumentStageOptions, options ...RequestOption) (*LayerResponse[DocumentsStageResponse], error) {
	body := documentsStageRequest(documentIDs, stage, stageOptions)
	return client.SetDocumentsStageWithPerf(ctx, pipelineID, body, options...)
}

func documentsStageRequest(documentIDs []string, stage string, options *DocumentStageOptions) *SetDocumentsStageRequest {
	body := &SetDocumentsStageRequest{DocumentIds: documentIDs, Stage: stage}
	if options != nil {
		body.FromStage = options.FromStage
		body.WorkerID = options.WorkerID
	}
	return body
}

func (client *Client) WriteSingleVector(ctx context.Context, pipelineID string, docID string, vector VectorEntry, options ...RequestOption) (*StatusResponse, error) {
	return client.PutPipelineDocumentVectors(ctx, pipelineID, docID, &PutVectorsRequest{Vectors: []VectorEntry{vector}}, options...)
}

func (client *Client) WriteSingleVectorWithPerf(ctx context.Context, pipelineID string, docID string, vector VectorEntry, options ...RequestOption) (*LayerResponse[StatusResponse], error) {
	return client.PutPipelineDocumentVectorsWithPerf(ctx, pipelineID, docID, &PutVectorsRequest{Vectors: []VectorEntry{vector}}, options...)
}

func (client *Client) WaitForScan(ctx context.Context, namespace string, scanID string, options *ScanWaitOptions) (*ScanJob, error) {
	wait := normalizeScanWaitOptions(options)
	started := time.Now()
	delay := wait.InitialDelay
	for {
		scan, err := client.GetScan(ctx, namespace, scanID)
		if err != nil {
			return nil, err
		}
		if scan.Status == "completed" || scan.Status == "failed" {
			return scan, nil
		}
		if wait.Timeout > 0 && time.Since(started) >= wait.Timeout {
			return nil, fmt.Errorf("scan %q did not finish within %s", scanID, wait.Timeout)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
		delay *= 2
		if delay > wait.MaxDelay {
			delay = wait.MaxDelay
		}
	}
}

func (client *Client) Scan(ctx context.Context, namespace string, body *CreateScanRequest, options *ScanWaitOptions, requestOptions ...RequestOption) (*ScanJob, error) {
	created, err := client.CreateScan(ctx, namespace, body, requestOptions...)
	if err != nil {
		return nil, err
	}
	scanID, err := scanIDFromCreated(created)
	if err != nil {
		return nil, err
	}
	return client.WaitForScan(ctx, namespace, scanID, options)
}

func scanIDFromCreated(created interface{}) (string, error) {
	switch typed := created.(type) {
	case map[string]interface{}:
		if rawID, ok := typed["id"]; ok && rawID != nil {
			return fmt.Sprint(rawID), nil
		}
	case ScanJob:
		return typed.ID, nil
	case *ScanJob:
		if typed != nil {
			return typed.ID, nil
		}
	}
	encoded, err := json.Marshal(created)
	if err != nil {
		return "", err
	}
	var decoded struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return "", err
	}
	if decoded.ID == "" {
		return "", fmt.Errorf("scan create response did not include id")
	}
	return decoded.ID, nil
}

func normalizeScanWaitOptions(options *ScanWaitOptions) ScanWaitOptions {
	wait := ScanWaitOptions{
		InitialDelay: 50 * time.Millisecond,
		MaxDelay: 2 * time.Second,
	}
	if options == nil {
		return wait
	}
	if options.InitialDelay > 0 {
		wait.InitialDelay = options.InitialDelay
	}
	if options.MaxDelay > 0 {
		wait.MaxDelay = options.MaxDelay
	}
	if options.Timeout > 0 {
		wait.Timeout = options.Timeout
	}
	return wait
}

func (client *Client) WarmNamespace(ctx context.Context, namespace string, params *WarmCacheParams, options ...RequestOption) (*WarmJob, error) {
	return client.WarmCache(ctx, namespace, params, options...)
}

func (client *Client) WarmNamespaceWithPerf(ctx context.Context, namespace string, params *WarmCacheParams, options ...RequestOption) (*LayerResponse[WarmJob], error) {
	return client.WarmCacheWithPerf(ctx, namespace, params, options...)
}

func (client *Client) PatchColumns(ctx context.Context, namespace string, ids []string, attrs map[string][]interface{}, options ...RequestOption) (*TurbopufferWriteResponse, error) {
	request, err := patchColumnsRequest(ids, attrs)
	if err != nil {
		return nil, err
	}
	return client.WriteNamespace(ctx, namespace, request, options...)
}

func (client *Client) PatchColumnsWithPerf(ctx context.Context, namespace string, ids []string, attrs map[string][]interface{}, options ...RequestOption) (*LayerResponse[TurbopufferWriteResponse], error) {
	request, err := patchColumnsRequest(ids, attrs)
	if err != nil {
		return nil, err
	}
	return client.WriteNamespaceWithPerf(ctx, namespace, request, options...)
}

func patchColumnsRequest(ids []string, attrs map[string][]interface{}) (TurbopufferWriteRequest, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("patch_columns requires at least one id")
	}
	columns := map[string][]interface{}{"id": make([]interface{}, len(ids))}
	for index, id := range ids {
		columns["id"][index] = id
	}
	for name, values := range attrs {
		if name == "id" {
			return nil, fmt.Errorf("patch_columns attrs must not include id")
		}
		if len(values) != len(ids) {
			return nil, fmt.Errorf("patch_columns attr %q has %d values for %d ids", name, len(values), len(ids))
		}
		columns[name] = append([]interface{}(nil), values...)
	}
	return TurbopufferWriteRequest{"patch_columns": columns}, nil
}


func (client *Client) request(ctx context.Context, method string, requestPath string, query url.Values, body interface{}, out interface{}, fallback *turbopufferFallback, options ...RequestOption) (*LayerPerf, error) {
	requestOptions, err := buildRequestOptions(options)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	data, headers, err := client.doJSON(ctx, client.httpClient, client.baseURL, client.apiKey, method, requestPath, query, body, requestOptions.headers)
	if err != nil {
		if fallback == nil {
			return nil, err
		}
		return client.requestTurbopuffer(ctx, started, err, fallback, query, body, out)
	}

	perf := &LayerPerf{LatencyMS: float64(time.Since(started).Microseconds()) / 1000}
	perf.CacheStatus = headers.Get("x-layer-cache")
	if err := decodeResponseData(data, out); err != nil {
		return nil, err
	}
	applyLayerHeaders(out, headers)
	return perf, nil
}

func (client *Client) requestTurbopuffer(ctx context.Context, started time.Time, originalErr error, fallback *turbopufferFallback, query url.Values, body interface{}, out interface{}) (*LayerPerf, error) {
	if !client.canFallbackToTurbopuffer() {
		return nil, originalErr
	}
	fallbackBody, err := client.fallbackBody(fallback, body)
	if err != nil {
		return nil, originalErr
	}
	log.Printf("hevlayer gateway unreachable; falling through to Turbopuffer direct for %s %s", fallback.method, fallback.path)
	data, _, err := client.doJSON(ctx, client.turbopufferHTTPClient, client.turbopufferBaseURL, client.turbopufferAPIKey, fallback.method, fallback.path, query, fallbackBody, nil)
	if err != nil {
		return nil, err
	}
	if fallback.transform == "query_namespace" {
		data, err = queryResponseFromTurbopufferBytes(data)
		if err != nil {
			return nil, err
		}
	}
	perf := &LayerPerf{
		LatencyMS: float64(time.Since(started).Microseconds()) / 1000,
		Fallback: "turbopuffer_direct",
	}
	if err := decodeResponseData(data, out); err != nil {
		return nil, err
	}
	return perf, nil
}

func (client *Client) doJSON(ctx context.Context, httpClient *http.Client, baseURL string, apiKey string, method string, requestPath string, query url.Values, body interface{}, headers http.Header) ([]byte, http.Header, error) {
	endpoint := strings.TrimRight(baseURL, "/") + requestPath
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, resp.Header, decodeError(resp.StatusCode, data)
	}
	return data, resp.Header, nil
}

func decodeResponseData(data []byte, out interface{}) error {
	if out == nil || len(data) == 0 {
		return nil
	}
	if bytesOut, ok := out.(*[]byte); ok {
		*bytesOut = append((*bytesOut)[:0], data...)
		return nil
	}
	return json.Unmarshal(data, out)
}

func applyLayerHeaders(out interface{}, headers http.Header) {
	if out == nil || headers == nil {
		return
	}
	stableRaw := strings.TrimSpace(headers.Get("x-layer-stable-as-of"))
	nextCursor := headers.Get("x-layer-next-cursor")
	var stable int64
	if stableRaw != "" {
		_, _ = fmt.Sscan(stableRaw, &stable)
	}
	switch typed := out.(type) {
	case *QueryResponse:
		if stableRaw != "" {
			typed.StableAsOf = stable
		}
		if nextCursor != "" {
			typed.NextCursor = nextCursor
		}
	case *TurbopufferMultiQueryResponse:
		if stableRaw != "" {
			typed.StableAsOf = stable
		}
	}
}

func (client *Client) canFallbackToTurbopuffer() bool {
	return client.fallbackToTurbopuffer && (client.turbopufferAPIKey != "" || client.turbopufferHTTPClientExplicit)
}

func (client *Client) fallbackBody(fallback *turbopufferFallback, body interface{}) (interface{}, error) {
	if fallback.transform == "query_namespace" {
		return turbopufferQueryBody(body)
	}
	return body, nil
}

func turbopufferQueryBody(body interface{}) (map[string]interface{}, error) {
	var query QueryRequest
	switch typed := body.(type) {
	case QueryRequest:
		query = typed
	case *QueryRequest:
		if typed == nil {
			return nil, fmt.Errorf("query fallback requires an object body")
		}
		query = *typed
	case map[string]interface{}:
		return turbopufferQueryBodyFromMap(typed)
	default:
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		var decoded map[string]interface{}
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			return nil, err
		}
		return turbopufferQueryBodyFromMap(decoded)
	}
	if len(query.NearestToID) > 0 || query.Cursor != "" {
		return nil, fmt.Errorf("query fallback cannot resolve layer-only fields")
	}
	if len(query.Vector) == 0 {
		return nil, fmt.Errorf("query fallback requires vector")
	}
	bodyOut := map[string]interface{}{
		"rank_by": []interface{}{"vector", "ANN", query.Vector},
		"top_k": int64(10),
		"consistency": map[string]interface{}{"level": "eventual"},
	}
	if query.TopK != 0 {
		bodyOut["top_k"] = query.TopK
	}
	if query.Filters != nil {
		bodyOut["filters"] = query.Filters
	}
	if query.IncludeAttributes != nil {
		bodyOut["include_attributes"] = query.IncludeAttributes
	}
	return bodyOut, nil
}

func turbopufferQueryBodyFromMap(value map[string]interface{}) (map[string]interface{}, error) {
	if value["nearest_to_id"] != nil || value["cursor"] != nil {
		return nil, fmt.Errorf("query fallback cannot resolve layer-only fields")
	}
	vector, ok := value["vector"].([]interface{})
	if !ok || len(vector) == 0 {
		return nil, fmt.Errorf("query fallback requires vector")
	}
	body := map[string]interface{}{
		"rank_by": []interface{}{"vector", "ANN", vector},
		"top_k": int64(10),
		"consistency": map[string]interface{}{"level": "eventual"},
	}
	if value["top_k"] != nil {
		body["top_k"] = value["top_k"]
	}
	if value["filters"] != nil {
		body["filters"] = value["filters"]
	}
	if value["include_attributes"] != nil {
		body["include_attributes"] = value["include_attributes"]
	}
	return body, nil
}

func queryResponseFromTurbopufferBytes(data []byte) ([]byte, error) {
	if len(data) > 0 {
		var raw interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func buildRequestOptions(options []RequestOption) (requestOptions, error) {
	requestOptions := requestOptions{headers: http.Header{}}
	for _, option := range options {
		if option != nil {
			option(&requestOptions)
		}
		if requestOptions.err != nil {
			return requestOptions, requestOptions.err
		}
	}
	return requestOptions, nil
}

func decodeError(statusCode int, data []byte) error {
	var payload struct {
		Error string `json:"error"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(data, &payload)
	if payload.Message == "" {
		payload.Message = strings.TrimSpace(string(data))
	}
	return &HevlayerError{StatusCode: statusCode, Kind: payload.Error, Message: payload.Message, Body: data}
}

func addQueryValue(query url.Values, name string, value interface{}) error {
	switch typed := value.(type) {
	case string:
		if typed != "" {
			query.Set(name, typed)
		}
	case bool:
		if typed {
			query.Set(name, "true")
		}
	case int:
		if typed != 0 {
			query.Set(name, fmt.Sprint(typed))
		}
	case int64:
		if typed != 0 {
			query.Set(name, fmt.Sprint(typed))
		}
	case float64:
		if typed != 0 {
			query.Set(name, fmt.Sprint(typed))
		}
	case []string:
		if len(typed) > 0 {
			values := typed
			if name == "tag" {
				cleaned, err := cleanHistoryTags(typed)
				if err != nil {
					return err
				}
				values = cleaned
			}
			query.Set(name, strings.Join(values, ","))
		}
	}
	return nil
}

func cleanToken(value string) string {
	return strings.TrimSpace(value)
}

func cleanHistoryTags(tags []string) ([]string, error) {
	seen := map[string]bool{}
	cleaned := make([]string, 0, len(tags))
	for _, rawTag := range tags {
		tag := strings.TrimSpace(rawTag)
		if tag == "" {
			continue
		}
		if len([]byte(tag)) > searchHistoryMaxTagLength {
			return nil, fmt.Errorf("search-history tag %q exceeds %d bytes", tag, searchHistoryMaxTagLength)
		}
		if !validHistoryTag(tag) {
			return nil, fmt.Errorf("search-history tags may contain only ASCII letters, digits, ':', '_', '-', '.', '/', '=', or '+'; commas separate tags and cannot be escaped")
		}
		if !seen[tag] {
			seen[tag] = true
			cleaned = append(cleaned, tag)
		}
	}
	sort.Strings(cleaned)
	if len(cleaned) > searchHistoryMaxTags {
		return nil, fmt.Errorf("search-history tags are limited to %d unique tags", searchHistoryMaxTags)
	}
	return cleaned, nil
}

func validHistoryTag(tag string) bool {
	for _, r := range tag {
		if r > unicode.MaxASCII || !utf8.ValidRune(r) {
			return false
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(":_-.=/+", r) {
			continue
		}
		return false
	}
	return true
}
