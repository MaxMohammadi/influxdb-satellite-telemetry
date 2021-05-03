package gather

import (
	"context"
	"encoding/json"

	"github.com/influxdata/influxdb/v2/nats"
	"github.com/influxdata/influxdb/v2/storage"
	"go.uber.org/zap"
)

// PointWriter will use the storage.PointWriter interface to record metrics.
type PointWriter struct {
	Writer storage.PointsWriter
}

// Record the metrics and write using storage.PointWriter interface.
func (s PointWriter) Record(collected MetricsCollection) error {
	ps, err := collected.MetricsSlice.Points()
	if err != nil {
		return err
	}

	return s.Writer.WritePoints(context.Background(), collected.OrgID, collected.BucketID, ps)
}

// Recorder record the metrics of a time based.
type Recorder interface {
	// Subscriber nats.Subscriber
	Record(collected MetricsCollection) error
}

// RecorderHandler implements nats.Handler interface.
type RecorderHandler struct {
	Recorder Recorder
	log      *zap.Logger
}

func NewRecorderHandler(log *zap.Logger, recorder Recorder) *RecorderHandler {
	return &RecorderHandler{
		Recorder: recorder,
		log:      log,
	}
}

// Process consumes job queue, and use recorder to record.
func (h *RecorderHandler) Process(s nats.Subscription, m nats.Message) {
	defer m.Ack()
	collected := new(MetricsCollection)
	err := json.Unmarshal(m.Data(), &collected)
	if err != nil {
		h.log.Error("Recorder handler error", zap.Error(err))
		return
	}
	err = h.Recorder.Record(*collected)
	if err != nil {
		h.log.Error("Recorder handler error", zap.Error(err))
	}
}
