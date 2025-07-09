package requests

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Client 封装了 http 的参数等信息
type Client struct {
	// 自定义 Client
	client *http.Client

	url    string
	method string
	header http.Header
	params url.Values

	form      url.Values
	json      interface{}
	multipart FileForm
}

// FileForm form 参数和文件参数
type FileForm struct {
	Value url.Values
	File  map[string]string
}

// Result http 响应结果
type Result struct {
	Resp *http.Response
	Err  error
}

// Get http `GET` 请求
func Get(url string) *Client {
	return newClient(url, http.MethodGet, nil)
}

// Post http `POST` 请求
func Post(url string) *Client {
	return newClient(url, http.MethodPost, nil)
}

// Put http `PUT` 请求
func Put(url string) *Client {
	return newClient(url, http.MethodPut, nil)
}

// Delete http `DELETE` 请求
func Delete(url string) *Client {
	return newClient(url, http.MethodDelete, nil)
}

// Request 用于自定义请求方式，比如 `HEAD`、`PATCH`、`OPTIONS`、`TRACE`
// client 参数用于替换 DefaultClient，如果为 nil 则会使用默认的
func Request(url, method string, client *http.Client) *Client {
	return newClient(url, method, client)
}

// Params http 请求中 url 参数
func (c *Client) Params(params url.Values) *Client {
	for k, v := range params {
		c.params[k] = v
	}
	return c
}

// Header http 请求头
func (c *Client) Header(k, v string) *Client {
	c.header.Set(k, v)
	return c
}

// Headers http 请求头
func (c *Client) Headers(header http.Header) *Client {
	for k, v := range header {
		c.header[k] = v
	}
	return c
}

// Form 表单提交参数
func (c *Client) Form(form url.Values) *Client {
	c.header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.form = form
	return c
}

// Json json 提交参数
func (c *Client) Json(json interface{}) *Client {
	c.header.Set("Content-Type", "application/json")
	c.json = json
	return c
}

// Multipart form-data 提交参数
func (c *Client) Multipart(multipart FileForm) *Client {
	c.multipart = multipart
	return c
}

// Send 发送 http 请求
func (c *Client) Send() *Result {
	var result *Result

	if len(c.params) > 0 {
		u, err := url.Parse(c.url)
		if err != nil {
			result = &Result{Err: err}
			return result
		}
		q := u.Query()
		for k, vs := range c.params {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		u.RawQuery = q.Encode()
		c.url = u.String()
	}

	contentType := c.header.Get("Content-Type")
	if c.multipart.Value != nil || c.multipart.File != nil {
		result = c.createMultipartForm()
	} else if strings.HasPrefix(contentType, "application/json") {
		result = c.createJson()
	} else if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		result = c.createForm()
	} else {
		result = c.createEmptyBody()
	}

	return result
}

// form-data
func (c *Client) createMultipartForm() *Result {
	var result = new(Result)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for name, filename := range c.multipart.File {
		file, err := os.Open(filename)
		if err != nil {
			result.Err = err
			return result
		}
		err = func() (err error) {
			defer func(file *os.File) {
				_ = file.Close()
			}(file)

			var part io.Writer
			if part, err = writer.CreateFormFile(name, filename); err != nil {
				return
			}
			_, err = io.Copy(part, file)
			return
		}()
		if err != nil {
			result.Err = err
			return result
		}
	}

	for name, values := range c.multipart.Value {
		for _, value := range values {
			_ = writer.WriteField(name, value)
		}
	}

	if err := writer.Close(); err != nil {
		result.Err = err
		return result
	}

	req, err := http.NewRequest(c.method, c.url, body)
	if err != nil {
		result.Err = err
		return result
	}

	req.Header = c.header
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.doSend(req, result)
	return result
}

// application/json
func (c *Client) createJson() *Result {
	var result = new(Result)

	b, err := json.Marshal(c.json)
	if err != nil {
		result.Err = err
		return result
	}

	req, err := http.NewRequest(c.method, c.url, bytes.NewReader(b))
	if err != nil {
		result.Err = err
		return result
	}

	req.Header = c.header
	c.doSend(req, result)
	return result
}

// application/x-www-form-urlencoded
func (c *Client) createForm() *Result {
	var result = new(Result)

	form := c.form.Encode()

	req, err := http.NewRequest(c.method, c.url, strings.NewReader(form))
	if err != nil {
		result.Err = err
		return result
	}

	req.Header = c.header
	c.doSend(req, result)
	return result
}

// none http body
func (c *Client) createEmptyBody() *Result {
	var result = new(Result)

	req, err := http.NewRequest(c.method, c.url, nil)
	if err != nil {
		result.Err = err
		return result
	}

	req.Header = c.header
	c.doSend(req, result)
	return result
}

func (c *Client) doSend(req *http.Request, result *Result) {
	if c.client != nil {
		result.Resp, result.Err = c.client.Do(req)
		return
	}

	result.Resp, result.Err = http.DefaultClient.Do(req)
}

// StatusOk 判断 http 响应码是否为 200
func (r *Result) StatusOk() *Result {
	if r.Err != nil {
		return r
	}
	if r.Resp.StatusCode != http.StatusOK {
		r.Err = errors.New("status code is not 200")
		return r
	}

	return r
}

// Status2xx 判断 http 响应码是否为 2xx
func (r *Result) Status2xx() *Result {
	if r.Err != nil {
		return r
	}
	if r.Resp.StatusCode < http.StatusOK || r.Resp.StatusCode >= http.StatusMultipleChoices {
		r.Err = errors.New("status code is not match [200, 300)")
		return r
	}

	return r
}

// Raw 获取 http 响应内容，返回字节数组
func (r *Result) Raw() ([]byte, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(r.Resp.Body)

	return io.ReadAll(r.Resp.Body)
}

// Text 获取 http 响应内容，返回字符串
func (r *Result) Text() (string, error) {
	b, err := r.Raw()
	if err != nil {
		r.Err = err
		return "", r.Err
	}

	return string(b), nil
}

// Json 获取 http 响应内容，返回 json
func (r *Result) Json(v interface{}) error {
	b, err := r.Raw()
	if err != nil {
		r.Err = err
		return r.Err
	}

	return json.Unmarshal(b, v)
}

// Save 获取 http 响应内容，保存为文件
func (r *Result) Save(name string) error {
	if r.Err != nil {
		return r.Err
	}

	f, err := os.Create(name)
	if err != nil {
		r.Err = err
		return r.Err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	_, err = io.Copy(f, r.Resp.Body)
	if err != nil {
		r.Err = err
		return r.Err
	}

	_ = r.Resp.Body.Close()
	return nil
}

func newClient(u string, method string, client *http.Client) *Client {
	return &Client{
		client: client,
		url:    u,
		method: method,
		header: make(http.Header),
		params: make(url.Values),
		form:   make(url.Values),
	}
}
