package rpc

import (
	"bufio"
	"encoding/gob"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

/*
关于reflect的介绍： reflect.TypeOf 和 reflect.ValueOf

一个 Type 表示一个Go类型. 它是一个接口, 有许多方法来区分类型以及检查它们的组成部分, 例如一个结构体的成员或一个函数的参数等.
函数 reflect.TypeOf 接受任意的 interface{} 类型, 并以reflect.Type形式返回其动态类型

一个 reflect.Value 可以装载任意类型的值. 函数 reflect.ValueOf 接受任意的 interface{} 类型, 并返回一个装载着其动态值的 reflect.Value.
和 reflect.TypeOf 类似, reflect.ValueOf 返回的结果也是具体的类型, 但是 reflect.Value 也可以持有一个接口值.
对 Value 调用 Type 方法将返回具体类型所对应的 reflect.Type
reflect.ValueOf 的逆操作是 reflect.Value.Interface 方法. 它返回一个 interface{} 类型，装载着与 reflect.Value 相同的具体值

reflect.Value 和 interface{} 都能装载任意的值.
所不同的是, 一个空的接口隐藏了值内部的表示方式和所有方法, 因此只有我们知道具体的动态类型才能使用类型断言来访问内部的值(就像上面那样), 内部值我们没法访问.
相比之下, 一个 Value 则有很多方法来检查其内容, 无论它的具体类型是什么.

*/

/*
Go官方提供了一个RPC库: net/rpc。
包rpc提供了通过网络访问一个对象的方法的能力。
服务器需要注册对象， 通过对象的类型名暴露这个服务。
注册后这个对象的输出方法就可以远程调用，这个库封装了底层传输的细节，包括序列化。
服务器可以注册多个不同类型的对象，但是注册相同类型的多个对象的时候回出错。
*/

/*

方法的类型是可输出的 (the method's type is exported)
方法本身也是可输出的 （the method is exported）
方法必须由两个参数，必须是输出类型或者是内建类型 (the method has two arguments, both exported or builtin types)
方法的第二个参数是指针类型 (the method's second argument is a pointer)
方法返回类型为 error (the method has return type error)

func (t *T) MethodName(argType T1, replyType *T2) error

这个方法的第一个参数代表调用者(client)提供的参数，
第二个参数代表要返回给调用者的计算结果，

方法的返回值如果不为空， 那么它作为一个字符串返回给调用者。
如果返回error，则reply参数不会返回给调用者。

*/

const (
	DefaultRPCPath   = "/_goRPC_"
	DefaultDebugPath = "/debug/rpc"
)

var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

//方法 handler
type methodType struct {
	sync.Mutex
	method    reflect.Method
	ArgType   reflect.Type //T1
	ReplyType reflect.Type //T2
	numCalls  uint         //调用次数
}

//服务实例 controller 或者 receiver
type service struct {
	name   string
	rcvr   reflect.Value          // controller的动态值
	typ    reflect.Type           // controller的动态类型
	method map[string]*methodType //注册方法
}

//rpc服务器 router
type Server struct {
	serviceMap sync.Map
	reqLock    sync.Mutex
	freeReq    *Request
	respLock   sync.Mutex
	freeResp   *Response
}

func NewServer() *Server {
	return &Server{}
}

var DefaultServer = NewServer()

/*
rune 等同于int32,常用来处理unicode或utf-8字符
判断函数是否为导出 （大写开头）
*/
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

//类型为可导出或者为内置类型
func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	return isExported(t.Name()) || t.PkgPath() == ""
}

/*
// Register publishes in the server the set of methods of the
// receiver value that satisfy the following conditions:
//	- exported method of exported type
//	- two arguments, both of exported type
//	- the second argument is a pointer
//	- one return value, of type error
// It returns an error if the receiver is not an exported type or has
// no suitable methods. It also logs the error using package log.
// The client accesses each method using a string of the form "Type.Method",
// where Type is the receiver's concrete type.

concrete 具体的
*/

//按实例注册服务
func (server *Server) Register(rcrv interface{}) error {
	return server.register(rcrv, "", false)
}

//指定名称注册服务
func (server *Server) RegisterName(rcrv interface{}, name string) error {
	return server.register(rcrv, name, true)
}

func (server *Server) register(rcrv interface{}, name string, useName bool) error {
	s := new(service)

	s.typ = reflect.TypeOf(rcrv)
	s.rcvr = reflect.ValueOf(rcrv)

	//Indirect returns the value that v points to.
	//返回其（称之为receiver或者controller）指向值的类型名称 如返回A而非返回*
	sname := reflect.Indirect(s.rcvr).Type().Name()

	//直接使用参数name
	if useName {
		sname = name
	}

	//服务名应该指定
	if sname == "" {
		s := "rpc.Register: no service name for type " + s.typ.String()
		log.Print(s)
		return errors.New(s)
	}

	//未指定名称且结构体小写; 例如 type a struct{}
	if !useName && !isExported(sname) {
		s := "rpc.Register: type " + sname + "is not exported"
		log.Print(s)
		return errors.New(s)
	}

	s.name = sname

	//获取结构体可作为rpc服务调用的方法（方法满足条件）
	s.method = suitableMethods(s.typ, true)

	if len(s.method) == 0 {
		str := ""

		// PtrTo returns the pointer type with element t.
		// For example, if t represents type Foo, PtrTo(t) represents *Foo.
		// 尝试用指针获取方法
		method := suitableMethods(reflect.PtrTo(s.typ), false)
		if len(method) == 0 {
			str = "rpc.Register : type " + sname + " has no exproted methods of suitable type (hint: pass a pointer to value of that type"
		} else {
			str = "rpc.Register : type " + sname + " has no exported methods of suitable type，try use address of struct as receiver"
		}
		log.Print(str)
		return errors.New(str)
	}

	//相当于注册controller到router中
	if _, dup := server.serviceMap.LoadOrStore(sname, s); dup {
		return errors.New("rpc: service has already defined : " + sname)
	}
	return nil
}

//获取某类型定义的满足rpc要求的方法
func suitableMethods(typ reflect.Type, reportErr bool) map[string]*methodType {
	methods := make(map[string]*methodType)
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)

		mtype := method.Type
		mname := method.Name

		//PkgPath is the package path that qualifies a lower case (unexported)
		//包内使用，未导出，不满足rpc要求
		if method.PkgPath != "" {
			log.Print("method "+mname+" pkgpath ", method.PkgPath)
			continue
		}

		//0 caller 1 arg 2 reply
		if mtype.NumIn() != 3 {
			if reportErr {
				log.Printf("rpc.Register: method %q has %d input paramters;needs exactly three \n", mname, mtype.NumIn())
			}
			continue
		}

		//参数是可导出或者内建
		argType := mtype.In(1)
		if !isExportedOrBuiltinType(argType) {
			if reportErr {
				log.Printf("rpc.Register: argument type of method %q is not exported or builtin: %q \n ", mname, argType)
			}
			continue
		}

		//结果可导出
		replyType := mtype.In(2)
		if !isExportedOrBuiltinType(argType) {
			if reportErr {
				log.Printf("rpc.Register: reply type of method %q is not exported or builtin: %q \n ", mname, replyType)
			}
			continue
		}

		//结果要求指针
		if replyType.Kind() != reflect.Ptr {
			if reportErr {
				log.Printf("rpc.Register: reply type of method %q is not a pointer: %q\n", mname, replyType)
			}
			continue
		}

		// 只有一个返回值
		if mtype.NumOut() != 1 {
			if reportErr {
				log.Printf("rpc.Register: method %q has %d output parameters; needs exactly one\n", mname, mtype.NumOut())
			}
			continue
		}
		// 返回值类型为error
		if returnType := mtype.Out(0); returnType != typeOfError {
			if reportErr {
				log.Printf("rpc.Register: return type of method %q is %q, must be error\n", mname, returnType)
			}
			continue
		}

		methods[mname] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
	}
	return methods
}

var invalidRequest = struct{}{}

/*
请求是在每次RPC调用之前编写的头文件。它在内部使用
但是这里记录下来是为了帮助调试，比如分析网络流量。
*/
type Request struct {
	ServiceMethod string
	Seq           uint64
	next          *Request
}

//获得一个指向Request{}的指针
func (server *Server) getRequest() *Request {
	server.reqLock.Lock()
	req := server.freeReq
	if req == nil {
		req = new(Request)
	} else {
		//server.freeReq指向req的next
		server.freeReq = req.next
		*req = Request{}
	}
	server.reqLock.Unlock()
	return req
}

//设置server的freeReq
func (server *Server) freeRequest(req *Request) {
	server.reqLock.Lock()
	req.next = server.freeReq
	server.freeReq = req
	server.reqLock.Unlock()
}

/*
响应是在每次RPC返回之前编写的头文件。它在内部使用
但是在这里记录是为了帮助调试，比如在分析网络流量时
*/
type Response struct {
	ServiceMethod string
	Seq           uint64
	Error         string
	next          *Response
}

//get a empty response
func (server *Server) getResponse() *Response {
	server.respLock.Lock()
	resp := server.freeResp
	if resp == nil {
		resp = new(Response)
	} else {
		server.freeResp = resp.next
		*resp = Response{}
	}
	server.respLock.Unlock()
	return resp
}

func (server *Server) freeResponse(resp *Response) {
	server.respLock.Lock()
	resp.next = server.freeResp
	server.freeResp = resp
	server.respLock.Unlock()
}

func (server *Server) sendResponse(sending *sync.Mutex, req *Request, reply interface{}, codec ServerCodec, errmsg string) {
	resp := server.getResponse()
	resp.ServiceMethod = req.ServiceMethod
	if errmsg != "" {
		resp.Error = errmsg
		reply = invalidRequest
	}
	resp.Seq = req.Seq

	sending.Lock()
	err := codec.WriteResponse(resp, reply)
	if debugLog && err != nil {
		log.Println("rpc: writing response error: ", err)
	}
	sending.Unlock()
	server.freeResponse(resp)

}

func (m *methodType) Numcalls() uint {
	m.Lock()
	n := m.numCalls
	m.Unlock()
	return n
}

//参数调用，写入结果
func (s *service) call(server *Server, sending *sync.Mutex, wg *sync.WaitGroup, mtype *methodType, req *Request, argv, replyv reflect.Value, codec ServerCodec) {
	if wg != nil {
		defer wg.Done()
	}
	mtype.Lock()
	mtype.numCalls++
	mtype.Unlock()

	f := mtype.method.Func

	returnValues := f.Call([]reflect.Value{s.rcvr, argv, replyv})

	errInter := returnValues[0].Interface()
	errmsg := ""
	if errInter != nil {
		errmsg = errInter.(error).Error()
	}
	server.sendResponse(sending, req, replyv.Interface(), codec, errmsg)
	server.freeRequest(req)
}

//服务编解码器：
/*
服务器成对调用ReadRequestHeader和ReadRequestBody
读取来自连接的请求，它调用WriteResponse回写一个响应。

服务器在链接用完后调用Close函数。ReadRequestBody可以用nil调用
参数来强制读取和丢弃请求体。

*/
type ServerCodec interface {
	ReadRequestHeader(*Request) error
	ReadRequestBody(interface{}) error
	WriteResponse(*Response, interface{}) error

	// Close can be called multiple times and must be idempotent. //幂等f(f(x)) = f(x). setTRUE
	Close() error
}

//实现 ServerCodec 接口
type gobServerCodec struct {
	rwc    io.ReadWriteCloser
	dec    *gob.Decoder
	enc    *gob.Encoder
	encBuf *bufio.Writer
	closed bool
}

//
func (c *gobServerCodec) ReadRequestHeader(r *Request) error {
	return c.dec.Decode(r)
}

//
func (c *gobServerCodec) ReadRequestBody(body interface{}) error {
	return c.dec.Decode(body)
}

//回应encode到c.enc中
func (c *gobServerCodec) WriteResponse(r *Response, body interface{}) (err error) {
	if err = c.enc.Encode(r); err != nil {
		if c.encBuf.Flush() == nil {
			log.Println("rpc : gob error encoding response:", err)
			c.Close()
		}
		return
	}

	if err = c.enc.Encode(body); err != nil {
		if c.encBuf.Flush() == nil {
			//body出错，flush response
			log.Println("rpc: gob error encoding body: ", err)
			c.Close()
		}
		return
	}

	//Flush 将缓存中的数据提交到底层的 io.Writer 中
	return c.encBuf.Flush()
}

//幂等
func (c *gobServerCodec) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return c.rwc.Close()
}

//采用 gobServerCodec 处理连接
func (server *Server) ServeConn(conn io.ReadWriteCloser) {
	buf := bufio.NewWriter(conn)
	srv := &gobServerCodec{
		rwc:    conn,
		dec:    gob.NewDecoder(conn),
		enc:    gob.NewEncoder(conn),
		encBuf: buf,
	}

	server.ServeCodec(srv)
}

//指定ServerCodec处理
func (server *Server) ServeCodec(codec ServerCodec) {
	sending := new(sync.Mutex)

	wg := new(sync.WaitGroup)

	for {
		service, mtype, req, argv, replyv, keepReading, err := server.readRequest(codec)
		if err != nil {
			if debugLog && err != io.EOF {
				log.Println("rpc: ", err)
			}

			if !keepReading {
				break
			}

			if req != nil {
				server.sendResponse(sending, req, invalidRequest, codec, err.Error())
				server.freeRequest(req)
			}
			continue
		}
		wg.Add(1)
		go service.call(server, sending, wg, mtype, req, argv, replyv, codec)

	}
	wg.Wait()
	codec.Close()
}

//ServeRequest类似于ServeCodec，但同步服务于单个请求。
//完成后不会关闭编解码器。
func (server *Server) ServerRequest(codec ServerCodec) error {
	sending := new(sync.Mutex)
	service, mtype, req, argv, replyv, keepReading, err := server.readRequest(codec)
	if err != nil {
		if !keepReading {
			return err
		}
		if req != nil {
			server.sendResponse(sending, req, invalidRequest, codec, err.Error())
			server.freeRequest(req)
		}
		return err
	}
	service.call(server, sending, nil, mtype, req, argv, replyv, codec)
	return nil
}

//得到request以及做函数调用前的参数准备
func (server *Server) readRequest(codec ServerCodec) (service *service, mtype *methodType, req *Request, argv, replyv reflect.Value, keepReading bool, err error) {
	service, mtype, req, keepReading, err = server.readRequestHeader(codec)

	if err != nil {
		if !keepReading {
			return
		}

		//忽略body
		codec.ReadRequestBody(nil)
		return
	}

	argIsValue := false

	if mtype.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(mtype.ArgType)
		argIsValue = true
	}

	if err = codec.ReadRequestBody(argv.Interface()); err != nil {
		return
	}

	if argIsValue {
		argv = argv.Elem()
	}

	replyv = reflect.New(mtype.ReplyType.Elem())

	switch mtype.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(mtype.ReplyType.Elem()))
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(mtype.ReplyType.Elem(), 0, 0))
	}

	return

}

func (server *Server) readRequestHeader(codec ServerCodec) (svc *service, mtype *methodType, req *Request, keepReading bool, err error) {
	//抢占request header.
	req = server.getRequest()

	//从连接中读请求数据解码到request中
	err = codec.ReadRequestHeader(req)
	if err != nil {
		req = nil
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		}
		err = errors.New("rpc: server cannot decode request: " + err.Error())
		return
	}

	keepReading = true

	dot := strings.LastIndex(req.ServiceMethod, ".")
	if dot < 0 {
		err = errors.New("roc : service/method request ill-formed: " + req.ServiceMethod)
		return
	}

	serviceName := req.ServiceMethod[:dot]
	methodName := req.ServiceMethod[dot:]

	svci, ok := server.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc: can't find service " + req.ServiceMethod)
		return
	}
	svc = svci.(*service)
	mtype = svc.method[methodName]
	if mtype == nil {
		err = errors.New("rpc: can't find method " + req.ServiceMethod)
	}
	return
}

//从端口监听中获取连接
func (server *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("roc.Serve: accept ", err.Error())
			return
		}
		go server.ServeConn(conn)
	}
}

// Can connect to RPC service using HTTP CONNECT to rpcPath.
var connected = "200 Connected to Go RPC"

//实现了http Handler接口
func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		io.WriteString(w, "405 must CONNECT\n")
		return
	}

	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Print("rpc hijacking ", req.RemoteAddr, " : ", err.Error())
		return
	}
	io.WriteString(conn, "HTTP/1.0 "+connected+"\n\n")
	server.ServeConn(conn)
}

func (server *Server) HandleHTTP(rpcPath, debugPath string) {
	http.Handle(rpcPath, server)
	http.Handle(debugPath, debugHTTP{server})
}

//公共函数提供给外界
func Register(rcvr interface{}) error { return DefaultServer.Register(rcvr) }

func RegisterName(name string, rcvr interface{}) error {
	return DefaultServer.RegisterName(rcvr, name)
}

func ServeConn(conn io.ReadWriteCloser) {
	DefaultServer.ServeConn(conn)
}

func Accept(lis net.Listener) { DefaultServer.Accept(lis) }

func HandleHTTP() {
	DefaultServer.HandleHTTP(DefaultRPCPath, DefaultDebugPath)
}
