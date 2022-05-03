package httpkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dgrr/http2"
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"net/url"
	"time"
)

var (
	MethodGet     = fasthttp.MethodGet
	MethodHead    = fasthttp.MethodHead
	MethodPost    = fasthttp.MethodPost
	MethodPut     = fasthttp.MethodPut
	MethodPatch   = fasthttp.MethodPatch
	MethodDelete  = fasthttp.MethodDelete
	MethodConnect = fasthttp.MethodConnect
	MethodOptions = fasthttp.MethodOptions
	MethodTrace   = fasthttp.MethodTrace
)

type http struct {
	c           *fasthttp.Client
	h2c         *fasthttp.HostClient //http2
	usedH2      bool
	req         *fasthttp.Request
	resp        *fasthttp.Response
	uri         *fasthttp.URI
	qs          map[string][]string
	method      string
	contentType string
	timeout     time.Duration
}

func new(method string) *http {
	h := &http{}
	h.c = &fasthttp.Client{}
	h.uri = fasthttp.AcquireURI()
	h.req = fasthttp.AcquireRequest()
	h.resp = fasthttp.AcquireResponse()
	h.method = method
	h.qs = make(map[string][]string)
	return h
}

func New(method string) *http {
	h := new(method)
	return h
}

// newH2
// @Param host api.xx.xx:443,only support 443(https)
func newH2(method, host string) *http {
	h := &http{}
	h.h2c = &fasthttp.HostClient{
		Addr: host,
	}
	err := http2.ConfigureClient(h.h2c, http2.ClientOpts{})
	if err != nil {
		panic(fmt.Errorf("http2 configuration failed:%v", err))
	}
	h.usedH2 = true
	h.uri = fasthttp.AcquireURI()
	h.req = fasthttp.AcquireRequest()
	h.resp = fasthttp.AcquireResponse()
	h.method = method
	h.qs = make(map[string][]string)
	return h
}

func NewH2(method, host string) *http {
	h := newH2(method, host)
	return h
}

func (h *http) Url(rawUrl string) *http {
	err := h.uri.Parse(nil, []byte(rawUrl))
	if err != nil {
		fmt.Println("invalid raw url:", err)
	}
	return h
}

func (h *http) Scheme(scheme string) *http {
	h.uri.SetScheme(scheme)
	return h
}

func (h *http) Header(k, v string) *http {
	h.req.Header.Add(k, v)
	return h
}

func (h *http) SetCookie(k, v string) *http {
	h.req.Header.SetCookie(k, v)
	return h
}

func (h *http) SetCookieKVs(kvs string) *http {
	cookies := readCookies(kvs)
	for _, c := range cookies {
		h.req.Header.SetCookie(c.Name, c.Value)
	}
	return h
}

func (h *http) Param(k, v string) *http {
	if m, ok := h.qs[k]; ok {
		h.qs[k] = append(m, v)
	} else {
		h.qs[k] = []string{v}
	}
	return h
}

func (h *http) buildQueryString() string {
	var queryString string
	var buf bytes.Buffer
	buf.Write(h.uri.QueryString())
	if len(h.qs) > 0 {
		buf.WriteByte('&')
		for k, v := range h.qs {
			for _, vv := range v {
				buf.WriteString(url.QueryEscape(k))
				buf.WriteByte('=')
				buf.WriteString(url.QueryEscape(vv))
				buf.WriteByte('&')
			}
		}
	}
	queryString = buf.String()
	queryString = queryString[0 : len(queryString)-1]
	return queryString
}

func (h *http) Body(body interface{}) *http {
	switch t := body.(type) {
	case string:
		bf := bytes.NewBufferString(t)
		h.req.SetBody(bf.Bytes())
		h.req.Header.SetContentLength(len(t))
	case []byte:
		bf := bytes.NewBuffer(t)
		h.req.SetBody(bf.Bytes())
		h.req.Header.SetContentLength(len(t))
	default:
		fmt.Println("unsupported body data type:", t)
	}
	return h
}

func (h *http) JSONMarshal(obj interface{}) ([]byte, error) {
	bf := bytes.NewBuffer([]byte{})
	jsonEncoder := json.NewEncoder(bf)
	err := jsonEncoder.Encode(obj)
	if err != nil {
		return nil, err
	}
	return bf.Bytes(), nil
}

func (h *http) JsonBody(body interface{}) (*http, error) {
	if body != nil {
		b, err := h.JSONMarshal(body)
		if err != nil {
			return h, errors.Wrap(err, fmt.Sprintf("obj could not be converted to JSON body"))
		}
		h.req.SetBody(b)
		h.req.Header.SetContentLength(len(b))
		h.req.Header.SetContentType("application/json")
	}
	return h, nil
}

func (h *http) ContentType(ct string) *http {
	h.req.Header.SetContentType(ct)
	return h
}

func (h *http) SetTimeout(dur time.Duration) *http {
	h.timeout = dur
	return h
}

func (h *http) do() error {
	qs := h.buildQueryString()
	h.uri.SetQueryString(qs)
	h.req.SetURI(h.uri)
	h.req.Header.SetMethod(h.method)
	if h.timeout <= 0 {
		h.timeout = time.Second * 60
	}
	if h.usedH2 {
		err := h.h2c.DoTimeout(h.req, h.resp, h.timeout)
		if err != nil {
			return err
		}
	} else {
		err := h.c.DoTimeout(h.req, h.resp, h.timeout)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *http) String() (string, error) {
	e := h.do()
	if e != nil {
		return "", e
	}
	defer func() {
		fasthttp.ReleaseResponse(h.resp)
		fasthttp.ReleaseRequest(h.req)
	}()
	b := h.resp.Body()
	return string(b), nil
}

func (h *http) Bytes() ([]byte, error) {
	e := h.do()
	if e != nil {
		return nil, e
	}
	defer func() {
		fasthttp.ReleaseResponse(h.resp)
		fasthttp.ReleaseRequest(h.req)
	}()
	b := h.resp.Body()
	return b, nil
}

func (h *http) Response() (*fasthttp.Response, error) {
	e := h.do()
	if e != nil {
		return nil, e
	}
	defer func() {
		fasthttp.ReleaseResponse(h.resp)
		fasthttp.ReleaseRequest(h.req)
	}()
	var resp fasthttp.Response
	h.resp.CopyTo(&resp)
	return &resp, nil
}

// Get Sample Get
func Get(url string) *http {
	return new(MethodGet).Url(url)
}

// Post Sample Post
func Post(url string) *http {
	return new(MethodPost).Url(url)
}

// GetH2 Sample Http2 Get
// @Param host api.xx.xx:443,only support 443(https)
func GetH2(url, host string) *http {
	return newH2(MethodGet, host).Url(url)
}

// PostH2 Sample Http2 Post
// @Param host api.xx.xx:443,only support 443(https)
func PostH2(url, host string) *http {
	return newH2(MethodPost, host).Url(url)
}
