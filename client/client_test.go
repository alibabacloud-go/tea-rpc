package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	util "github.com/alibabacloud-go/tea-utils/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/alibabacloud-go/tea/utils"
	credential "github.com/aliyun/credentials-go/credentials"
)

func mockServer(status int, json string) (server *httptest.Server) {
	// Start a test server locally.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(json))
		return
	}))
	return ts
}

type countingMarshaler struct {
	count *int32
}

func (m countingMarshaler) MarshalJSON() ([]byte, error) {
	atomic.AddInt32(m.count, 1)
	return []byte(`"body-value"`), nil
}

func Test_DoRequest(t *testing.T) {
	conf := new(credential.Config)
	conf.AccessKeyId = tea.String("accesskey_id")
	conf.AccessKeySecret = tea.String("accesskey_secret")
	conf.Type = tea.String("access_key")
	c, err := credential.NewCredential(conf)
	utils.AssertNil(t, err)

	config := new(Config).SetRegionId("域名")
	_, err = NewClient(config)
	utils.AssertNotNil(t, err)
	utils.AssertEqual(t, err.Error(), "域名 is not matched ^[a-zA-Z0-9_-]+$")

	_, err = NewClient(nil)
	utils.AssertNotNil(t, err)
	utils.AssertEqual(t, err.Error(), "SDKError:\n   StatusCode: 0\n   Code: ParameterMissing\n   Message: 'config' can not be unset\n   Data: \n")

	config.SetRegionId("cn-hangzhou")
	_, err = NewClient(config)
	utils.AssertNotNil(t, err)
	utils.AssertEqual(t, err.Error(), "SDKError:\n   StatusCode: 0\n   Code: ParameterMissing\n   Message: 'accessKeyId' and 'accessKeySecret' or 'credential' can not be unset\n   Data: \n")

	config.SetCredential(c)
	_, err = NewClient(config)
	utils.AssertNil(t, err)

	config.SetAccessKeyId("accesskey_id").
		SetAccessKeySecret("accesskey_secret")
	_, err = NewClient(config)
	utils.AssertNil(t, err)

	config.SetSecurityToken("SecurityToken")
	_, err = NewClient(config)
	utils.AssertNil(t, err)

	utils.AssertEqual(t, fmt.Sprintln(config), fmt.Sprintln(config.GoString()))

	config.SetProtocol("http").
		SetReadTimeout(10).
		SetConnectTimeout(10).
		SetHttpProxy("httpproxy").
		SetHttpsProxy("httpsproxy").
		SetEndpoint("endpoint").
		SetNoProxy("npproxy").
		SetMaxIdleConns(1).
		SetNetwork("public").
		SetUserAgent("aliyun").
		SetSuffix("ali").
		SetSocks5NetWork("tcp").
		SetSocks5Proxy("proxy").
		SetType("access_key").
		SetEndpointType("inner").
		SetOpenPlatformEndpoint("endpoint")

	config.SetHttpProxy("").
		SetHttpsProxy("").
		SetSocks5NetWork("").
		SetSocks5Proxy("")
	client, err := NewClient(config)
	utils.AssertNil(t, err)
	utils.AssertNotNil(t, client)

	ts := mockServer(400, `{"Code": "杭州"}`)
	defer ts.Close()
	client.Endpoint = tea.String(strings.Replace(ts.URL, "http://", "", 1))
	runtime := new(util.RuntimeOptions)
	resp, err := client.DoRequest(tea.String("testApi"), tea.String("HTTP"), tea.String("GET"),
		tea.String("2019-12-12"), tea.String("AK"), nil, nil, runtime)
	utils.AssertNotNil(t, err)
	utils.AssertEqual(t, err.Error(), "SDKError:\n   StatusCode: 400\n   Code: 杭州\n   Message: code: 400, <nil> request id: <nil>\n   Data: {\"Code\":\"杭州\"}\n")
	utils.AssertNil(t, resp)

	runtime.SetMaxAttempts(3).SetAutoretry(true).SetBackoffPeriod(1).SetBackoffPolicy("ok")
	resp, err = client.DoRequest(tea.String("testApi"), tea.String("HTTP"), tea.String("GET"),
		tea.String("2019-12-12"), tea.String("AK"), nil, map[string]interface{}{"test": "ok"}, runtime)
	utils.AssertNotNil(t, err)
	utils.AssertEqual(t, err.Error(), "SDKError:\n   StatusCode: 400\n   Code: 杭州\n   Message: code: 400, <nil> request id: <nil>\n   Data: {\"Code\":\"杭州\"}\n")
	utils.AssertNil(t, resp)

	ts = mockServer(200, `{"Code": "杭州"}`)
	client.Endpoint = tea.String(strings.Replace(ts.URL, "http://", "", 1))
	resp, err = client.DoRequest(tea.String("testApi"), tea.String("HTTP"), tea.String("GET"),
		tea.String("2019-12-12"), tea.String("AK"), nil, map[string]interface{}{"test": "ok"}, runtime)
	utils.AssertNil(t, err)
	utils.AssertEqual(t, resp, map[string]interface{}{"Code": "杭州"})

	client.Credential = nil
	ak, err := client.GetAccessKeyId()
	utils.AssertNil(t, err)
	utils.AssertEqual(t, tea.StringValue(ak), "")

	secret, err := client.GetAccessKeySecret()
	utils.AssertNil(t, err)
	utils.AssertEqual(t, tea.StringValue(secret), "")

	token, err := client.GetSecurityToken()
	utils.AssertNil(t, err)
	utils.AssertEqual(t, tea.StringValue(token), "")

	err = client.CheckConfig(config)
	utils.AssertNil(t, err)

	config.SetEndpoint("")
	client.EndpointRule = tea.String("")
	err = client.CheckConfig(config)
	utils.AssertEqual(t, err.Error(), "SDKError:\n   StatusCode: 0\n   Code: ParameterMissing\n   Message: 'config.endpoint' can not be empty\n   Data: \n")
}

func Test_DoRequestReturnsFlattenError(t *testing.T) {
	config := new(Config).
		SetAccessKeyId("accesskey_id").
		SetAccessKeySecret("accesskey_secret").
		SetRegionId("cn-hangzhou")
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()
	client.Endpoint = tea.String(strings.TrimPrefix(ts.URL, "http://"))

	tests := []struct {
		name  string
		query map[string]interface{}
		body  map[string]interface{}
	}{
		{
			name: "query",
			query: map[string]interface{}{
				"Items": []interface{}{"first", nil},
			},
		},
		{
			name: "body",
			body: map[string]interface{}{
				"Items": []interface{}{"first", nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atomic.StoreInt32(&requestCount, 0)
			runtime := new(util.RuntimeOptions).
				SetAutoretry(true).
				SetMaxAttempts(3)
			resp, err := client.DoRequest(
				tea.String("testApi"),
				tea.String("HTTP"),
				tea.String("POST"),
				tea.String("2019-12-12"),
				tea.String("AK"),
				tt.query,
				tt.body,
				runtime,
			)
			if err == nil {
				t.Fatal("DoRequest() error = nil, want flatten error")
			}
			wantErr := `cannot serialize repeated parameter element "Items.2": value is nil`
			if err.Error() != wantErr {
				t.Fatalf("DoRequest() error = %q, want %q", err, wantErr)
			}
			if resp != nil {
				t.Fatalf("DoRequest() response = %#v, want nil", resp)
			}
			if got := atomic.LoadInt32(&requestCount); got != 0 {
				t.Fatalf("server request count = %d, want 0", got)
			}
		})
	}
}

func Test_DoRequestPreservesSuccessfulSerializationFlow(t *testing.T) {
	config := new(Config).
		SetAccessKeyId("accesskey_id").
		SetAccessKeySecret("accesskey_secret").
		SetRegionId("cn-hangzhou")
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	var requestCount int32
	var marshalCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		queryValues := r.URL.Query()
		if got := queryValues.Get("Items.1"); got != "query-value" {
			t.Errorf("query Items.1 = %q, want query-value", got)
		}
		if values, ok := queryValues["Items.2"]; !ok || len(values) != 1 || values[0] != "" {
			t.Errorf("query Items.2 = %#v, want one empty value", values)
		}
		if queryValues.Get("Signature") == "" {
			t.Error("query Signature is empty")
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm() error = %v", err)
		}
		if got := r.PostForm.Get("BodyValue"); got != "body-value" {
			t.Errorf("body BodyValue = %q, want body-value", got)
		}
		if values, ok := r.PostForm["Empty"]; !ok || len(values) != 1 || values[0] != "" {
			t.Errorf("body Empty = %#v, want one empty value", values)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()
	client.Endpoint = tea.String(strings.TrimPrefix(ts.URL, "http://"))

	resp, err := client.DoRequest(
		tea.String("testApi"),
		tea.String("HTTP"),
		tea.String("POST"),
		tea.String("2019-12-12"),
		tea.String("AK"),
		map[string]interface{}{
			"Items": []interface{}{"query-value", ""},
		},
		map[string]interface{}{
			"BodyValue": countingMarshaler{count: &marshalCount},
			"Empty":     "",
		},
		new(util.RuntimeOptions),
	)
	if err != nil {
		t.Fatalf("DoRequest() error = %v", err)
	}
	if resp == nil {
		t.Fatal("DoRequest() response = nil")
	}
	if got := atomic.LoadInt32(&requestCount); got != 1 {
		t.Fatalf("server request count = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&marshalCount); got != 2 {
		t.Fatalf("body marshal count = %d, want 2 to match the original body and signature serialization calls", got)
	}
}
