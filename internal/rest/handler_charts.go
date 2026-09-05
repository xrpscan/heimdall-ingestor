package rest

import (
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/xrpscan/heimdall-ingestor/internal/store"
	"github.com/xrpscan/heimdall-ingestor/pkg/httputils"
)

// heatmapResponse is the response schema for the heatmap API.
type heatmapResponse struct {
	Data       [][3]any   `json:"data"`
	XAxis      []string   `json:"xAxis"`
	YAxis      []string   `json:"yAxis"`
	Pagination pagination `json:"pagination"`
}

// pagination represents generic pagination details for any API response.
type pagination struct {
	Limit      uint `json:"limit"`
	Offset     uint `json:"offset"`
	TotalCount uint `json:"totalCount"`
}

// ledgerCloseIntervalResponse is the response schema for the ledger close interval API.
type ledgerCloseIntervalResponse struct {
	Times         []string  `json:"times"`
	Intervals     []float64 `json:"intervals"`
	LedgerIndices []string  `json:"ledgerIndices"`
}

func (h *Handler) handleValidatorAgmtHeatmap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Read query parameters.
	q := r.URL.Query()
	limitStr, offsetStr := q.Get("limit"), q.Get("offset")
	startTimeStr, endTimeStr := q.Get("startTime"), q.Get("endTime")

	// Parse query parameters into concrete types.
	options, err := parseHeatmapOptions(limitStr, offsetStr, startTimeStr, endTimeStr)
	if err != nil {
		slog.ErrorContext(ctx, "invalid query parameters", "error", err)
		httputils.WriteError(w, httputils.BadRequest().WithReasonErr(err))
		return
	}

	data, err := h.database.GetValidatorAgreementRates(ctx, options)
	if err != nil {
		slog.ErrorContext(ctx, "error in GetValidatorAgreementRates call", "error", err)
		httputils.WriteError(w, httputils.InternalServerError().WithReasonErr(err))
		return
	}

	// Transform to ECharts format.
	resp := transformAgreementRatesToHeatmap(data)
	resp.Pagination = pagination{
		TotalCount: data.TotalValidatorCount,
		Limit:      options.Limit,
		Offset:     options.Offset,
	}

	httputils.WriteJson(w, http.StatusOK, nil, resp)
}

func (h *Handler) handleLedgerCloseInterval(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Read query parameters.
	q := r.URL.Query()
	startTimeStr, endTimeStr := q.Get("startTime"), q.Get("endTime")

	// Parse query parameters.
	startTime, endTime, err := parseTimeRange(startTimeStr, endTimeStr)
	if err != nil {
		slog.ErrorContext(ctx, "invalid query parameters", "error", err)
		httputils.WriteError(w, httputils.BadRequest().WithReasonErr(err))
		return
	}

	data, err := h.database.GetLedgerCloseIntervals(ctx, startTime, endTime)
	if err != nil {
		slog.ErrorContext(ctx, "error in GetLedgerCloseIntervals call", "error", err)
		httputils.WriteError(w, httputils.InternalServerError().WithReasonErr(err))
		return
	}

	// Transform to ECharts format.
	resp := transformLedgerIntervalsToChart(data)
	httputils.WriteJson(w, http.StatusOK, nil, resp)
}

// transformLedgerIntervalsToChart transforms the database results into a format suitable for
// ECharts line chart.
func transformLedgerIntervalsToChart(data []store.LedgerCloseInterval) ledgerCloseIntervalResponse {
	times := make([]string, len(data))
	intervals := make([]float64, len(data))
	indices := make([]string, len(data))

	for i, row := range data {
		times[i] = row.Time.Format("2006-01-02 15:04:05")
		intervals[i] = row.IntervalSeconds
		indices[i] = strconv.FormatInt(row.LedgerIndex, 10)
	}

	return ledgerCloseIntervalResponse{
		Times:         times,
		Intervals:     intervals,
		LedgerIndices: indices,
	}
}

// transformAgreementRatesToHeatmap transforms the list agreement rates coming from the database
// into a heatmap format that Apache ECharts can render.
func transformAgreementRatesToHeatmap(data store.AgreementRatesData) heatmapResponse {
	// Sets for extract unique times and validators.
	timeSet, validatorSet := map[string]struct{}{}, map[string]struct{}{}

	// Populate sets.
	for _, row := range data.Rates {
		timeStr := row.Time.Format("2006-01-02 15:04")
		timeSet[timeStr] = struct{}{}
		validatorSet[row.Validator] = struct{}{}
	}

	// Convert sets to arrays since Echarts expects xAxis and yAxis arrays.
	var xAxis []string
	for t := range timeSet {
		xAxis = append(xAxis, t)
	}
	slices.Sort(xAxis) // Works because we're using lexicographically sortable time format.

	var yAxis []string
	for v := range validatorSet {
		yAxis = append(yAxis, v)
	}

	// Build reverse index maps for O(1) lookups. These are required because the "data" field in
	// the response requires index values of the xAxis and yAxis arrays.
	timeIndex := make(map[string]int)
	for i, t := range xAxis {
		timeIndex[t] = i
	}

	validatorIndex := make(map[string]int)
	for i, v := range yAxis {
		validatorIndex[v] = i
	}

	// Transform each row to [xIndex, yIndex, value]
	chartData := make([][3]any, 0, len(data.Rates))
	for _, rate := range data.Rates {
		timeStr := rate.Time.Format("2006-01-02 15:04")

		xIdx := timeIndex[timeStr]
		yIdx := validatorIndex[rate.Validator]
		chartData = append(chartData, [3]any{xIdx, yIdx, rate.AgreementPercent})
	}

	return heatmapResponse{Data: chartData, XAxis: xAxis, YAxis: yAxis}
}

// parseTimeRange parses startTime and endTime query parameters.
// Defaults to last 24 hours if not provided.
func parseTimeRange(startTimeStr, endTimeStr string) (time.Time, time.Time, error) {
	// Default endTime is now, default startTime is 24 hours ago.
	now := time.Now()
	if endTimeStr == "" {
		endTimeStr = strconv.FormatInt(now.UnixMilli(), 10)
	}
	if startTimeStr == "" {
		startTimeStr = strconv.FormatInt(now.Add(-24*time.Hour).UnixMilli(), 10)
	}

	parsedStartTime, err := strconv.ParseInt(startTimeStr, 10, 64)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("startTime is invalid: %w", err)
	}

	parsedEndTime, err := strconv.ParseInt(endTimeStr, 10, 64)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("endTime is invalid: %w", err)
	}

	if parsedStartTime >= parsedEndTime {
		return time.Time{}, time.Time{}, fmt.Errorf("startTime should be before endTime")
	}

	return time.UnixMilli(parsedStartTime), time.UnixMilli(parsedEndTime), nil
}

// parseHeatmapOptions parses and validates all query paramters of the heatmap API.
// It also handles defaults.
func parseHeatmapOptions(
	limitStr, offsetStr, startTimeStr, endTimeStr string,
) (store.ValidatorAgreementHeatmapOptions, error) {
	// Default limit and offset.
	if limitStr == "" {
		limitStr = "10"
	}
	if offsetStr == "" {
		offsetStr = "0"
	}

	parsedLimit, err := strconv.ParseUint(limitStr, 10, 64)
	if err != nil {
		return store.ValidatorAgreementHeatmapOptions{}, fmt.Errorf("limit is invalid: %w", err)
	}

	parsedOffset, err := strconv.ParseUint(offsetStr, 10, 64)
	if err != nil {
		return store.ValidatorAgreementHeatmapOptions{}, fmt.Errorf("offset is invalid: %w", err)
	}

	startTime, endTime, err := parseTimeRange(startTimeStr, endTimeStr)
	if err != nil {
		return store.ValidatorAgreementHeatmapOptions{}, err
	}

	return store.ValidatorAgreementHeatmapOptions{
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     uint(parsedLimit),
		Offset:    uint(parsedOffset),
	}, nil
}
