package rest

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sort"
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

func (h *Handler) handleValidatorRating(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Read query parameters.
	q := r.URL.Query()
	startTimeStr, endTimeStr := q.Get("startTime"), q.Get("endTime")
	unlOnlyStr := q.Get("unlOnly")

	startTime, endTime, unlOnly, err := parseRatingOptions(ctx, startTimeStr, endTimeStr, unlOnlyStr)
	if err != nil {
		slog.ErrorContext(ctx, "invalid query parameters", "error", err)
		httputils.WriteError(w, httputils.BadRequest().WithReasonErr(err))
		return
	}

	options := store.ValidatorAgreementRateBucketOptions{
		// Use millenium so only one bucket is returned.
		Bucket:    store.BucketMillennium,
		StartTime: startTime,
		EndTime:   endTime,
		UNLOnly:   unlOnly,
	}

	// Database call to fetch agreement counts and ledger counts.
	data, err := h.database.GetValidatorAgreementRateBuckets(ctx, options)
	if err != nil {
		slog.ErrorContext(ctx, "error in GetValidatorAgreementRates call", "error", err)
		httputils.WriteError(w, httputils.InternalServerError().WithReasonErr(err))
		return
	}

	type scoreDetails struct {
		ValidatorDomain         string  `json:"validatorDomain"`
		ValidatorMasterKey      string  `json:"validatorMasterKey"`
		AgreementRate           float64 `json:"agreementRate"`
		AgreementRateWeightage  float64 `json:"agreementRateWeightage"`
		DomainVerified          bool    `json:"domainVerified"`
		DomainVerifiedWeightage float64 `json:"domainVerifiedWeightage"`
		Score                   float64 `json:"score"`
	}

	// Update score as per agreement rate.
	scores := make([]scoreDetails, len(data.AgreementCounts))
	for i, item := range data.AgreementCounts {
		// Defend against index-out-of-bounds.
		if len(item.Agreement) == 0 {
			continue
		}

		// For easier access.
		t, v := item.Agreement[0].Time, item.Agreement[0].Value
		// Get ledger count for this bucket.
		lc, exists := data.LedgerCounts[t]
		if !exists {
			continue
		}

		agreementRate := 100 * float64(v) / float64(lc)
		// Here 0.8 represents an 80% weightage in the final score.
		agreementRateWeightage := 0.8 * agreementRate
		var domainVerifiedWeightage float64
		if item.DomainVerified {
			domainVerifiedWeightage = 20.0
		}

		// Update score.
		scores[i] = scoreDetails{
			ValidatorDomain:         item.Domain,
			ValidatorMasterKey:      item.MasterKey,
			AgreementRate:           agreementRate,
			AgreementRateWeightage:  agreementRateWeightage,
			DomainVerified:          item.DomainVerified,
			DomainVerifiedWeightage: domainVerifiedWeightage,
			Score:                   agreementRateWeightage + domainVerifiedWeightage,
		}
	}

	sort.SliceStable(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	httputils.WriteJson(w, http.StatusOK, nil, scores)
}

func (h *Handler) handleValidatorAgmtHeatmap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Read query parameters.
	// Note that due to there being only ~150 validators, this is not a paginated API,
	// so limit and skip are not accepted.
	q := r.URL.Query()
	startTimeStr, endTimeStr, bucketStr := q.Get("startTime"), q.Get("endTime"), q.Get("bucket")
	unlOnly := q.Get("unlOnly")

	// Parse query parameters into concrete types.
	options, err := parseHeatmapOptions(ctx, startTimeStr, endTimeStr, bucketStr, unlOnly)
	if err != nil {
		slog.ErrorContext(ctx, "invalid query parameters", "error", err)
		httputils.WriteError(w, httputils.BadRequest().WithReasonErr(err))
		return
	}

	// Database call to fetch agreement counts and ledger counts.
	data, err := h.database.GetValidatorAgreementRateBuckets(ctx, options)
	if err != nil {
		slog.ErrorContext(ctx, "error in GetValidatorAgreementRates call", "error", err)
		httputils.WriteError(w, httputils.InternalServerError().WithReasonErr(err))
		return
	}

	// Transform to ECharts format.
	resp := transformAgreementRatesToHeatmap(data)
	resp.Pagination = pagination{
		// TODO: Confirm if this has any usage left.
		TotalCount: data.TotalValidatorCount,
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
	// Will hold all the validator domain/master-key.
	yAxis := make([]string, len(data.AgreementCounts))
	// Required to pre-allocate the chart data slice.
	var totalEntries int

	// Populate yAxis with validator domain/master-key.
	for i, item := range data.AgreementCounts {
		totalEntries += len(item.Agreement)

		if item.Domain != "" {
			yAxis[i] = item.Domain
		} else {
			yAxis[i] = item.MasterKey
		}
	}

	// Will hold all the time values in order.
	var xAxis []string
	for t := range data.LedgerCounts {
		xAxis = append(xAxis, t.Format("2006-01-02 15:04"))
	}
	slices.Sort(xAxis)

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
	chartData := make([][3]any, 0, totalEntries)
	for _, item := range data.AgreementCounts {
		// Index for the domain or the master-key.
		yIdx, exists := validatorIndex[item.Domain]
		if !exists {
			yIdx = validatorIndex[item.MasterKey]
		}

		for _, ag := range item.Agreement {
			// Calculate percent by diving by the total ledger count in that interval.
			var agreementPercent float64
			ledgerCount := data.LedgerCounts[ag.Time]
			if ledgerCount == 0 {
				agreementPercent = 0
			} else {
				agreementPercent = float64(ag.Value) * 100 / float64(ledgerCount)
			}

			// Index for the time.
			timeStr := ag.Time.Format("2006-01-02 15:04")
			xIdx := timeIndex[timeStr]
			chartData = append(chartData, [3]any{xIdx, yIdx, agreementPercent})
		}
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

// parseRatingOptions parses and validates all query paramters of the validator rating API.
// It also handles defaults.
func parseRatingOptions(
	ctx context.Context, startTimeStr, endTimeStr, unlOnlyStr string,
) (time.Time, time.Time, bool, error) {
	if unlOnlyStr == "" {
		unlOnlyStr = "false"
	}

	startTime, endTime, err := parseTimeRange(startTimeStr, endTimeStr)
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}

	unlOnly, err := strconv.ParseBool(unlOnlyStr)
	if err != nil {
		return time.Time{}, time.Time{}, false,
			fmt.Errorf("unlOnly flag is invalid: %w", err)
	}

	return startTime, endTime, unlOnly, nil
}

// parseHeatmapOptions parses and validates all query paramters of the heatmap API.
// It also handles defaults.
func parseHeatmapOptions(
	ctx context.Context, startTimeStr, endTimeStr, bucketStr, unlOnlyStr string,
) (store.ValidatorAgreementRateBucketOptions, error) {
	startTime, endTime, unlOnly, err := parseRatingOptions(ctx, startTimeStr, endTimeStr, unlOnlyStr)
	if err != nil {
		return store.ValidatorAgreementRateBucketOptions{}, err
	}

	// Default values for bucket.
	if bucketStr == "" {
		dur := endTime.Sub(startTime)
		if dur < time.Hour {
			bucketStr = string(store.BucketMinute)
		} else if dur < time.Hour*24 {
			bucketStr = string(store.BucketHour)
		} else if dur < time.Hour*24*7 {
			bucketStr = string(store.BucketDay)
		} else if dur < time.Hour*24*30 {
			bucketStr = string(store.BucketWeek)
		} else if dur < time.Hour*24*30*4 {
			bucketStr = string(store.BucketMonth)
		} else if dur < time.Hour*24*365 {
			bucketStr = string(store.BucketQuarter)
		} else {
			bucketStr = string(store.BucketYear)
		}

		slog.InfoContext(ctx, "bucket query param was not provided, calculated automatically",
			"calculatedValue", bucketStr)
	}

	if !slices.Contains(store.AllBuckets(), store.Bucket(bucketStr)) {
		return store.ValidatorAgreementRateBucketOptions{},
			fmt.Errorf("bucket is invalid, expected: %v", store.AllBuckets())
	}

	return store.ValidatorAgreementRateBucketOptions{
		StartTime: startTime,
		EndTime:   endTime,
		Bucket:    store.Bucket(bucketStr),
		UNLOnly:   unlOnly,
	}, nil
}
