package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Blockchain metrics
var (
	BlockHeight = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "linkm",
		Subsystem: "chain",
		Name:      "block_height",
		Help:      "Current blockchain height.",
	})

	BlockProductionLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "linkm",
		Subsystem: "chain",
		Name:      "block_production_seconds",
		Help:      "Time to produce a block in seconds.",
		Buckets:   prometheus.DefBuckets,
	})

	BlockTxCount = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "linkm",
		Subsystem: "chain",
		Name:      "block_tx_count",
		Help:      "Number of transactions per block.",
		Buckets:   []float64{0, 1, 5, 10, 25, 50, 100, 250, 500},
	})
)

// Transaction metrics
var (
	TxPoolSize = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "linkm",
		Subsystem: "txpool",
		Name:      "size",
		Help:      "Current number of transactions in the mempool.",
	})

	TxProcessedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "linkm",
		Subsystem: "tx",
		Name:      "processed_total",
		Help:      "Total transactions processed by result.",
	}, []string{"status"}) // "success" or "failed"

	TxThroughput = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "linkm",
		Subsystem: "tx",
		Name:      "throughput_total",
		Help:      "Total successful transactions processed.",
	})
)

// P2P metrics
var (
	PeerCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "linkm",
		Subsystem: "p2p",
		Name:      "peer_count",
		Help:      "Number of connected P2P peers.",
	})

	GossipMessagesReceived = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "linkm",
		Subsystem: "p2p",
		Name:      "gossip_messages_received_total",
		Help:      "Total gossip messages received by topic.",
	}, []string{"topic"}) // "blocks" or "txs"

	GossipMessagesSent = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "linkm",
		Subsystem: "p2p",
		Name:      "gossip_messages_sent_total",
		Help:      "Total gossip messages sent by topic.",
	}, []string{"topic"})
)

// Consensus metrics
var (
	VRFEvalLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "linkm",
		Subsystem: "consensus",
		Name:      "vrf_eval_seconds",
		Help:      "VRF evaluation latency in seconds.",
		Buckets:   prometheus.DefBuckets,
	})

	CommitteeSize = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "linkm",
		Subsystem: "consensus",
		Name:      "committee_size",
		Help:      "Current committee size.",
	})

	ValidatorCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "linkm",
		Subsystem: "consensus",
		Name:      "active_validators",
		Help:      "Number of active validators.",
	})
)

// Fee metrics
var (
	FeesCollectedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "linkm",
		Subsystem: "fee",
		Name:      "collected_total",
		Help:      "Total fees collected (in base units).",
	})

	TokensBurnedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "linkm",
		Subsystem: "fee",
		Name:      "burned_total",
		Help:      "Total tokens burned (in base units).",
	})

	TreasuryBalance = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "linkm",
		Subsystem: "fee",
		Name:      "treasury_balance",
		Help:      "Current treasury balance.",
	})
)
