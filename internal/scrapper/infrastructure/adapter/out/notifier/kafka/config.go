package kafka

import (
	"github.com/IBM/sarama"
)

func newConfig(opts ...func(*sarama.Config)) *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V4_0_0_0

	cfg.Producer.Partitioner = sarama.NewHashPartitioner
	cfg.Producer.Return.Successes = true
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Compression = sarama.CompressionGZIP

	for _, o := range opts {
		o(cfg)
	}

	return cfg
}
