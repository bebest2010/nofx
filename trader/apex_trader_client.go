package trader

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type ApexClientRequest struct {
	c      *Client
	params map[string]interface{}
	isUta  bool
}

type ServerResponse struct {
	Code    int               `json:"code,omitempty"`
	Msg     string            `json:"msg,omitempty"`
	Message string            `json:"message,omitempty"`
	Key     string            `json:"key,omitempty"`
	Detail  map[string]string `json:"detail,omitempty"`

	Total    uint64      `json:"total,omitempty"`
	Data     interface{} `json:"data,omitempty"`
	TimeCost int64       `json:"timeCost,omitempty"`
}

func SendRequest(ctx context.Context, opts []RequestOption, r *request, s *ApexClientRequest, err error) ([]byte, error) {
	r.setParams(s.params)
	data, err := s.c.callAPI(ctx, r, opts...)
	return data, err
}

type Client struct {
	APIKey     string
	APISecret  string
	Passphrase string
	BaseURL    string
	HTTPClient *http.Client
	Debug      bool
	Logger     *log.Logger
	do         doFunc
	ProxyURL   string
	PriKey     string
}

func NewApexHttpClient(baseUrl, apiKey string, APISecret string, passphrase string, options ...ClientOption) *Client {
	c := &Client{
		APIKey:     apiKey,
		APISecret:  APISecret,
		Passphrase: passphrase,
		BaseURL:    baseUrl,
		HTTPClient: http.DefaultClient,
		Logger:     log.New(os.Stderr, "Apex trading", log.LstdFlags),
	}

	// Apply the provided options
	for _, opt := range options {
		opt(c)
	}

	if c.ProxyURL != "" {
		proxyURL, err := url.Parse(c.ProxyURL)
		if err != nil {
			c.Logger.Printf("Error parsing proxy URL: %v", err)
			return nil // Or handle this more gracefully
		}
		c.HTTPClient.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	return c
}

func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.BaseURL = baseURL
	}
}

func WithPriKey(priKey string) ClientOption {
	return func(c *Client) {
		c.PriKey = priKey
	}
}

type doFunc func(req *http.Request) (*http.Response, error)

type ClientOption func(*Client)

func WithDebug(debug bool) ClientOption {
	return func(c *Client) {
		c.Debug = debug
	}
}

func PrettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", " ")
	return string(s)
}

func (c *Client) debug(format string, v ...interface{}) {
	if c.Debug {
		c.Logger.Printf(format, v...)
	}
}

func (c *Client) callAPI(ctx context.Context, r *request, opts ...RequestOption) (data []byte, err error) {
	err = c.parseRequest(r, opts...)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(r.method, r.fullURL, r.body)
	if err != nil {
		return []byte{}, err
	}
	req = req.WithContext(ctx)
	req.Header = r.header
	c.debug("request: %#v", req)
	f := c.do
	if f == nil {
		f = c.HTTPClient.Do
	}
	res, err := f(req)
	if err != nil {
		return []byte{}, err
	}
	data, err = io.ReadAll(res.Body)
	if err != nil {
		return []byte{}, err
	}
	defer func() {
		cerr := res.Body.Close()
		if err == nil && cerr != nil {
			err = cerr
		}
	}()
	c.debug("response: %#v", res)
	c.debug("response body: %s", string(data))
	c.debug("response status code: %d", res.StatusCode)

	if res.StatusCode >= http.StatusBadRequest {
		var (
			apiErr = new(APIError)
		)
		e := json.Unmarshal(data, apiErr)
		if e != nil {
			c.debug("failed to unmarshal json: %s", e)
		}
		return nil, apiErr
	}
	return data, nil
}

func (c *Client) parseRequest(r *request, opts ...RequestOption) (err error) {
	for _, opt := range opts {
		opt(r)
	}
	if err = r.validate(); err != nil {
		return err
	}

	baseURL := c.BaseURL
	path := r.endpoint
	fullURL := fmt.Sprintf("%s%s", baseURL, path)
	method := strings.ToUpper(r.method)

	h := http.Header{}
	if r.header != nil {
		h = r.header.Clone()
	}
	h.Set("User-Agent", fmt.Sprintf("%s/%s", Name, Version))

	// Build maps from params (JSON body) and query (url.Values)
	paramsMap, err := mergeParamsAndQuery(r.params, nil, c)
	if err != nil {
		return err
	}
	queryMap, _ := mergeParamsAndQuery(nil, r.query, c)

	// Signed endpoints
	if r.secType == secTypeSigned {
		ts := GetCurrentTime()

		// Sign over POST params or GET query, not both
		var signMap map[string]string
		if method == http.MethodPost {
			signMap = paramsMap
		} else {
			signMap = queryMap
		}
		dataString := BuildSortedKV(signMap)

		sig, err := c.signCompat(method, path, signMap, ts)
		if err != nil {
			c.debug("signCompat failed: %s", err)
			return err
		}
		h.Set(signatureKey, sig)
		h.Set(timestampKey, strconv.FormatInt(ts, 10))
		h.Set(apiRequestKey, c.APIKey)
		h.Set(passphraseKey, c.Passphrase)

		c.debug("sign inputs method=%s path=%s ts=%d dataString=%q", method, path, ts, dataString)
		c.debug("signature=%s", sig)

		if method == http.MethodPost {
			// POST signed: send as x-www-form-urlencoded body
			h.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
			r.body = bytes.NewBufferString(dataString)
			r.fullURL = fullURL
		} else {
			// GET signed: append to URL
			if dataString != "" {
				fullURL = fullURL + "?" + dataString
			}
			r.body = bytes.NewBuffer(nil)
			r.fullURL = fullURL
		}
		r.header = h
		return nil
	} else if r.secType == secTypePrivateKey {

	}

	// Unsigned endpoints (server uses ShouldBindQuery)
	if method == http.MethodPost {
		// Merge params+query and put them into the URL querystring
		merged := make(map[string]string, len(paramsMap)+len(queryMap))
		for k, v := range paramsMap {
			merged[k] = v
		}
		for k, v := range queryMap {
			merged[k] = v
		}
		ds := BuildSortedKV(merged)
		if ds != "" {
			fullURL = fullURL + "?" + ds
		}
		r.body = bytes.NewBuffer(nil) // no body so server reads from query
		r.fullURL = fullURL
		r.header = h
		c.debug("unsigned POST as query url=%s", r.fullURL)
		return nil
	}

	// Unsigned GET
	qs := ""
	if r.query != nil {
		qs = r.query.Encode()
	}
	if qs != "" {
		fullURL = fullURL + "?" + qs
	}
	r.fullURL = fullURL
	r.body = bytes.NewBuffer(nil)
	r.header = h
	return nil
}

func (c *Client) signCompat(method, pattern string, para map[string]string, ts int64) (string, error) {
	dataString := BuildSortedKV(para)
	message := strconv.FormatInt(ts, 10) + strings.ToUpper(method) + pattern + dataString

	key := base64.StdEncoding.EncodeToString([]byte(c.APISecret))
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(message))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return sig, nil
}

// mergeParamsAndQuery merges params and query into a single key-value map.
func mergeParamsAndQuery(rawParams []byte, q url.Values, c *Client) (map[string]string, error) {
	out := make(map[string]string)

	if len(rawParams) > 0 {
		var any interface{}
		if err := json.Unmarshal(rawParams, &any); err == nil {
			if m, ok := any.(map[string]interface{}); ok {
				for k, v := range m {
					if s := Stringify(v); s != "" {
						out[k] = s
					}
				}
			}
		} else {
			c.debug("failed to unmarshal json: %s", err)
		}
	}

	if q != nil {
		for k, vals := range q {
			if len(vals) > 0 && vals[0] != "" {
				out[k] = vals[0]
			}
		}
	}
	return out, nil
}

func GetServerResponse(err error, data []byte) (*ServerResponse, error) {
	if err != nil {
		return nil, err
	}
	resp := new(ServerResponse)
	err = json.Unmarshal(data, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) NewUtaApexServiceWithParams(params map[string]interface{}) *ApexClientRequest {
	return &ApexClientRequest{
		c:      c,
		params: params,
		isUta:  true,
	}
}

func (c *Client) NewUtaApexServiceNoParams() *ApexClientRequest {
	return &ApexClientRequest{
		c:     c,
		isUta: true,
	}
}

func GetCurrentTime() int64 {
	now := time.Now()
	unixNano := now.UnixNano()
	timeStamp := unixNano / int64(time.Millisecond)
	return timeStamp
}
