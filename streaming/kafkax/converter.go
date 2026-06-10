package kafkax

import "github.com/twmb/franz-go/pkg/kgo"

type RecordConverter interface {
	ToKafkaRecord() (*kgo.Record, error)
}
