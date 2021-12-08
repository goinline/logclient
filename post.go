package logclient

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

//Deflate 数据压缩
func Deflate(b []byte) ([]byte, error) {
	r := bytes.Buffer{}
	w := zlib.NewWriter(&r)
	_, err := w.Write(b)
	w.Close()
	if err == nil {
		return r.Bytes(), nil
	}
	return nil, err
}

//Inflate 解压缩数据
func Inflate(b []byte) ([]byte, error) {
	buf := bytes.NewBuffer(b)
	r, err := zlib.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return ioutil.ReadAll(r)
}

type ioReader struct {
	data  []byte
	begin int
}

//go:nosplit
func min(a, b int) int {

	if a < b {
		return a
	}
	return b
}
func (b *ioReader) Read(p []byte) (n int, err error) {
	if b.data == nil {
		return 0, io.EOF
	}
	if len(b.data) == b.begin {
		b.data = nil
		return 0, io.EOF
	}
	res := copy(p, b.data[b.begin:])
	b.begin += res
	if b.begin == len(b.data) {
		b.data = nil
	}
	return res, nil
}
func (b *ioReader) Close() error {
	if b != nil {
		b.data = nil
		b.begin = 0
	}
	return nil
}

//发起一个post请求,返回请求对象
func PostHttpRequest(url string, params map[string]string, data []byte, duration time.Duration) ([]byte, int, error) {
	var body []byte
	var err error = nil
	if v, ok := params["Content-Encoding"]; ok && v == "deflate" {
		body, err = Deflate(data)
	} else {
		body = data
	}
	if err != nil {
		return nil, -1, err
	}
	request, err := http.NewRequest("POST", url, &ioReader{data: body})
	if nil != err {
		return nil, -1, err
	}
	useParams := map[string]string{
		"Accept-Encoding": "identity, deflate",
		"Content-Type":    "Application/json;charset=UTF-8",
		"User-Agent":      "TingYun-Agent/HttpLog",
	}
	for k, v := range params {
		useParams[k] = v
	}
	for k, v := range useParams {
		request.Header.Add(k, v)
	}

	client := &http.Client{Timeout: duration}
	defer func() {
		if exception := recover(); exception != nil {
			fmt.Println(exception)
		}
		if request.Body != nil && request.Body != http.NoBody {
			request.Body.Close()
		}
	}()
	response, err := client.Do(request)
	if err != nil {
		return nil, -1, err
	}
	defer response.Body.Close()
	if response.StatusCode == 200 {
		if b, err := ioutil.ReadAll(response.Body); err != nil {
			return nil, 200, err
		} else {
			encoding := response.Header.Get("Content-Encoding")
			if encoding == "gzip" || encoding == "deflate" {
				d, err := Inflate(b)
				if err == nil {
					return d, 200, nil
				}
			}
			return b, 200, nil
		}
	} else {
		return nil, response.StatusCode, nil
	}
}
