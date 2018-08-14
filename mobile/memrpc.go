package lndmobile

import (
	"context"
	"net"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/lightningnetwork/lnd/lnrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

var (
	lis = bufconn.Listen(100)
)

// Callback is an interface that is passed in by callers of the library, and
// specifies where the responses should be deliver.
type Callback interface {
	// OnResponse is called by the library when a response from the daemon
	// for the associated RPC call is received. The reponse is a serialized
	// protobuf for the expected response, and must be deserialized by the
	// caller.
	OnResponse([]byte)

	// OnError is called by the library if any error is encountered during
	// the execution of the RPC call, or if the response stream ends. No
	// responses will be received after this.
	OnError(error)
}

// SendStream is an interface that the caller of the library can use to send
// requests to the server during the execution of a bidirectional streaming RPC
// call, or stop the stream.
type SendStream interface {
	// Send sends the serialized protobuf request to the server.
	Send([]byte) error

	// Stop closes the bidirecrional connection.
	Stop() error
}

// sendStream is an internal struct that satisifies the SendStream interface.
// We use it to wrap customizable send and stop methods, that can be tuned to
// the specific RPC call in question.
type sendStream struct {
	send func([]byte) error
	stop func() error
}

// Send sends the serialized protobuf request to the server.
//
// Part of the SendStream interface.
func (s *sendStream) Send(req []byte) error {
	return s.send(req)
}

// Stop closes the bidirectional connection.
//
// Part of the SendStream interface.
func (r *sendStream) Stop() error {
	return r.stop()
}

// receiver is a struct used to hold a generic recv closure, that can be set to
// return messages from the desired stream of responses.
type receiver struct {
	// recv returns a message from the stream of responses.
	recv func() (proto.Message, error)
}

// sender is a struct used to hold a generic send closure, that can be set to
// send messages to the desired stream of requests.
type sender struct {
	// send sends the given message to the request stream.
	send func(proto.Message) error

	// close closes the request stream.
	close func() error
}

// getClient returns a client connection to the server listening on lis.
func getClient() (lnrpc.LightningClient, context.Context, func(), error) {
	conn, err := lis.Dial()
	if err != nil {
		return nil, nil, nil, err
	}

	clientConn, err := grpc.Dial("",
		grpc.WithDialer(func(target string,
			timeout time.Duration) (net.Conn, error) {
			return conn, nil
		}),
		grpc.WithInsecure(),
		grpc.WithBackoffMaxDelay(10*time.Second),
	)
	if err != nil {
		return nil, nil, nil, err
	}
	client := lnrpc.NewLightningClient(clientConn)
	ctx, cancel := context.WithCancel(context.Background())
	return client, ctx, cancel, nil
}

// onceHandler is a struct used to call the daemon's RPC interface on methods
// where only one request and one response is expected.
type onceHandler struct {
	// newProto returns an empty struct for the desired grpc request.
	newProto func() proto.Message

	// getSync calls the desired method on the given client in a
	// blocking matter.
	getSync func(context.Context, interface{},
		proto.Message) (proto.Message, error)
}

// start executes the RPC call specified by this onceHandler using the
// specified serialized msg request.
func (s *onceHandler) start(msg []byte, callback Callback) {
	// We must make a copy of the passed byte slice, as there is no
	// guarantee the contents won't be changed while the go routine is
	// executing.
	data := make([]byte, len(msg))
	copy(data[:], msg[:])

	go func() {
		// Get an empty proto of the desired type, and deserialize msg
		// as this proto type.
		req := s.newProto()
		err := proto.Unmarshal(data, req)
		if err != nil {
			callback.OnError(err)
			return
		}

		// Get the gRPC client.
		client, ctx, cancel, err := getClient()
		if err != nil {
			callback.OnError(err)
			return
		}
		defer cancel()

		// Now execute the RPC call.
		resp, err := s.getSync(ctx, client, req)
		if err != nil {
			callback.OnError(err)
			return
		}

		// We serialize the response before returning it to the caller.
		b, err := proto.Marshal(resp)
		if err != nil {
			callback.OnError(err)
			return
		}

		callback.OnResponse(b)
	}()
}

// readStreamHandler is a struct used to call the daemon's RPC interface on
// methods where a stream of responses is expected, as in subscription type
// requests.
type readStreamHandler struct {
	// newProto returns an empty struct for the desired grpc request.
	newProto func() proto.Message

	// recvStream calls the given client with the request and returns a
	// receiver that reads the stream of responses.
	recvStream func(context.Context, lnrpc.LightningClient,
		proto.Message) (*receiver, error)
}

// start executes the RPC call specified by this readStreamHandler using the
// specified serialized msg request.
func (s *readStreamHandler) start(msg []byte, callback Callback) {
	// We must make a copy of the passed byte slice, as there is no
	// guarantee the contents won't be changed while the go routine is
	// executing.
	data := make([]byte, len(msg))
	copy(data[:], msg[:])

	go func() {
		// Get a new proto of the desired type and deserialize the
		// passed msg as this type.
		req := s.newProto()
		err := proto.Unmarshal(data, req)
		if err != nil {
			callback.OnError(err)
			return
		}

		// Get the client.
		client, ctx, cancel, err := getClient()
		if err != nil {
			callback.OnError(err)
			return
		}
		defer cancel()

		// Call the desired method on the client using the decoded gRPC
		// request, and get the receive stream back.
		stream, err := s.recvStream(ctx, client, req)
		if err != nil {
			callback.OnError(err)
			return
		}

		// We will read responses from the stream until we encounter an
		// error.
		for {
			// Read a response from the stream.
			resp, err := stream.recv()
			if err != nil {
				callback.OnError(err)
				return
			}

			// Serielize the response before returning it to the
			// caller.
			b, err := proto.Marshal(resp)
			if err != nil {
				callback.OnError(err)
				return
			}
			callback.OnResponse(b)
		}
	}()

}

// biStreamHandler is a struct used to call the daemon's RPC interface on
// methods where a bidirectional stream of request and responses is expected.
type biStreamHandler struct {
	// newProto returns an empty struct for the desired grpc request.
	newProto func() proto.Message

	// biStream calls the desired method on the given client and returns a
	// receiver that reads the stream of responses, and a sender that can
	// be used to send a stream of requests.
	biStream func(context.Context, lnrpc.LightningClient) (*receiver,
		*sender, error)
}

// start executes the RPC call specified by this biStreamHandler, sending
// messages coming from the returned SendStream.
func (b *biStreamHandler) start(callback Callback) (SendStream, error) {
	// Get the client connection.
	client, ctx, cancel, err := getClient()
	if err != nil {
		return nil, err
	}

	// Start a bidirectional stream for the desired RPC method.
	r, s, err := b.biStream(ctx, client)
	if err != nil {
		cancel()
		return nil, err
	}

	// We create a sendStream which is a wrapper for the methods we
	// will expose to the caller via the SendStream interface.
	ss := &sendStream{
		send: func(msg []byte) error {
			// Get an empty proto and deserialize the message
			// coming from the caller.
			req := b.newProto()
			err := proto.Unmarshal(msg, req)
			if err != nil {
				return err
			}

			// Send the request to the server.
			return s.send(req)
		},
		stop: s.close,
	}

	// Now launch a goroutine that will handle the asynchronous stream of
	// responses.
	go func() {
		defer cancel()

		// We will read responses from the recv stream until we
		// encounter an error.
		for {
			// Wait for a new response from the server.
			resp, err := r.recv()
			if err != nil {
				callback.OnError(err)
				return
			}

			// Serialize the response before returning it to the
			// caller.
			b, err := proto.Marshal(resp)
			if err != nil {
				callback.OnError(err)
				return
			}
			callback.OnResponse(b)
		}
	}()

	// Return the send stream to the caller, which then can be used to pass
	// messages to the server.
	return ss, nil
}
