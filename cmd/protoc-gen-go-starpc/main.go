// This code is a modified version of protoc-gen-drpc.
// Licensed under the MIT/expat license from Storj Labs, Inc.
// See LICENSE.drpc for informationj.

package main

import (
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

const SRPCPackage = "github.com/aperturerobotics/starpc/srpc"

func main() {
	opts := protogen.Options{}
	opts.Run(func(plugin *protogen.Plugin) error {
		for _, f := range plugin.Files {
			if !f.Generate || len(f.Services) == 0 {
				continue
			}
			generatePluginFile(plugin, f)
		}
		plugin.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		return nil
	})
}

func generatePluginFile(plugin *protogen.Plugin, file *protogen.File) {
	gf := plugin.NewGeneratedFile(file.GeneratedFilenamePrefix+"_srpc.pb.go", file.GoImportPath)
	s := &srpc{gf, file}

	s.P("// Code generated by protoc-gen-srpc. DO NOT EDIT.")
	if bi, ok := debug.ReadBuildInfo(); ok {
		s.P("// protoc-gen-srpc version: ", bi.Main.Version)
	}
	s.P("// source: ", file.Desc.Path())
	s.P()
	s.P("package ", file.GoPackageName)
	s.P()

	for _, service := range file.Services {
		s.generateService(service)
	}
}

type srpc struct {
	*protogen.GeneratedFile
	file *protogen.File
}

func (s *srpc) Ident(path, ident string) string {
	return s.QualifiedGoIdent(protogen.GoImportPath(path).Ident(ident))
}

// GetServiceID returns the service id for the srpc.
func (s *srpc) GetServiceID(p *protogen.Service) (service string) {
	return string(p.Desc.FullName())
}

// GetServiceAndMethodID returns the service and method for the srpc.
func (s *srpc) GetServiceAndMethodID(p *protogen.Method) (service, method string) {
	return string(p.Parent.Desc.FullName()), string(p.Desc.Name())
}

/*
func (s *srpc) ServiceMethodString(method *protogen.Method) string {
	return strconv.Quote(fmt.Sprintf("/%s/%s", method.Parent.Desc.FullName(), method.Desc.Name()))
}
*/

func (s *srpc) InputType(method *protogen.Method) string {
	return s.QualifiedGoIdent(method.Input.GoIdent)
}

func (s *srpc) OutputType(method *protogen.Method) string {
	return s.QualifiedGoIdent(method.Output.GoIdent)
}

func (s *srpc) ClientIface(service *protogen.Service) string {
	return "SRPC" + service.GoName + "Client"
}

func (s *srpc) ClientImpl(service *protogen.Service) string {
	return "srpc" + service.GoName + "Client"
}

func (s *srpc) ServerIface(service *protogen.Service) string {
	return "SRPC" + service.GoName + "Server"
}

func (s *srpc) ServerServiceID(service *protogen.Service) string {
	return "SRPC" + service.GoName + "ServiceID"
}

func (s *srpc) ServerImpl(service *protogen.Service) string {
	return "srpc" + service.GoName + "Server"
}

func (s *srpc) ServerUnimpl(service *protogen.Service) string {
	return "SRPC" + service.GoName + "UnimplementedServer"
}

func (s *srpc) ServerHandler(service *protogen.Service) string {
	return "SRPC" + service.GoName + "Handler"
}

func (s *srpc) ClientStreamIface(method *protogen.Method) string {
	return "SRPC" +
		strings.ReplaceAll(method.Parent.GoName, "_", "__") + "_" +
		strings.ReplaceAll(method.GoName, "_", "__") +
		"Client"
}

func (s *srpc) ClientStreamImpl(method *protogen.Method) string {
	return "srpc" +
		strings.ReplaceAll(method.Parent.GoName, "_", "__") + "_" +
		strings.ReplaceAll(method.GoName, "_", "__") +
		"Client"
}

func (s *srpc) ServerStreamIface(method *protogen.Method) string {
	return "SRPC" +
		strings.ReplaceAll(method.Parent.GoName, "_", "__") + "_" +
		strings.ReplaceAll(method.GoName, "_", "__") +
		"Stream"
}

func (s *srpc) ServerStreamImpl(method *protogen.Method) string {
	return "srpc" +
		strings.ReplaceAll(method.Parent.GoName, "_", "__") + "_" +
		strings.ReplaceAll(method.GoName, "_", "__") +
		"Stream"
}

//
// service generation
//
func (s *srpc) generateService(service *protogen.Service) {
	// Client interface
	s.P("type ", s.ClientIface(service), " interface {")
	s.P("SRPCClient() ", s.Ident(SRPCPackage, "Client"))
	s.P()
	for _, method := range service.Methods {
		s.P(s.generateClientSignature(method))
	}
	s.P("}")
	s.P()

	// Client implementation
	s.P("type ", s.ClientImpl(service), " struct {")
	s.P("cc ", s.Ident(SRPCPackage, "Client"))
	s.P("}")
	s.P()

	// Client constructor
	s.P("func New", s.ClientIface(service), "(cc ", s.Ident(SRPCPackage, "Client"), ") ", s.ClientIface(service), " {")
	s.P("return &", s.ClientImpl(service), "{cc}")
	s.P("}")
	s.P()

	// Client method implementations
	s.P("func (c *", s.ClientImpl(service), ") SRPCClient() ", s.Ident(SRPCPackage, "Client"), "{ return c.cc }")
	s.P()
	for _, method := range service.Methods {
		s.generateClientMethod(method)
	}

	// Server interface
	s.P("type ", s.ServerIface(service), " interface {")
	for _, method := range service.Methods {
		s.P(s.generateServerSignature(method))
	}
	s.P("}")
	s.P()

	// Server Unimplemented struct
	s.P("type ", s.ServerUnimpl(service), " struct {}")
	s.P()
	for _, method := range service.Methods {
		s.generateUnimplementedServerMethod(method)
	}
	s.P()

	// Service ID constant
	serviceID := s.GetServiceID(service)
	s.P("const ", s.ServerServiceID(service), " = ", strconv.Quote(serviceID))

	// Handler implementation.
	s.P("type ", s.ServerHandler(service), " struct{")
	s.P("impl ", s.ServerIface(service))
	s.P("}")
	s.P()
	s.P("func (", s.ServerHandler(service), ") GetServiceID() string { return ", s.ServerServiceID(service), " }")
	s.P()
	s.P("func (", s.ServerHandler(service), ") GetMethodIDs() []string {")
	s.P("return []string{")
	for _, method := range service.Methods {
		_, methodID := s.GetServiceAndMethodID(method)
		s.P(strconv.Quote(methodID), ",")
	}
	s.P("}")
	s.P("}")
	s.P()

	// InvokeMethod function.
	s.P("func (d *", s.ServerHandler(service), ") InvokeMethod(")
	s.P("serviceID, methodID string,")
	s.P("strm srpc.Stream,")
	s.P(") (bool, error) {")
	s.P("if serviceID != \"\" && serviceID != d.GetServiceID() {")
	s.P("return false, nil")
	s.P("}")
	s.P()
	s.P("switch methodID {")
	for _, method := range service.Methods {
		_, methodID := s.GetServiceAndMethodID(method)
		s.P("case ", strconv.Quote(methodID), ":")
		s.P("return true, d.InvokeMethod_", method.GoName, "(d.impl, strm)")
	}
	s.P("default:")
	s.P("return false, nil")
	s.P("}")
	s.P("}")

	// InvokeMethod_Echo function.
	for _, method := range service.Methods {
		inType := s.InputType(method)
		// outType := s.OutputType(method)
		// _, methodID := s.GetServiceAndMethodID(method)
		s.P()
		s.P(
			"func (", s.ServerHandler(service), ") InvokeMethod_", method.GoName,
			"(impl ", s.ServerIface(service), ", strm srpc.Stream) error {",
		)

		if method.Desc.IsStreamingClient() {
			// streaming client
			s.P("clientStrm := &", s.ServerStreamImpl(method), "{strm}")
			s.P("return impl.", method.GoName, "(clientStrm)")
		} else {
			s.P("req := new(", inType, ")")
			s.P("if err := strm.MsgRecv(req); err != nil { return err }")

			if method.Desc.IsStreamingServer() {
				// non-streaming client, streaming server
				s.P("serverStrm := &", s.ServerStreamImpl(method), "{strm}")
				s.P("return impl.", method.GoName, "(req, serverStrm)")
			} else {
				// non-streaming client, non-streaming server
				s.P("out, err := impl.", method.GoName, "(strm.Context(), req)")
				s.P("if err != nil { return err }")
				s.P("return strm.MsgSend(out)")
			}
		}

		s.P("}")
	}

	/*
		s.P("func (", s.ServerHandler(service), ") NumMethods() int { return ", len(service.Methods), " }")
		s.P()
		s.P("func (", s.ServerHandler(service), ") Method(n int) (string, ", s.Ident(SRPCPackage, "Encoding"), ", ", s.Ident(SRPCPackage, "Receiver"), ", interface{}, bool) {")
		s.P("switch n {")
		for i, method := range service.Methods {
			s.P("case ", i, ":")
			// encodingName := s.EncodingName()
			s.P("return ", s.RPCGoString(method), ", ", encodingName, "{}, ")
			s.generateServerReceiver(method)
			s.P("}, ", s.ServerIface(service), ".", method.GoName, ", true")
		}
		s.P("default:")
		s.P(`return "", nil, nil, nil, false`)
		s.P("}")
		s.P("}")
	*/

	s.P()

	// Registration helper
	s.P("func SRPCRegister", service.GoName, "(mux ", s.Ident(SRPCPackage, "Mux"), ", impl ", s.ServerIface(service), ") error {")
	s.P("return mux.Register(&", s.ServerHandler(service), "{impl: impl})")
	s.P("}")

	// Server methods
	for _, method := range service.Methods {
		s.generateServerMethod(method)
	}
}

//
// client methods
//

func (s *srpc) generateClientSignature(method *protogen.Method) string {
	reqArg := ", in *" + s.InputType(method)
	if method.Desc.IsStreamingClient() {
		reqArg = ""
	}
	respName := "*" + s.OutputType(method)
	if method.Desc.IsStreamingServer() || method.Desc.IsStreamingClient() {
		respName = s.ClientStreamIface(method)
	}
	return fmt.Sprintf("%s(ctx %s%s) (%s, error)", method.GoName, s.Ident("context", "Context"), reqArg, respName)
}

func (s *srpc) generateClientMethod(p *protogen.Method) {
	recvType := s.ClientImpl(p.Parent)
	outType := s.OutputType(p)
	inType := s.InputType(p)

	service, method := s.GetServiceAndMethodID(p)
	serviceQuote, methodQuote := strconv.Quote(service), strconv.Quote(method)

	s.P("func (c *", recvType, ") ", s.generateClientSignature(p), "{")
	if !p.Desc.IsStreamingServer() && !p.Desc.IsStreamingClient() {
		s.P("out := new(", outType, ")")
		s.P("err := c.cc.Invoke(ctx, ", serviceQuote, ", ", methodQuote, ", ", "in, out)")
		s.P("if err != nil { return nil, err }")
		s.P("return out, nil")
		s.P("}")
		s.P()
		return
	}

	firstMsgRef := "nil"
	if !p.Desc.IsStreamingClient() {
		firstMsgRef = "in"
	}

	s.P("stream, err := c.cc.NewStream(ctx, ", serviceQuote, ", ", methodQuote, ", ", firstMsgRef, ")")
	s.P("if err != nil { return nil, err }")
	s.P("strm := &", s.ClientStreamImpl(p), "{stream}")
	if !p.Desc.IsStreamingClient() {
		s.P("if err := strm.CloseSend(); err != nil { return nil, err }")
	}
	s.P("return strm, nil")
	s.P("}")
	s.P()

	genSend := p.Desc.IsStreamingClient()
	genRecv := p.Desc.IsStreamingServer()
	genCloseAndRecv := !p.Desc.IsStreamingServer()

	// Stream auxiliary types and methods.
	s.P("type ", s.ClientStreamIface(p), " interface {")
	s.P(s.Ident(SRPCPackage, "Stream"))
	if genSend {
		s.P("Send(*", inType, ") error")
	}
	if genRecv {
		s.P("Recv() (*", outType, ", error)")
		s.P("RecvTo(*", outType, ") error")
	}
	if genCloseAndRecv {
		s.P("CloseAndRecv() (*", outType, ", error)")
	}
	s.P("}")
	s.P()

	s.P("type ", s.ClientStreamImpl(p), " struct {")
	s.P(s.Ident(SRPCPackage, "Stream"))
	s.P("}")
	s.P()

	if genSend {
		s.P("func (x *", s.ClientStreamImpl(p), ") Send(m *", inType, ") error {")
		s.P("return x.MsgSend(m)")
		s.P("}")
		s.P()
	}
	if genRecv {
		s.P("func (x *", s.ClientStreamImpl(p), ") Recv() (*", outType, ", error) {")
		s.P("m := new(", outType, ")")
		s.P("if err := x.MsgRecv(m); err != nil { return nil, err }")
		s.P("return m, nil")
		s.P("}")
		s.P()

		s.P("func (x *", s.ClientStreamImpl(p), ") RecvTo(m *", outType, ") error {")
		s.P("return x.MsgRecv(m)")
		s.P("}")
		s.P()
	}
	if genCloseAndRecv {
		s.P("func (x *", s.ClientStreamImpl(p), ") CloseAndRecv() (*", outType, ", error) {")
		s.P("if err := x.CloseSend(); err != nil { return nil, err }")
		s.P("m := new(", outType, ")")
		s.P("if err := x.MsgRecv(m); err != nil { return nil, err }")
		s.P("return m, nil")
		s.P("}")
		s.P()

		s.P("func (x *", s.ClientStreamImpl(p), ") CloseAndMsgRecv(m *", outType, ") error {")
		s.P("if err := x.CloseSend(); err != nil { return err }")
		s.P("return x.MsgRecv(m)")
		s.P("}")
		s.P()
	}
}

//
// server methods
//

func (s *srpc) generateServerSignature(method *protogen.Method) string {
	var reqArgs []string
	ret := "error"
	if !method.Desc.IsStreamingServer() && !method.Desc.IsStreamingClient() {
		reqArgs = append(reqArgs, s.Ident("context", "Context"))
		ret = "(*" + s.OutputType(method) + ", error)"
	}
	if !method.Desc.IsStreamingClient() {
		reqArgs = append(reqArgs, "*"+s.InputType(method))
	}
	if method.Desc.IsStreamingServer() || method.Desc.IsStreamingClient() {
		reqArgs = append(reqArgs, s.ServerStreamIface(method))
	}
	return method.GoName + "(" + strings.Join(reqArgs, ", ") + ") " + ret
}

func (s *srpc) generateUnimplementedServerMethod(method *protogen.Method) {
	s.P("func (s *", s.ServerUnimpl(method.Parent), ") ", s.generateServerSignature(method), " {")
	if !method.Desc.IsStreamingServer() && !method.Desc.IsStreamingClient() {
		s.P("return nil, ", s.Ident(SRPCPackage, "ErrUnimplemented"))
	} else {
		s.P("return ", s.Ident(SRPCPackage, "ErrUnimplemented"))
	}
	s.P("}")
	s.P()
}

/*
func (s *srpc) generateServerReceiver(method *protogen.Method) {
	s.P("func (srv interface{}, ctx context.Context, in1, in2 interface{}) (" + s.Ident(SRPCPackage, "Message") + ", error) {")
	if !method.Desc.IsStreamingServer() && !method.Desc.IsStreamingClient() {
		s.P("return srv.(", s.ServerIface(method.Parent), ").")
	} else {
		s.P("return nil, srv.(", s.ServerIface(method.Parent), ").")
	}
	s.P(method.GoName, "(")

	n := 1
	if !method.Desc.IsStreamingServer() && !method.Desc.IsStreamingClient() {
		s.P("ctx,")
	}
	if !method.Desc.IsStreamingClient() {
		s.P("in", n, ".(*", s.InputType(method), "),")
		n++
	}
	if method.Desc.IsStreamingServer() || method.Desc.IsStreamingClient() {
		s.P("&", s.ServerStreamImpl(method), "{in", n, ".(", s.Ident(SRPCPackage, "Stream"), ")},")
	}
	s.P(")")
}
*/

func (s *srpc) generateServerMethod(method *protogen.Method) {
	genSend := method.Desc.IsStreamingServer()
	genSendAndClose := !method.Desc.IsStreamingServer()
	genRecv := method.Desc.IsStreamingClient()

	// Stream auxiliary types and methods.
	s.P("type ", s.ServerStreamIface(method), " interface {")
	s.P(s.Ident(SRPCPackage, "Stream"))
	if genSend {
		s.P("Send(*", s.OutputType(method), ") error")
	}
	if genSendAndClose {
		s.P("SendAndClose(*", s.OutputType(method), ") error")
	}
	if genRecv {
		s.P("Recv() (*", s.InputType(method), ", error)")
	}
	s.P("}")
	s.P()

	s.P("type ", s.ServerStreamImpl(method), " struct {")
	s.P(s.Ident(SRPCPackage, "Stream"))
	s.P("}")
	s.P()

	if genSend {
		s.P("func (x *", s.ServerStreamImpl(method), ") Send(m *", s.OutputType(method), ") error {")
		s.P("return x.MsgSend(m)")
		s.P("}")
		s.P()
	}

	if genSendAndClose {
		s.P("func (x *", s.ServerStreamImpl(method), ") SendAndClose(m *", s.OutputType(method), ") error {")
		s.P("if err := x.MsgSend(m); err != nil { return err }")
		s.P("return x.CloseSend()")
		s.P("}")
		s.P()
	}

	if genRecv {
		s.P("func (x *", s.ServerStreamImpl(method), ") Recv() (*", s.InputType(method), ", error) {")
		s.P("m := new(", s.InputType(method), ")")
		s.P("if err := x.MsgRecv(m); err != nil { return nil, err }")
		s.P("return m, nil")
		s.P("}")
		s.P()

		s.P("func (x *", s.ServerStreamImpl(method), ") RecvTo(m *", s.InputType(method), ") error {")
		s.P("return x.MsgRecv(m)")
		s.P("}")
		s.P()
	}
}
