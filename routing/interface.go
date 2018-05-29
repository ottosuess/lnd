package routing

import (
	"time"

	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/roasbeef/btcd/btcec"
)

// MissionControl...
type MissionControl interface {
	// CommitDecay...
	CommitDecay(t time.Time, blockHeight uint32) error

	// NewPaymentSession...
	NewPaymentSession(additionalEdges map[Vertex][]addEdge,
		amt lnwire.MilliSatoshi, target *btcec.PublicKey) (
		PaymentSession, error)
}

// PaymentSession is an interface type returned from the MissionControl,
// exposing all the methods used for carrying out a payment in the dynamically
// changing graph environment.
type PaymentSession interface {
	// ReportRouteSuccess...
	ReportRouteSuccess(payAmt lnwire.MilliSatoshi, route Route) error

	// ReportEdgeFailure...
	ReportChannelFailure(payAmt lnwire.MilliSatoshi, failedEdge *ChannelHop) error

	// ReportVertexFailure...
	ReportVertexFailure(payAmt lnwire.MilliSatoshi, failedNode Vertex) error

	// EdgeScore...
	EdgeScore(payAmt lnwire.MilliSatoshi,
		e *channeldb.ChannelEdgePolicy) (float64, error)

	// ForeEachNode...
	ForEachNode(func(*channeldb.LightningNode) error) error

	// ForEachChannel...
	// TODO: remove bandwidth.
	ForEachChannel(*btcec.PublicKey,
		func(outEdge *channeldb.ChannelEdgePolicy,
			bandwidth lnwire.MilliSatoshi) error) error
}
