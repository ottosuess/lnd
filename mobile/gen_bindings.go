// +build ignore

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"text/template"

	"strings"

	"github.com/golang/glog"
	"github.com/golang/protobuf/protoc-gen-go/generator"
	"github.com/grpc-ecosystem/grpc-gateway/codegenerator"
	"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway/descriptor"
)

var (
	importPrefix         = flag.String("import_prefix", "", "prefix to be added to go package paths for imported proto files")
	importPath           = flag.String("import_path", "", "used as the package if no input files declare go_package. If it contains slashes, everything up to the rightmost slash is ignored.")
	useRequestContext    = flag.Bool("request_context", true, "determine whether to use http.Request's context or not")
	allowDeleteBody      = flag.Bool("allow_delete_body", false, "unless set, HTTP DELETE methods may not have a body")
	grpcAPIConfiguration = flag.String("grpc_api_configuration", "", "path to gRPC API Configuration in YAML format")
)

func main() {
	flag.Parse()
	defer glog.Flush()

	reg := descriptor.NewRegistry()

	req, err := codegenerator.ParseRequest(os.Stdin)

	if err != nil {
		fmt.Println(err)
		return
	}

	f, err := os.Create("./api_generated.go")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	wr := bufio.NewWriter(f)
	defer wr.Flush()

	reg.SetPrefix(*importPrefix)
	reg.SetImportPath(*importPath)
	reg.SetAllowDeleteBody(*allowDeleteBody)
	if err := reg.Load(req); err != nil {
		fmt.Println("err loading: ", err)
		return
	}

	// Extract the RPC call godoc from the proto.
	godoc := make(map[string]string)
	for _, f := range req.GetProtoFile() {
		fd := &generator.FileDescriptor{
			FileDescriptorProto: f,
		}
		for _, loc := range fd.GetSourceCodeInfo().GetLocation() {
			if loc.LeadingComments == nil {
				continue
			}
			c := *loc.LeadingComments

			// Find the first newline. The actual comment will
			// start following this.
			i := 0
			for j := range c {
				if c[j] == '\n' {
					i = j
					break
				}
			}
			c = c[i+1:]

			// Find the first space. The method's name will
			// be all characters up to that space.
			i = 0
			for j := range c {
				if c[j] == ' ' {
					i = j
					break
				}
			}
			method := c[:i]

			// Insert comment // instead of every newline.
			c = strings.Replace(c, "\n", "\n// ", -1)

			// Add a leading comment // and remove the traling
			// one.
			if len(c) < 4 {
				continue
			}
			c = "// " + c[:len(c)-4]

			godoc[method] = c
		}
	}

	var targets []*descriptor.File
	for _, target := range req.FileToGenerate {
		f, err := reg.LookupFile(target)
		if err != nil {
			glog.Fatal(err)
		}

		targets = append(targets, f)
	}

	params := struct {
		Name    string
		Package string
	}{
		"rpc.proto",
		"lndmobile",
	}
	//w := bytes.NewBuffer(nil)
	if err := headerTemplate.Execute(wr, params); err != nil {
		fmt.Println(err)
		return
	}

	var services []*descriptor.Service

	for _, target := range targets {
		for _, s := range target.Services {
			services = append(services, s)
		}
	}

	for _, s := range services {
		if s.GetName() != "Lightning" {
			continue
		}
		for _, m := range s.Methods {
			name := m.GetName()
			clientStream := false
			serverStream := false
			if m.ClientStreaming != nil {
				clientStream = *m.ClientStreaming
			}

			if m.ServerStreaming != nil {
				serverStream = *m.ServerStreaming
			}

			switch {
			case !clientStream && !serverStream:
				p := rpcParams{
					MethodName:  m.GetName(),
					RequestType: m.GetInputType()[1:],
					Comment:     godoc[name],
				}

				if err := onceTemplate.Execute(wr, p); err != nil {
					fmt.Println(err)
					return
				}
			case !clientStream && serverStream:
				p := rpcParams{
					MethodName:  m.GetName(),
					RequestType: m.GetInputType()[1:],
					Comment:     godoc[name],
				}

				if err := readStreamTemplate.Execute(wr, p); err != nil {
					fmt.Println(err)
					return
				}
			case clientStream && serverStream:
				p := rpcParams{
					MethodName:  m.GetName(),
					RequestType: m.GetInputType()[1:],
					Comment:     godoc[name],
				}

				if err := biStreamTemplate.Execute(wr, p); err != nil {
					fmt.Println(err)
					return
				}
			}
		}
	}
}

type rpcParams struct {
	MethodName  string
	RequestType string
	Comment     string
}

var (
	headerTemplate = template.Must(template.New("header").Parse(`
// Code generated by protoc-gen-grpc-gateway. DO NOT EDIT.
// source: {{.Name}}
package {{.Package}}

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/lightningnetwork/lnd/lnrpc"
)
`))

	onceTemplate = template.Must(template.New("once").Parse(`
{{.Comment}}
//
// NOTE: This method produces a single result or error, and the callback
// will be called only once.
func {{.MethodName}}(msg []byte, callback Callback) {
	s := &onceHandler{
		newProto: func() proto.Message {
			return &{{.RequestType}}{}
		},
		getSync: func(ctx context.Context, client lnrpc.LightningClient,
			req proto.Message) (proto.Message, error) {
			r := req.(*{{.RequestType}})
			return client.{{.MethodName}}(ctx, r)
		},
	}
	s.start(msg, callback)
}
`))

	readStreamTemplate = template.Must(template.New("once").Parse(`
{{.Comment}}
//
// NOTE: This method produces a stream of responses, and the callback
// can be called zero or more times. After EOF error is returned, no
// more responses will be produced.
func {{.MethodName}}(msg []byte, callback Callback) {
	s := &readStreamHandler{
		newProto: func() proto.Message {
			return &{{.RequestType}}{}
		},
		recvStream: func(ctx context.Context,
			client lnrpc.LightningClient,
			req proto.Message) (*receiver, error) {
			r := req.(*{{.RequestType}})
			stream, err := client.{{.MethodName}}(ctx, r)
			if err != nil {
				return nil, err
			}
			return &receiver{
				recv: func() (proto.Message, error) {
					return stream.Recv()
				},
			}, nil
		},
	}
	s.start(msg, callback)
}
`))

	biStreamTemplate = template.Must(template.New("once").Parse(`
{{.Comment}}
//
// NOTE: This method produces a stream of responses, and the callback
// can be called zero or more times. After EOF error is returned, no
// more responses will be produced. The send stream can accept zero
// or more requests before it is closed.
func {{.MethodName}}(callback Callback) (SendStream, error) {
	b := &biStreamHandler{
		newProto: func() proto.Message {
			return &{{.RequestType}}{}
		},
		biStream: func(ctx context.Context,
			client lnrpc.LightningClient) (*receiver, *sender,
			error) {
			stream, err := client.{{.MethodName}}(ctx)
			if err != nil {
				return nil, nil, err
			}
			return &receiver{
					recv: func() (proto.Message, error) {
						return stream.Recv()
					},
				},
				&sender{
					send: func(req proto.Message) error {
						r := req.(*{{.RequestType}})
						return stream.Send(r)
					},
					close: stream.CloseSend,
				}, nil
		},
	}
	return b.start(callback)
}
`))
)
