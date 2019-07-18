package jsonrpc

import (
	"encoding/json"
	"errors"
	"github.com/shengzhch/learn/rpc"
	"io"
	"sync"
)

//(rpc.Requese -- serverRequst -- rpc.Response -- serverResponse)

var errMissingParams = errors.New("jsonrpc: request body miss params")

type serverCodec struct {
	dec     *json.Decoder
	enc     *json.Encoder
	c       io.Closer
	req     serverRequest
	mux     sync.Mutex
	seq     uint64
	pending map[uint64]*json.RawMessage
}

func NewServerCodec(conn io.ReadWriteCloser) rpc.ServerCodec {
	return &serverCodec{
		dec:     json.NewDecoder(conn),
		enc:     json.NewEncoder(conn),
		c:       conn,
		pending: make(map[uint64]*json.RawMessage),
	}
}

type serverRequest struct {
	Method string           `json:"method"`
	Params *json.RawMessage `json:"params"`
	Id     *json.RawMessage `json:"id"`
}

func (r *serverRequest) reset() {
	r.Method = ""
	r.Params = nil
	r.Id = nil
}

type serverResponse struct {
	Id     *json.RawMessage `json:"id"`
	Result interface{}      `json:"result"`
	Error  interface{}      `json:"error"`
}

//ReadRequseHeader
func (c *serverCodec) ReadRequestHeader(r *rpc.Request) error {
	c.req.reset()
	if err := c.dec.Decode(&c.req); err != nil {
		return err
	}
	r.ServiceMethod = c.req.Method

	c.mux.Lock()
	c.seq++
	c.pending[c.seq] = c.req.Id
	c.req.Id = nil
	r.Seq = c.seq
	c.mux.Unlock()

	return nil
}

//ReadRequestBody
func (c *serverCodec) ReadRequestBody(x interface{}) error {
	if x == nil {
		return nil
	}

	if c.req.Params == nil {
		return errMissingParams
	}
	var params [1]interface{}
	params[0] = x
	return json.Unmarshal(*c.req.Params, &params)
}

//null RawMessage
var null = json.RawMessage([]byte("null"))

//WriteResponse
func (c *serverCodec) WriteResponse(r *rpc.Response, x interface{}) error {
	c.mux.Lock()
	b, ok := c.pending[r.Seq]
	if !ok {
		c.mux.Unlock()
		return errors.New("invalid sequence number in response")
	}
	delete(c.pending, r.Seq)
	c.mux.Unlock()

	if b == nil {
		b = &null
	}

	resp := serverResponse{Id: b}

	if r.Error == "" {
		resp.Result = x
	} else {
		resp.Error = r.Error
	}

	return c.enc.Encode(resp)
}

func (c *serverCodec) Close() error {
	return c.c.Close()
}

func ServerConn(conn io.ReadWriteCloser) {
	rpc.ServeCodec(NewServerCodec(conn))
}
